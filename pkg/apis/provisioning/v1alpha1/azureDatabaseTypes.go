package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	// Existing database to be used instead of creating a new one
	// eg: /subscriptions/00000000-1111-2222-3333-444444444444/resourceGroups/Default-SQL-SouthEastAsia/providers/Microsoft.Sql/servers/testsvr/databases/testdb
	// +optional
	ImportDatabaseId string `json:"importDatabaseId,omitempty"`
	// Optional app-facing database users (contained, password-based — AzureDatabase has no
	// server-login concept). One entry per consuming application, each with a unique name.
	// +listType=map
	// +listMapKey=name
	// +optional
	Users []DatabaseUserSpec `json:"users,omitempty"`
	// Optional Entra (Azure AD) user-assigned managed identities, each wired in as a contained AAD
	// database user. One entry per consuming application, each with a unique name.
	// +listType=map
	// +listMapKey=name
	// +optional
	ManagedIdentities []ManagedIdentitySpec `json:"managedIdentities,omitempty"`
	// +optional
	Exports          []AzureDatabaseExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type TenanantOverridesMeta struct {
	TenantOverrides map[string]*apiextensionsv1.JSON `json:"tenantOverrides,omitempty"`
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
	// Name of the users[] entry whose username/password are exported below. Optional when users has
	// exactly one entry (defaults to it); required to disambiguate when users has more than one entry.
	// +optional
	UserRef string `json:"userRef,omitempty"`
	// +optional
	Username ValueExport `json:"username,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
	// Name of the managedIdentities[] entry whose identity is exported below. Same defaulting rule
	// as userRef.
	// +optional
	IdentityRef string `json:"identityRef,omitempty"`
	// +optional
	IdentityClientId ValueExport `json:"identityClientId,omitempty"`
	// +optional
	IdentityPrincipalId ValueExport `json:"identityPrincipalId,omitempty"`
}

// GetDomain returns the export's domain, satisfying the validateUniqueDomains helper's domained[T]
// constraint in mssql_user.go.
func (e AzureDatabaseExportsSpec) GetDomain() string { return e.Domain }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureDatabase `json:"items"`
}

func (db *AzureDatabase) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *AzureDatabase) GetSpec() any {
	return &db.Spec
}
