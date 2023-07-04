package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domains",type=string,JSONPath=`.spec.domains`

type AzureManagedDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureManagedDatabaseSpec `json:"spec,omitempty"`
}

type AzureManagedDatabaseSpec struct {
	// Managed database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// Target managed instance spec.
	ManagedInstance AzureManagedInstanceSpec `json:"managedInstance"`
	// Restore from external backup. Leave empty for a new empty database.
	// +optional
	RestoreFrom AzureManagedDatabaseRestoreSpec `json:"restoreFrom,omitempty"`
	// Existing database to be used instead of creating a new one
	// eg: /subscriptions/00000000-1111-2222-3333-444444444444/resourceGroups/Default-SQL-SouthEastAsia/providers/Microsoft.Sql/servers/testsvr/databases/testdb
	// +optional
	ImportDatabaseId string `json:"importDatabaseId,omitempty"`
	// Export provisioning values spec.
	// +optional
	Exports          []AzureManagedDatabaseExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
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
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureManagedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureManagedDatabase `json:"items"`
}

func (db *AzureManagedDatabase) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *AzureManagedDatabase) GetSpec() any {
	return &db.Spec
}
