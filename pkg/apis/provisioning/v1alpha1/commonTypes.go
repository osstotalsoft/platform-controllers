package v1alpha1

import apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

type ValueExport struct {
	// +optional
	ToConfigMap ConfigMapTemplate `json:"toConfigMap,omitempty"`
	// +optional
	ToVault VaultSecretTemplate `json:"toVault,omitempty"`
}

type ConfigMapTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

type VaultSecretTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

type ProvisioningMeta struct {
	// Target platform (custom resource name).
	// +required
	PlatformRef string `json:"platformRef"`
	// Business Domain that this resource is provision for.
	// +required
	DomainRef string `json:"domainRef"`
	// Overrides for tenants. Dictionary with tenant name as key, spec override as value.
	// The spec override has the same structure as Spec
	// +optional
	TenantOverrides map[string]*apiextensionsv1.JSON `json:"tenantOverrides,omitempty"`
}

func (meta *ProvisioningMeta) GetTenantOverrides() map[string]*apiextensionsv1.JSON {
	return meta.TenantOverrides
}
