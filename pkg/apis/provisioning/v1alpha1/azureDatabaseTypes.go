package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domains",type=string,JSONPath=`.spec.domains`

type AzureDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureDatabaseSpec `json:"spec"`
}

type AzureDatabaseSpec struct {
	// Target platform (custom resource name).
	PlatformRef string `json:"platformRef"`
	// The domain or bounded-context in which this database will be used.
	Domains []string `json:"domains"`
	// Database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// +optional
	Sku string `json:"sku,omitempty"`
	// +optional
	Exports AzureDatabaseExportsSpec `json:"exports,omitempty"`
}

type AzureDatabaseExportsSpec struct {
	// +optional
	UserName ValueExport `json:"userName,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureDatabase `json:"items"`
}
