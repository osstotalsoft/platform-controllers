package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	Exports ExportsSpec `json:"exports,omitempty"`
}

type ExportsSpec struct {
	// +optional
	UserName ValueExport `json:"userName,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
}

type ValueExport struct {
	// +optional
	ToConfigMap ConfigMapTemplate `json:"toConfigMap,omitempty"`
	// +optional
	ToVault VaultSecretTemplate `json:"toVault,omitempty"`
}

type ConfigMapTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

type VaultSecretTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureDatabase `json:"items"`
}
