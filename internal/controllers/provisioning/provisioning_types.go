package provisioning

import (
	"encoding/json"
	"fmt"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/strings/slices"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type CreateInfrastructureFunc func(
	target ProvisioningTarget,
	domain string,
	infra *InfrastructureManifests) ProvisioningResult

type InfrastructureManifests struct {
	EntraUsers           []*provisioningv1.EntraUser
	AzureDbs             []*provisioningv1.AzureDatabase
	AzureManagedDbs      []*provisioningv1.AzureManagedDatabase
	HelmReleases         []*provisioningv1.HelmRelease
	AzureVirtualMachines []*provisioningv1.AzureVirtualMachine
	AzureVirtualDesktops []*provisioningv1.AzureVirtualDesktop
}

func (infra *InfrastructureManifests) Get(id provisioningv1.ProvisioningResourceIdendtifier) (BaseProvisioningResource, bool) {
	switch id.Kind {
	case provisioningv1.ProvisioningResourceKindEntraUser:
		return FindByName[*provisioningv1.EntraUser](id.Name, infra.EntraUsers)
	case provisioningv1.ProvisioningResourceKindAzureDatabase:
		return FindByName[*provisioningv1.AzureDatabase](id.Name, infra.AzureDbs)
	case provisioningv1.ProvisioningResourceKindAzureManagedDatabase:
		return FindByName[*provisioningv1.AzureManagedDatabase](id.Name, infra.AzureManagedDbs)
	case provisioningv1.ProvisioningResourceKindHelmRelease:
		return FindByName[*provisioningv1.HelmRelease](id.Name, infra.HelmReleases)
	case provisioningv1.ProvisioningResourceKindAzureVirtualMachine:
		return FindByName[*provisioningv1.AzureVirtualMachine](id.Name, infra.AzureVirtualMachines)
	case provisioningv1.ProvisioningResourceKindAzureVirtualDesktop:
		return FindByName[*provisioningv1.AzureVirtualDesktop](id.Name, infra.AzureVirtualDesktops)
	default:
		return nil, false
	}
}

func FindByName[R ProvisioningResource](name string, slice []R) (R, bool) {
	for _, res := range slice {
		if res.GetName() == name {
			return res, true
		}
	}
	return nil, false
}

type ProvisioningResult struct {
	Error      error
	HasChanges bool
}

type ProvisioningResource interface {
	*provisioningv1.EntraUser | *provisioningv1.AzureDatabase | *provisioningv1.AzureManagedDatabase | *provisioningv1.HelmRelease | *provisioningv1.AzureVirtualMachine | *provisioningv1.AzureVirtualDesktop

	GetProvisioningMeta() *provisioningv1.ProvisioningMeta
	GetSpec() any
	GetName() string
	GetNamespace() string
	GetObjectKind() schema.ObjectKind
}

type BaseProvisioningResource interface {
	GetProvisioningMeta() *provisioningv1.ProvisioningMeta
	GetName() string
	GetNamespace() string
	GetObjectKind() schema.ObjectKind
}

type ProvisioningTarget interface {
	GetName() string
	GetDescription() string
	GetNamespace() string
	GetPlatformName() string

	runtime.Object
}

func Match[T any](target ProvisioningTarget, ifTenant func(*platformv1.Tenant) T, ifPlatform func(*platformv1.Platform) T) T {
	switch target.(type) {
	case *platformv1.Tenant:
		return ifTenant(target.(*platformv1.Tenant))
	case *platformv1.Platform:
		return ifPlatform(target.(*platformv1.Platform))
	default:
		panic(fmt.Errorf("unsupported target: '%s'", reflect.TypeOf(target)))
	}
}

func GetDeletePolicy(target ProvisioningTarget) platformv1.DeletePolicy {
	return Match(target,
		func(tenant *platformv1.Tenant) platformv1.DeletePolicy {
			return tenant.Spec.DeletePolicy
		},
		func(*platformv1.Platform) platformv1.DeletePolicy {
			return platformv1.DeletePolicyRetainStatefulResources
		},
	)
}

func GetTemplateContext(target ProvisioningTarget) any {
	return Match(target,
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

type Cloner[C any] interface {
	DeepCopy() C
}

func getResourceKeys[R ProvisioningResource](res R) (platform, domain string, target provisioningv1.ProvisioningTargetCategory, ok bool) {
	platform = res.GetProvisioningMeta().PlatformRef
	domain = res.GetProvisioningMeta().DomainRef
	target = res.GetProvisioningMeta().Target.Category

	if len(platform) == 0 || len(domain) == 0 {
		return platform, domain, target, false
	}

	return platform, domain, target, true
}

func exludeTenant(filter provisioningv1.ProvisioningTargetFilter, tenant string) bool {
	tenantInList := slices.Contains(filter.Values, tenant)
	if filter.Kind == provisioningv1.ProvisioningFilterKindBlacklist && tenantInList {
		return true
	}

	if filter.Kind == provisioningv1.ProvisioningFilterKindWhitelist && !tenantInList {
		return true
	}

	return false
}

func selectItemsInTarget[R ProvisioningResource](platform string, domain string, source []R, target ProvisioningTarget) []R {
	result := []R{}
	for _, res := range source {
		provisioningMeta := res.GetProvisioningMeta()

		if provisioningMeta.Target.Category == provisioningv1.ProvisioningTargetCategoryTenant {
			if exludeTenant(provisioningMeta.Target.Filter, target.GetName()) {
				continue
			}

		}

		targetCategory := Match(target,
			func(tenant *platformv1.Tenant) provisioningv1.ProvisioningTargetCategory {
				return provisioningv1.ProvisioningTargetCategoryTenant
			},
			func(*platformv1.Platform) provisioningv1.ProvisioningTargetCategory {
				return provisioningv1.ProvisioningTargetCategoryPlatform
			},
		)

		if targetCategory != provisioningMeta.Target.Category {
			continue
		}

		if provisioningMeta.PlatformRef == platform && provisioningMeta.DomainRef == domain {
			result = append(result, res)
		}
	}
	return result
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

		overrides := Match(target,
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

type jsonTransformer struct {
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
