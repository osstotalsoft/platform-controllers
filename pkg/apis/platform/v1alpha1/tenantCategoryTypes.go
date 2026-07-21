package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`

// TenantCategory describes a classification that tenants can optionally reference
// (e.g. country, business typology, tenant group).
type TenantCategory struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec TenantCategorySpec `json:"spec"`
}

// TenantCategorySpec is the spec for a tenant category.
type TenantCategorySpec struct {
	// PlatformRef is the target platform. A TenantCategory is only resolvable by tenants
	// belonging to the same platform.
	// +required
	PlatformRef string `json:"platformRef"`

	// +optional
	Description string `json:"description,omitempty"`

	// ProvisioningOverrides contains a list of resource overrides to be applied during provisioning,
	// for every tenant referencing this category.
	// +optional
	ProvisioningOverrides []ProvisioningResourcePatch `json:"provisioningOverrides,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TenantCategoryList is a list of TenantCategories.
type TenantCategoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TenantCategory `json:"items"`
}

func (c *TenantCategory) GetDescription() string {
	return c.Spec.Description
}
