package provisioning

type CreateInfrastructureFunc func(
	target ProvisioningTarget,
	domain string,
	infra *InfrastructureManifests) ProvisioningResult

type ProvisioningResult struct {
	Error      error
	HasChanges bool
}
