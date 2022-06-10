package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=string,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
type ConfigurationDomain struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationDomainSpec   `json:"spec"`
	Status ConfigurationDomainStatus `json:"status,omitempty"`
}

type ConfigurationDomainSpec struct {
	PlatformRef string `json:"platformRef"`
	// +optional
	AggregateConfigMaps bool `json:"aggregateConfigMaps,omitempty"`
	// +optional
	AggregateSecrets bool `json:"aggregateSecrets,omitempty"`
}

type ConfigurationDomainStatus struct {
	// Conditions holds the conditions for the ConfigurationDomain.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConfigurationDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ConfigurationDomain `json:"items"`
}
