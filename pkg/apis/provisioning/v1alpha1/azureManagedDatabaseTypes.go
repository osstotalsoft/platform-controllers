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
	// Target platform (custom resource name).
	PlatformRef string `json:"platformRef"`
	// Managed database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// Target managed instance spec
	ManagedInstance AzureManagedInstance `json:"managedInstance"`
}
type AzureManagedInstance struct {
	// Managed instance name
	Name string `json:"name"`
	// Managed instance ressource group
	ResourceGroup string `json:"resourceGroup"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureManagedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureManagedDatabase `json:"items"`
}
