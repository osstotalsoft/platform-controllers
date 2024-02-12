package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

type AzurePowerShellScript struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzurePowerShellScriptSpec `json:"spec"`
}

type AzurePowerShellScriptSpec struct {

	// ScriptContent represents the content of an Azure PowerShell script.
	ScriptContent string `json:"scriptContent"`

	// Represents the arguments to be passed to the PowerShell script.
	// eg: "-name JohnDoe"
	// +optional
	ScriptArguments string `json:"scriptArguments,omitempty"`

	// +optional
	// Change value to force the script to execute even if it has not changed.
	ForceUpdateTag string `json:"forceUpdateTag,omitempty"`

	// Export provisioning values spec.
	// +optional
	Exports          []AzurePowerShellScriptExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type AzurePowerShellScriptExportsSpec struct {

	// The domain or bounded-context in which this script will be used.
	// +optional
	Domain string `json:"domain"`

	// Represents the outputs of the Azure PowerShell script.
	// +optional
	ScriptOutputs ValueExport `json:"scriptOutputs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzurePowerShellScriptList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzurePowerShellScript `json:"items"`
}

func (db *AzurePowerShellScript) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *AzurePowerShellScript) GetSpec() any {
	return &db.Spec
}
