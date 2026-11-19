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

	// AzureKeyVault is the platform's Azure Key Vault. Values exported with toAzureKeyVault by the
	// platform's provisioning resources are written to it, and the platform's configuration domains
	// read them back from it. Platforms without it cannot use toAzureKeyVault.
	// +optional
	AzureKeyVault *AzureKeyVaultSpec `json:"azureKeyVault,omitempty"`
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
