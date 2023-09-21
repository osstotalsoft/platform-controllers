package provisioning

import (
	"encoding/json"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type ProvisioningResource interface {
	*provisioningv1.AzureDatabase | *provisioningv1.AzureManagedDatabase | *provisioningv1.HelmRelease | *provisioningv1.AzureVirtualMachine | *provisioningv1.AzureVirtualDesktop

	GetProvisioningMeta() *provisioningv1.ProvisioningMeta
	GetSpec() any
	GetName() string
	GetNamespace() string
}

type Cloner[C any] interface {
	DeepCopy() C
}

func getPlatformAndDomain[R ProvisioningResource](res R) (platform, domain string, ok bool) {
	platform = res.GetProvisioningMeta().PlatformRef
	domain = res.GetProvisioningMeta().DomainRef
	if len(platform) == 0 || len(domain) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}

func selectItemsInPlatformAndDomain[R ProvisioningResource](platform, domain string, source []R) []R {
	result := []R{}
	for _, res := range source {
		if res.GetProvisioningMeta().PlatformRef == platform && res.GetProvisioningMeta().DomainRef == domain {

			result = append(result, res)
		}
	}
	return result
}

func applyTenantOverrides[R interface {
	ProvisioningResource
	Cloner[R]
}](source []R, tenantName string) ([]R, error) {
	if source == nil {
		return source, nil
	}

	result := []R{}

	for _, res := range source {
		overrides := res.GetProvisioningMeta().TenantOverrides

		if overrides == nil {
			result = append(result, res)
			continue
		}

		tenantOverridesJson, exists := overrides[tenantName]
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

type jsonTransformer struct {
}

func (t jsonTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(apiextensionsv1.JSON{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				srcRaw := src.FieldByName("Raw").Bytes()
				var srcMap map[string]interface{}
				if err := json.Unmarshal(srcRaw, &srcMap); err != nil {
					return err
				}

				dstRaw := dst.FieldByName("Raw").Bytes()
				var dstMap map[string]interface{}
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
