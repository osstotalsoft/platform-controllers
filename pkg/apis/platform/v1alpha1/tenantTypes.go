package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// Tenant describes an Rusi component type.
type Tenant struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// TenantSpec is the spec for a tenant.
type TenantSpec struct {
	Id          string `json:"id"`
	Code        string `json:"code"`
	PlatformRef string `json:"platformRef"`
}

// TenantStatus is the status for a tenant.
type TenantStatus struct {

	// LastResyncTime contains a timestamp for the last time a resync of the tenant took place.
	// +optional
	LastResyncTime metav1.Time `json:"lastResyncTime,omitempty"`

	// State is the state of the tenant update - one of `succeeded` or `failed`
	// +optional
	State string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TenantList is a list of Tenants.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Tenant `json:"items"`
}
