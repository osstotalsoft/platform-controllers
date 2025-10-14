package provisioning

import (
	"encoding/json"
	"fmt"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"totalsoft.ro/platform-controllers/internal/tuple"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
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

func applyTargetOverrides[R interface {
	ProvisioningResource
	Cloner[R]
}](source []R, target ProvisioningTarget) ([]R, error) {
	if source == nil {
		return source, nil
	}

	result := []R{}

	for _, res := range source {
		overrides := MatchTarget(target,
			func(tenant *platformv1.Tenant) tuple.T2[*apiextensionsv1.JSON, *apiextensionsv1.JSON] {
				var overridesFromTenant *apiextensionsv1.JSON
				for _, override := range tenant.Spec.ProvisioningOverrides {
					gvk := res.GetObjectKind().GroupVersionKind()
					if override.APIVersion == gvk.GroupVersion().String() &&
						override.Kind == gvk.Kind &&
						override.Name == res.GetName() &&
						(override.Namespace == res.GetNamespace() || (override.Namespace == "" && res.GetNamespace() == target.GetNamespace())) {
						overridesFromTenant = override.Spec
						break
					}
				}
				var overridesFromResource *apiextensionsv1.JSON
				if (res.GetProvisioningMeta()).TenantOverrides != nil {
					tenantOverridesJson, exists := (res.GetProvisioningMeta()).TenantOverrides[target.GetName()]
					if exists {
						overridesFromResource = tenantOverridesJson
					}
				}

				return tuple.New2(overridesFromTenant, overridesFromResource)
			},
			func(*platformv1.Platform) tuple.T2[*apiextensionsv1.JSON, *apiextensionsv1.JSON] {
				return tuple.New2[*apiextensionsv1.JSON, *apiextensionsv1.JSON](nil, nil)
			},
		)

		if overrides.V1 == nil && overrides.V2 == nil {
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

		if overrides.V1 != nil {
			var overridesFromTenantMap map[string]any
			if err := json.Unmarshal(overrides.V1.Raw, &overridesFromTenantMap); err != nil {
				return nil, err
			}

			if err := mergo.Merge(&targetSpecMap, overridesFromTenantMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
				return nil, err
			}
		}

		if overrides.V2 != nil {
			var overridesFromResourceMap map[string]any
			if err := json.Unmarshal(overrides.V2.Raw, &overridesFromResourceMap); err != nil {
				return nil, err
			}

			if err := mergo.Merge(&targetSpecMap, overridesFromResourceMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
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
