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
type DomainConfigurator struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainConfiguratorSpec   `json:"spec"`
	Status DomainConfiguratorStatus `json:"status,omitempty"`
}

type DomainConfiguratorIntegrationType string

const (
	DomainConfiguratorIntegrationTypeMessaging = DomainConfiguratorIntegrationType("Messaging")
	DomainConfiguratorIntegrationTypeHttp      = DomainConfiguratorIntegrationType("Http")
)

type DomainConfigurationMessaging struct {
	CommandTopic      string `json:"commandTopic"`
	SuccessEventTopic string `json:"successEventTopic"`
	FailedEventTopic  string `json:"failedEventTopic"`
}

type DomainConfiguratorSpec struct {
	PlatformRef string `json:"platformRef"`

	// Selects the integration type. Possibile values: Messaging, Http
	// +kubebuilder:validation:Enum=Messaging;Http
	// +kubebuilder:default:=Messaging
	IntegrationType DomainConfiguratorIntegrationType `json:"integrationType"`

	// Messaging settings (applies for integration type "Messaging").
	// +optional
	Messaging DomainConfigurationMessaging `json:"messaging"`
}

type DomainConfiguratorStatus struct {
	// Conditions holds the conditions for the ConfigurationDomain.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConfigurationDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []DomainConfigurator `json:"items"`
}
