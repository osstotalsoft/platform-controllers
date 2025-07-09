package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domains",type=string,JSONPath=`.spec.domains`

type MsSqlDatabase struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MsSqlDatabaseSpec `json:"spec"`
}

type MsSqlDatabaseSpec struct {
	// Database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// Sql Server spec. New database will be created on this server
	SqlServer MsSqlServerSpec `json:"sqlServer"`
	// Restore from backup. Leave empty for a new empty database.
	// +kubebuilder:default:={backupFilePath: ""}
	RestoreFrom MsSqlDatabaseRestoreSpec `json:"restoreFrom"`
	// Existing database to be used instead of creating a new one
	// eg: "2"
	// +optional
	ImportDatabaseId string `json:"importDatabaseId,omitempty"`
	// +optional
	Exports          []MsSqlDatabaseExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type MsSqlServerSpec struct {
	// Sql Server host name.
	HostName string `json:"hostName"`

	// Sql server endpoint port
	Port int `json:"port"`

	// Sql server authentication
	SqlAuth MsSqlServerAuth `json:"sqlAuth"`
}

type MsSqlServerAuth struct {
	// Sql server username
	Username string `json:"username"`

	// Sql server password
	Password string `json:"password"`
}

type MsSqlDatabaseRestoreSpec struct {
	//The backup file to restore from. Should be located on the SQL server machine.
	BackupFilePath string `json:"backupFilePath"`
	// The logical name of the data file in the backup.
	LogicalDataFileName string `json:"logicalDataFileName,omitempty"`
	// The logical name of the log file in the backup.
	LogicalLogFileName string `json:"logicalLogFileName,omitempty"`
}

type MsSqlDatabaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MsSqlDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MsSqlDatabase `json:"items"`
}

func (db *MsSqlDatabase) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *MsSqlDatabase) GetSpec() any {
	return &db.Spec
}
