package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

type Platform struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformSpec   `json:"spec,omitempty"`
	Status PlatformStatus `json:"status,omitempty"`
}

type PlatformSpec struct {
	TargetNamespace string `json:"targetNamespace"`

	// SecretStore is where the values the platform's provisioning resources export with toVault are
	// written, and where its configuration domains aggregate secrets from. Absent, it is HashiCorp Vault.
	// +optional
	SecretStore *SecretStoreSpec `json:"secretStore,omitempty"`
}

// SecretStoreType is the kind of secret store a platform uses.
type SecretStoreType string

const (
	// SecretStoreTypeHashicorpVault stores secrets in HashiCorp Vault, under <platform>/<namespace>/<domain>/.
	SecretStoreTypeHashicorpVault SecretStoreType = "HashicorpVault"
	// SecretStoreTypeAzureKeyVault stores every secret as its own Azure Key Vault secret, tagged with where it
	// comes from.
	SecretStoreTypeAzureKeyVault SecretStoreType = "AzureKeyVault"
)

// SecretStoreSpec is a platform's secret store: exactly one per platform.
// +kubebuilder:validation:XValidation:rule="(has(self.type) && self.type == 'AzureKeyVault') == has(self.azureKeyVault)",message="azureKeyVault is required exactly when type is AzureKeyVault"
type SecretStoreSpec struct {
	// Type is the kind of secret store.
	// +kubebuilder:validation:Enum=HashicorpVault;AzureKeyVault
	// +kubebuilder:default=HashicorpVault
	// +optional
	Type SecretStoreType `json:"type,omitempty"`

	// AzureKeyVault is the platform's vault, required when Type is AzureKeyVault.
	// +optional
	AzureKeyVault *AzureKeyVaultSpec `json:"azureKeyVault,omitempty"`
}

// SecretStoreType returns the type of the platform's secret store: HashicorpVault when it has none.
func (s PlatformSpec) SecretStoreType() SecretStoreType {
	if s.SecretStore == nil || s.SecretStore.Type == "" {
		return SecretStoreTypeHashicorpVault
	}
	return s.SecretStore.Type
}

// AzureKeyVault returns the platform's Azure Key Vault, or nil when its secret store is not Azure Key Vault.
func (s PlatformSpec) AzureKeyVault() *AzureKeyVaultSpec {
	if s.SecretStoreType() != SecretStoreTypeAzureKeyVault {
		return nil
	}
	return s.SecretStore.AzureKeyVault
}

// AzureKeyVaultSpec identifies an Azure Key Vault in the subscription given by AZURE_SUBSCRIPTION_ID.
type AzureKeyVaultSpec struct {
	// Name is the vault's name: 3-24 letters, digits and dashes, starting with a letter and ending
	// with a letter or digit.
	// +kubebuilder:validation:Pattern=`^[A-Za-z][0-9A-Za-z-]{1,22}[0-9A-Za-z]$`
	Name string `json:"name"`

	// ResourceGroupName is the name of the resource group that contains the vault.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=90
	ResourceGroupName string `json:"resourceGroupName"`

	// WorkloadIdentityClientId is the client ID of the managed identity the platform's pods read the vault's
	// secrets with, through the Secrets Store CSI driver. It needs Key Vault Secrets User on the vault, and an
	// AKS identity binding on every cluster running the platform.
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	WorkloadIdentityClientId string `json:"workloadIdentityClientId"`
}

type PlatformStatus struct {
	// Conditions holds the conditions for the HelmRepository.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlatformList is a list of Platforms.
type PlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Platform `json:"items"`
}

func (platform *Platform) GetDescription() string {
	return platform.GetName()
}

func (platform *Platform) GetPlatformName() string {
	return platform.GetName()
}
