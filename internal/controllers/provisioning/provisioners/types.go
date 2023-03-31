package provisioners

import (
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type CreateInfrastructureFunc func(platform string,
	tenant *platformv1.Tenant,
	infra *InfrastructureManifests) ProvisioningResult

type InfrastructureManifests struct {
	AzureDbs        []*provisioningv1.AzureDatabase
	AzureManagedDbs []*provisioningv1.AzureManagedDatabase
	HelmReleases    []*provisioningv1.HelmRelease
}

type ProvisioningResult struct {
	Error      error
	HasChanges bool
}
