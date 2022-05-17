package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domain`

type AzureManagedDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureManagedDatabaseSpec `json:"spec,omitempty"`
}

type AzureManagedDatabaseSpec struct {
	// Target platform (custom resource name).
	PlatformRef string `json:"platformRef"`
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// Managed database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// Target managed instance spec.
	ManagedInstance AzureManagedInstanceSpec `json:"managedInstance"`
	// Restore from external backup. Leave empty for a new empty database.
	// +optional
	RestoreFrom AzureManagedDatabaseRestoreSpec `json:"restoreFrom,omitempty"`
	// Export provisioning values spec.
	// +optional
	Exports AzureManagedDatabaseExportsSpec `json:"exports,omitempty"`
}
type AzureManagedInstanceSpec struct {
	// Managed instance name.
	Name string `json:"name"`
	// Managed instance resource group.
	ResourceGroup string `json:"resourceGroup"`
}

type AzureManagedDatabaseRestoreSpec struct {
	//The backup file to restore from.
	BackupFileName string `json:"backupFileName"`
	//Azure storage container spec.
	StorageContainer AzureStorageContainerSpec `json:"storageContainer"`
}

type AzureStorageContainerSpec struct {
	// The storage container shared access signature token.
	SasToken string `json:"sasToken"`
	// The storage container uri.
	Uri string `json:"uri"`
}

type AzureManagedDatabaseExportsSpec struct {
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureManagedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureManagedDatabase `json:"items"`
}
