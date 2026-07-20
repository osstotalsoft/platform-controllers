package provisioning

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type ProvisioningTarget interface {
	GetName() string
	GetDescription() string
	GetNamespace() string
	GetPlatformName() string

	runtime.Object
}

type Cloner[C any] interface {
	DeepCopy() C
}

type jsonTransformer struct {
}

func MatchTarget[T any](target ProvisioningTarget, ifTenant func(*platformv1.Tenant) T, ifPlatform func(*platformv1.Platform) T) T {
	switch target := target.(type) {
	case *platformv1.Tenant:
		return ifTenant(target)
	case *platformv1.Platform:
		return ifPlatform(target)
	default:
		panic(fmt.Errorf("unsupported target: '%s'", reflect.TypeOf(target)))
	}
}

func GetDeletePolicy(target ProvisioningTarget) platformv1.DeletePolicy {
	return MatchTarget(target,
		func(tenant *platformv1.Tenant) platformv1.DeletePolicy {
			return tenant.Spec.DeletePolicy
		},
		func(*platformv1.Platform) platformv1.DeletePolicy {
			return platformv1.DeletePolicyRetainStatefulResources
		},
	)
}

func GetTemplateContext(target ProvisioningTarget) any {
	return MatchTarget(target,
		func(tenant *platformv1.Tenant) any {
			return struct {
				Platform string
				Tenant   struct {
					Id          string
					Code        string
					Description string
				}
			}{
				Platform: tenant.GetPlatformName(),
				Tenant: struct {
					Id          string
					Code        string
					Description string
				}{
					Id:          tenant.Spec.Id,
					Code:        tenant.GetName(),
					Description: tenant.GetDescription(),
				},
			}
		},
		func(platform *platformv1.Platform) any {
			return struct {
				Platform string
			}{
				Platform: platform.GetName(),
			}
		},
	)
}

// matchProvisioningOverride finds the first patch in patches whose Target matches res's GVK/name/namespace.
func matchProvisioningOverride(patches []platformv1.ProvisioningResourcePatch, res ProvisioningResource, target ProvisioningTarget) *apiextensionsv1.JSON {
	gvk := res.GetObjectKind().GroupVersionKind()
	for _, override := range patches {
		if override.Target.APIVersion == gvk.GroupVersion().String() &&
			override.Target.Kind == gvk.Kind &&
			override.Target.Name == res.GetName() &&
			(override.Target.Namespace == res.GetNamespace() || (override.Target.Namespace == "" && res.GetNamespace() == target.GetNamespace())) {
			return override.Spec
		}
	}
	return nil
}

func allNil(overrides []*apiextensionsv1.JSON) bool {
	for _, override := range overrides {
		if override != nil {
			return false
		}
	}
	return true
}

func applyTargetOverrides[R interface {
	ProvisioningResource
	Cloner[R]
}](source []R, target ProvisioningTarget, category *platformv1.TenantCategory) ([]R, error) {
	if source == nil {
		return source, nil
	}

	result := []R{}

	for _, res := range source {
		ensureResourceGVK(res)

		// Ordered by ascending precedence: category-map override (base) < TenantCategory override < tenant name override < tenant-specific override.
		overrides := MatchTarget(target,
			func(tenant *platformv1.Tenant) []*apiextensionsv1.JSON {
				var overridesFromCategoryMap *apiextensionsv1.JSON
				if categoryOverrides := (res.GetProvisioningMeta()).CategoryOverrides; categoryOverrides != nil {
					overridesFromCategoryMap = categoryOverrides[tenant.Spec.CategoryRef]
				}

				var overridesFromTenantCategory *apiextensionsv1.JSON
				if category != nil {
					overridesFromTenantCategory = matchProvisioningOverride(category.Spec.ProvisioningOverrides, res, target)
				}

				var overridesFromResource *apiextensionsv1.JSON
				if (res.GetProvisioningMeta()).TenantOverrides != nil {
					for key, val := range (res.GetProvisioningMeta()).TenantOverrides {
						if matched, _ := filepath.Match(key, target.GetName()); matched {
							overridesFromResource = val
							break
						}
					}
				}

				overridesFromTenant := matchProvisioningOverride(tenant.Spec.ProvisioningOverrides, res, target)

				return []*apiextensionsv1.JSON{overridesFromCategoryMap, overridesFromTenantCategory, overridesFromResource, overridesFromTenant}
			},
			func(*platformv1.Platform) []*apiextensionsv1.JSON {
				return nil
			},
		)

		if allNil(overrides) {
			result = append(result, res)
			continue
		}

		resSpecJsonBytes, err := json.Marshal(res.GetSpec())
		if err != nil {
			return nil, err
		}

		var targetSpecMap map[string]any
		if err := json.Unmarshal(resSpecJsonBytes, &targetSpecMap); err != nil {
			return nil, err
		}

		for _, override := range overrides {
			if override == nil {
				continue
			}

			var overrideMap map[string]any
			if err := json.Unmarshal(override.Raw, &overrideMap); err != nil {
				return nil, err
			}

			if err := mergo.Merge(&targetSpecMap, overrideMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
				return nil, err
			}
		}

		resSpecJsonBytes, err = json.Marshal(targetSpecMap)
		if err != nil {
			return nil, err
		}

		resClone := res.DeepCopy()

		if err := json.Unmarshal(resSpecJsonBytes, resClone.GetSpec()); err != nil {
			return nil, err
		}

		result = append(result, resClone)
	}

	return result, nil
}

func ensureResourceGVK(res ProvisioningResource) {
	gvk := res.GetObjectKind().GroupVersionKind()
	if gvk.Kind != "" && gvk.GroupVersion().String() != "" {
		return
	}

	kind := reflect.Indirect(reflect.ValueOf(res)).Type().Name()
	if kind == "" {
		return
	}

	res.GetObjectKind().SetGroupVersionKind(provisioningv1.SchemeGroupVersion.WithKind(kind))
}

func (t jsonTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(apiextensionsv1.JSON{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				srcRaw := src.FieldByName("Raw").Bytes()
				var srcMap map[string]any
				if err := json.Unmarshal(srcRaw, &srcMap); err != nil {
					return err
				}

				dstRaw := dst.FieldByName("Raw").Bytes()
				var dstMap map[string]any
				if err := json.Unmarshal(dstRaw, &dstMap); err != nil {
					return err
				}

				if err := mergo.Merge(&dstMap, srcMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
					return err
				}

				dstRaw, err := json.Marshal(dstMap)
				if err != nil {
					return err
				}

				dst.FieldByName("Raw").SetBytes(dstRaw)
			}
			return nil
		}
	}
	return nil
}
