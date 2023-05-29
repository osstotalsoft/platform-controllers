package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Service describes a business service.
type Service struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +required
	Spec ServiceSpec `json:"spec"`
}

type ServiceSpec struct {
	// PlatformRef is the target platform.
	// +required
	PlatformRef string `json:"platformRef"`
	// RequiredDomainRefs are the required business domains associated to this service.
	// +required
	RequiredDomainRefs []string `json:"requiredDomainRefs"`
	// OptionalDomainRefs are the optional business domains associated to this service.
	// +optional
	OptionalDomainRefs []string `json:"optionalDomainRefs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceList is a list of Services.
type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Service `json:"items"`
}
