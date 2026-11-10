package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Container name",type=string,JSONPath=`.spec.containerName`
// +kubebuilder:printcolumn:name="Storage account",type=string,JSONPath=`.spec.storageAccount.accountName`
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

// AzureBlobContainer provisions an Azure Blob Storage container in an existing storage account, one
// per tenant (or platform): the Azure alternative to MinioBucket.
type AzureBlobContainer struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureBlobContainerSpec `json:"spec"`
}

type AzureBlobContainerSpec struct {
	// Container name prefix. Will have platform and tenant suffix, e.g. "files-dev-tenant1". Dots and
	// underscores become hyphens; the result must be a valid container name (3-63 lowercase letters,
	// digits and single hyphens).
	ContainerName string `json:"containerName"`

	// Existing container to be adopted instead of creating a new one.
	// When set, it replaces the generated container name.
	// eg: "my-existing-container"
	// +optional
	ImportContainerName string `json:"importContainerName,omitempty"`

	// The existing storage account the container is created in.
	StorageAccount StorageAccountSpec `json:"storageAccount"`

	// Validity, in days, of the container SAS exported as containerSasUrl. A new SAS is issued every
	// 30 days, each valid for this many days, so the previous one keeps working while apps pick up the
	// new value. Defaults to 365.
	// +kubebuilder:validation:Minimum=31
	// +optional
	SasValidityDays int `json:"sasValidityDays,omitempty"`

	// Optional Entra (Azure AD) user-assigned managed identities granted a data-plane role on the
	// container. One entry per consuming application, each with a unique name.
	// +listType=map
	// +listMapKey=name
	// +optional
	ManagedIdentities []BlobContainerIdentitySpec `json:"managedIdentities,omitempty"`

	// Export provisioning values spec.
	// +optional
	Exports          []AzureBlobContainerExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type StorageAccountSpec struct {
	// Storage account resource group.
	ResourceGroupName string `json:"resourceGroupName"`
	// Storage account name.
	AccountName string `json:"accountName"`
}

// BlobContainerRole is a built-in Azure data-plane role granted on a container.
// +kubebuilder:validation:Enum=StorageBlobDataContributor;StorageBlobDataReader;StorageBlobDataOwner
type BlobContainerRole string

const (
	BlobContainerRoleContributor = BlobContainerRole("StorageBlobDataContributor")
	BlobContainerRoleReader      = BlobContainerRole("StorageBlobDataReader")
	BlobContainerRoleOwner       = BlobContainerRole("StorageBlobDataOwner")
)

// BlobContainerIdentitySpec describes an Entra (Azure AD) user-assigned managed identity granted a
// role on the container, which AKS Workload Identity ServiceAccounts can authenticate as.
type BlobContainerIdentitySpec struct {
	// Identity name. Must be unique within the owning resource's managedIdentities list.
	// +required
	Name string `json:"name"`
	// Resource group the managed identity is created in.
	ResourceGroupName string `json:"resourceGroupName"`
	// Azure region.
	Location string `json:"location"`
	// Role granted to this identity on the container. Defaults to StorageBlobDataContributor.
	// +optional
	Role BlobContainerRole `json:"role,omitempty"`
	// ServiceAccounts that may authenticate as this identity via AKS Workload Identity: each gets a
	// federated credential trusting AZURE_AKS_OIDC_ISSUER_URL.
	// +optional
	// +listType=atomic
	ServiceAccounts []FederatedServiceAccount `json:"serviceAccounts,omitempty"`
}

func (m BlobContainerIdentitySpec) GetName() string { return m.Name }

type AzureBlobContainerExportsSpec struct {
	// The domain or bounded-context in which this container will be used. Defaults to domainRef.
	// +optional
	Domain string `json:"domain,omitempty"`

	// The container name.
	// +optional
	ContainerName ValueExport `json:"containerName,omitempty"`
	// The storage account name.
	// +optional
	AccountName ValueExport `json:"accountName,omitempty"`
	// The storage account's blob endpoint, e.g. https://account.blob.core.windows.net/
	// +optional
	BlobEndpoint ValueExport `json:"blobEndpoint,omitempty"`
	// The container URL, e.g. https://account.blob.core.windows.net/container
	// +optional
	ContainerUrl ValueExport `json:"containerUrl,omitempty"`
	// The container URL with a SAS token granting read, add, create, write, delete and list on this
	// container only. Secret: export it to the vault or a Kubernetes secret.
	// +optional
	ContainerSasUrl ValueExport `json:"containerSasUrl,omitempty"`
	// Managed identities whose client/principal ids are exported to this domain, one item per
	// managedIdentities[] entry.
	// +optional
	Identities []ExportedIdentitySpec `json:"identities,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureBlobContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureBlobContainer `json:"items"`
}

func (c *AzureBlobContainer) GetProvisioningMeta() *ProvisioningMeta {
	return &c.Spec.ProvisioningMeta
}

func (c *AzureBlobContainer) GetSpec() any {
	return &c.Spec
}
