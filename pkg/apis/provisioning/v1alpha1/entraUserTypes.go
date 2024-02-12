package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Display name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="User principal name",type=string,JSONPath=`.spec.userPrincipalName`
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

type EntraUser struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec EntraUserSpec `json:"spec"`
}

type EntraUserSpec struct {
	// UserPrincipalName represents the user principal name, e.g. "jdoe@domain.com"
	UserPrincipalName string `json:"userPrincipalName"`

	// InitialPassword represents the initial password for the user
	// +optional
	InitialPassword string `json:"initialPassword,omitempty"`

	// DisplayName represents the display name of the user, e.g. "John Doe"
	DisplayName string `json:"displayName"`

	// Export provisioning values spec.
	// +optional
	Exports          []EntraUserExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type EntraUserExportsSpec struct {

	// The domain or bounded-context in which this user will be used.
	// +optional
	Domain string `json:"domain"`

	// The initial password for the user
	InitialPassword ValueExport `json:"initialPassword,omitempty"`

	// The user principal name
	UserPrincipalName ValueExport `json:"userPrincipalName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type EntraUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EntraUser `json:"items"`
}

func (db *EntraUser) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *EntraUser) GetSpec() any {
	return &db.Spec
}
