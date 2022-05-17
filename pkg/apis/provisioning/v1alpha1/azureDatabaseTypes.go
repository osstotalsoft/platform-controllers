package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domain`

type AzureDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureDatabaseSpec `json:"spec"`
}

type AzureDatabaseSpec struct {
	PlatformRef string `json:"platformRef"`
	Domain      string `json:"domain"`
	DbName      string `json:"dbName"`
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
