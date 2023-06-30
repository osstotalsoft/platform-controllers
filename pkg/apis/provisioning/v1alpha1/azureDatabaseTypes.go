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
	// +required
	PlatformRef string `json:"platformRef"`
	// Business Domain that this resource is provision for.
	// +required
	DomainRef string `json:"domainRef"`
	// Database name prefix. Will have platform and tenant suffix.
	DbName string `json:"dbName"`
	// Azure Sql Server spec. New database will be created on this server
	SqlServer SqlServerSpec `json:"sqlServer"`
	// +optional
	Sku string `json:"sku,omitempty"`
	// Source database from which a new database is copied
	// eg: /subscriptions/00000000-1111-2222-3333-444444444444/resourceGroups/Default-SQL-SouthEastAsia/providers/Microsoft.Sql/servers/testsvr/databases/testdb
	// +optional
	SourceDatabaseId string `json:"sourceDatabaseId,omitempty"`
	// +optional
	Exports []AzureDatabaseExportsSpec `json:"exports,omitempty"`
}

type SqlServerSpec struct {
	// Azure Sql Server resource group.
	ResourceGroupName string `json:"resourceGroupName"`
	// Azure Sql Server name.
	ServerName string `json:"serverName"`
	// +optional
	ElasticPoolName string `json:"elasticPoolName,omitempty"`
}

type AzureDatabaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureDatabase `json:"items"`
}

func (db *AzureDatabase) GetPlatformRef() string {
	return db.Spec.PlatformRef
}

func (db *AzureDatabase) GetDomainRef() string {
	return db.Spec.DomainRef
}
