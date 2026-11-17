package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Domain struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +required
	Spec DomainSpec `json:"spec"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.visibleInPortal) || !self.visibleInPortal || (has(self.url) && self.url != '')",message="url is required when visibleInPortal is true"
type DomainSpec struct {
	// PlatformRef is the target platform.
	// +required
	PlatformRef string `json:"platformRef"`

	// +optional
	ExportActiveDomains bool `json:"exportActiveDomains"`

	// Url of the domain's application. A template rendered with the tenant context,
	// e.g. "https://{{ .Tenant.Code }}.example.com/app". Required when VisibleInPortal is true.
	// +optional
	Url string `json:"url,omitempty"`

	// VisibleInPortal shows the domain in the portal, linking to Url.
	// +optional
	// +kubebuilder:default:=false
	VisibleInPortal bool `json:"visibleInPortal"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DomainList is a list of Domains.
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Domain `json:"items"`
}
