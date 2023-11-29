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

type ProvisioningFilterKind string

const (
	ProvisioningFilterKindBlacklist = ProvisioningFilterKind("Blacklist")
	ProvisioningFilterKindWhitelist = ProvisioningFilterKind("Whitelist")
)

type ProvisioningTargetFilter struct {
	// Includes or excludes the speciffied targets. Possibile values: Blacklist, Whitelist
	// +kubebuilder:validation:Enum=Blacklist;Whitelist
	// +kubebuilder:default:=Blacklist
	Kind ProvisioningFilterKind `json:"kind"`

	// A list of targets to include or exculde
	Values []string `json:"values,omitempty"`
}

type ProvisioningTargetCategory string

const (
	ProvisioningTargetCategoryTenant   = ProvisioningTargetCategory("Tenant")
	ProvisioningTargetCategoryPlatform = ProvisioningTargetCategory("Platform")
)

type ProvisioningTarget struct {
	// Provisioning target type. Possible values: Tenant, Platform
	// +kubebuilder:validation:Enum=Tenant;Platform
	// +kubebuilder:default:=Tenant
	Category ProvisioningTargetCategory `json:"category"`

	// Filter targets (applies for category "Tenant").
	// If ommited all targets are selected.
	// +optional
	Filter ProvisioningTargetFilter `json:"filter"`
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
	// The provisioning target.
	// +kubebuilder:default:={category: "Tenant"}
	Target ProvisioningTarget `json:"target"`
	// List of dependencies
	// +optional
	DependsOn []ProvisioningResourceIdendtifier `json:"dependsOn,omitempty"`
}

type ProvisioningResourceIdendtifier struct {
	// Kind is a string value representing the REST resource this dependency represents.
	// +required
	Kind ProvisioningResourceKind `json:"kind"`
	//  The name of the dependency.
	// +required
	Name string `json:"name"`
}

type ProvisioningResourceKind string

const (
	ProvisioningResourceKindAzureDatabase        = ProvisioningResourceKind("AzureDatabase")
	ProvisioningResourceKindAzureManagedDatabase = ProvisioningResourceKind("AzureManagedDatabase")
	ProvisioningResourceKindAzureVirtualDesktop  = ProvisioningResourceKind("AzureVirtualDesktop")
	ProvisioningResourceKindAzureVirtualMachine  = ProvisioningResourceKind("AzureVirtualMachine")
	ProvisioningResourceKindHelmRelease          = ProvisioningResourceKind("HelmRelease")
)
