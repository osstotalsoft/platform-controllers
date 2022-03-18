package provisioners

import provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"

type CreateInfrastructureFunc func(platform string,
	tenant *provisioningv1.Tenant,
	azureDbs []*provisioningv1.AzureDatabase) error
