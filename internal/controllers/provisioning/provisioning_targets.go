package provisioning

import (
	"encoding/json"
	"fmt"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
			func(tenant *platformv1.Tenant) map[string]*apiextensionsv1.JSON {
				return (res.GetProvisioningMeta()).TenantOverrides
			},
			func(*platformv1.Platform) map[string]*apiextensionsv1.JSON {
				return nil
			},
		)

		if overrides == nil {
			result = append(result, res)
			continue
		}

		tenantOverridesJson, exists := overrides[target.GetName()]
		if !exists {
			result = append(result, res)
			continue
		}

		var tenantOverridesMap map[string]any
		if err := json.Unmarshal(tenantOverridesJson.Raw, &tenantOverridesMap); err != nil {
			return nil, err
		}

		resSpecJsonBytes, err := json.Marshal(res.GetSpec())
		if err != nil {
			return nil, err
		}

		var targetSpecMap map[string]any
		if err := json.Unmarshal(resSpecJsonBytes, &targetSpecMap); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&targetSpecMap, tenantOverridesMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
			return nil, err
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
