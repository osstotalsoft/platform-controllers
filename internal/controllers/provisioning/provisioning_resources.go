package provisioning

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/strings/slices"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

const (
	ProvisioningResourceKindEntraUser             = provisioningv1.ProvisioningResourceKind("EntraUser")
	ProvisioningResourceKindAzureDatabase         = provisioningv1.ProvisioningResourceKind("AzureDatabase")
	ProvisioningResourceKindAzureManagedDatabase  = provisioningv1.ProvisioningResourceKind("AzureManagedDatabase")
	ProvisioningResourceKindAzurePowerShellScript = provisioningv1.ProvisioningResourceKind("AzurePowerShellScript")
	ProvisioningResourceKindAzureVirtualDesktop   = provisioningv1.ProvisioningResourceKind("AzureVirtualDesktop")
	ProvisioningResourceKindAzureVirtualMachine   = provisioningv1.ProvisioningResourceKind("AzureVirtualMachine")
	ProvisioningResourceKindHelmRelease           = provisioningv1.ProvisioningResourceKind("HelmRelease")
	ProvisioningResourceKindMsSqlDatabase         = provisioningv1.ProvisioningResourceKind("MsSqlDatabase")
)

type InfrastructureManifests struct {
	EntraUsers             []*provisioningv1.EntraUser
	AzureDbs               []*provisioningv1.AzureDatabase
	AzureManagedDbs        []*provisioningv1.AzureManagedDatabase
	AzurePowerShellScripts []*provisioningv1.AzurePowerShellScript
	HelmReleases           []*provisioningv1.HelmRelease
	AzureVirtualMachines   []*provisioningv1.AzureVirtualMachine
	AzureVirtualDesktops   []*provisioningv1.AzureVirtualDesktop
	MsSqlDbs               []*provisioningv1.MsSqlDatabase
}

type ProvisioningResource interface {
	GetProvisioningMeta() *provisioningv1.ProvisioningMeta
	GetName() string
	GetNamespace() string
	GetObjectKind() schema.ObjectKind
	GetSpec() any
}

func (infra *InfrastructureManifests) Get(id provisioningv1.ProvisioningResourceIdendtifier) (ProvisioningResource, bool) {
	switch id.Kind {
	case ProvisioningResourceKindEntraUser:
		return findByName(id.Name, infra.EntraUsers)
	case ProvisioningResourceKindAzureDatabase:
		return findByName(id.Name, infra.AzureDbs)
	case ProvisioningResourceKindAzureManagedDatabase:
		return findByName(id.Name, infra.AzureManagedDbs)
	case ProvisioningResourceKindAzurePowerShellScript:
		return findByName(id.Name, infra.AzurePowerShellScripts)
	case ProvisioningResourceKindHelmRelease:
		return findByName(id.Name, infra.HelmReleases)
	case ProvisioningResourceKindAzureVirtualMachine:
		return findByName(id.Name, infra.AzureVirtualMachines)
	case ProvisioningResourceKindAzureVirtualDesktop:
		return findByName(id.Name, infra.AzureVirtualDesktops)
	case ProvisioningResourceKindMsSqlDatabase:
		return findByName(id.Name, infra.MsSqlDbs)
	default:
		return nil, false
	}
}

func findByName[R ProvisioningResource](name string, slice []R) (ProvisioningResource, bool) {
	for _, res := range slice {
		if res.GetName() == name {
			return res, true
		}
	}
	return nil, false
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

		targetCategory := MatchTarget(target,
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

func getResourceKeys(res ProvisioningResource) (platform, domain string, target provisioningv1.ProvisioningTargetCategory, ok bool) {
	platform = res.GetProvisioningMeta().PlatformRef
	domain = res.GetProvisioningMeta().DomainRef
	target = res.GetProvisioningMeta().Target.Category

	if len(platform) == 0 || len(domain) == 0 {
		return platform, domain, target, false
	}

	return platform, domain, target, true
}
