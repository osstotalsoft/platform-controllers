package provisioning

import (
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type ProvisioningResource interface {
	*provisioningv1.AzureDatabase | *provisioningv1.AzureManagedDatabase | *provisioningv1.HelmRelease | *provisioningv1.AzureVirtualMachine | *provisioningv1.AzureVirtualDesktop
	GetName() string
	GetNamespace() string
	GetPlatformRef() string
	GetDomainRef() string
}

func getPlatformAndDomain[R ProvisioningResource](res R) (platform, domain string, ok bool) {
	platform = res.GetPlatformRef()
	domain = res.GetDomainRef()
	if len(platform) == 0 || len(domain) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}

func selectItemsInPlatformAndDomain[R ProvisioningResource](platform, domain string, source []R) []R {
	result := []R{}
	for _, res := range source {
		if res.GetPlatformRef() == platform && res.GetDomainRef() == domain {
			result = append(result, res)
		}
	}
	return result
}
