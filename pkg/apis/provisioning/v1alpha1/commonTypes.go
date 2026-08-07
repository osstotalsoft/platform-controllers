package v1alpha1

import apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

type ValueExport struct {
	// +optional
	ToConfigMap ConfigMapTemplate `json:"toConfigMap,omitempty"`
	// +optional
	ToVault VaultSecretTemplate `json:"toVault,omitempty"`
	// +optional
	ToKubeSecret KubeSecretTemplate `json:"toKubeSecret,omitempty"`
}

type ConfigMapTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

type KubeSecretTemplate struct {
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

type ProvisioningFilterBy string

const (
	ProvisioningFilterByName     = ProvisioningFilterBy("Name")
	ProvisioningFilterByCategory = ProvisioningFilterBy("Category")
)

type ProvisioningTargetFilter struct {
	// Includes or excludes the speciffied targets. Possibile values: Blacklist, Whitelist
	// +kubebuilder:validation:Enum=Blacklist;Whitelist
	// +kubebuilder:default:=Blacklist
	Kind ProvisioningFilterKind `json:"kind"`

	// What tenant attribute the Values are matched against. Possible values: Name, Category
	// +kubebuilder:validation:Enum=Name;Category
	// +kubebuilder:default:=Name
	By ProvisioningFilterBy `json:"by,omitempty"`

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
	// Overrides for tenant category. Dictionary with category value (Tenant.spec.categoryRef) as exact key, spec override as value.
	// The spec override has the same structure as Spec. Applied before TenantCategory.spec.provisioningOverrides,
	// TenantOverrides and Tenant.spec.provisioningOverrides, all of which take precedence when they also match.
	// +optional
	CategoryOverrides map[string]*apiextensionsv1.JSON `json:"categoryOverrides,omitempty"`
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

// DatabaseUserSpec describes an optional app-facing database user. The password is never part of
// the spec — it is always auto-generated and exported alongside the username.
type DatabaseUserSpec struct {
	// Login/user name. Defaults to the provisioned database name if omitted.
	// +optional
	Name string `json:"name,omitempty"`
	// Database role(s) granted to this user (e.g. db_owner, db_datareader). No roles are granted if omitted.
	// +optional
	Roles []string `json:"roles,omitempty"`
}

// ManagedIdentitySpec describes an optional Entra (Azure AD) user-assigned managed identity wired
// in as a contained database user. Only applicable to Azure-native database kinds.
type ManagedIdentitySpec struct {
	// Identity name. Defaults to the provisioned database name if omitted.
	// +optional
	Name string `json:"name,omitempty"`
	// Resource group the managed identity is created in.
	ResourceGroupName string `json:"resourceGroupName"`
	// Azure region.
	Location string `json:"location"`
	// Database role(s) granted to this identity. No roles are granted if omitted.
	// +optional
	Roles []string `json:"roles,omitempty"`
}
