package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureManagedDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureManagedDatabaseSpec `json:"spec,omitempty"`
}

type AzureManagedDatabaseSpec struct {
	PlatformRef     string               `json:"platformRef"`
	DbName          string               `json:"dbName"`
	ManagedInstance AzureManagedInstance `json:"managedInstance"`
}
type AzureManagedInstance struct {
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureManagedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureManagedDatabase `json:"items"`
}
