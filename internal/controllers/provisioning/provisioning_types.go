package provisioning

// Provisioner provisions the infrastructure of a domain for a target, a platform or a tenant.
type Provisioner interface {
	// Provision brings the infrastructure of domain for target in line with infra. Given no resources, it removes what
	// was provisioned before.
	Provision(target ProvisioningTarget, domain string, infra *InfrastructureManifests) ProvisioningResult
	// IsProvisioned tells whether domain has been provisioned successfully for target, and not removed since. An error
	// means the answer couldn't be determined.
	IsProvisioned(target ProvisioningTarget, domain string) (bool, error)
}

type ProvisioningResult struct {
	Error      error
	HasChanges bool
}
