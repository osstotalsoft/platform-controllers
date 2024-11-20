package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

type LocalScript struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LocalScriptSpec `json:"spec"`
}

type LocalScriptShell string

const (
	LocalScriptShellPwsh = LocalScriptShell("pwsh")
	LocalScriptShellSh   = LocalScriptShell("bash")
)

type LocalScriptSpec struct {

	// ScriptContent that runs on resource creation and update.
	CreateScriptContent string `json:"createScriptContent"`

	// ScriptContent that runs on resource deletion.
	DeleteScriptContent string `json:"deleteScriptContent"`

	// The shell to use for the script. Possibile values: pwsh, bash
	// +kubebuilder:validation:Enum=pwsh;bash
	// +kubebuilder:default:=pwsh
	Shell LocalScriptShell `json:"shell"`

	// Represents the environment variables to be passed to the script.
	// +optional
	Environment map[string]string `json:"environment,omitempty"`

	// +optional
	// Change value to force the script to execute even if it has not changed.
	ForceUpdateTag string `json:"forceUpdateTag,omitempty"`

	// Working directory for the script.
	// +optional
	WorkingDir string `json:"workingDir,omitempty"`

	// Export provisioning values spec.
	// +optional
	Exports          []LocalScriptExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type LocalScriptExportsSpec struct {

	// The domain or bounded-context in which this script will be used.
	// +optional
	Domain string `json:"domain"`

	// Represents the outputs of the Azure PowerShell script.
	// +optional
	ScriptOutput ValueExport `json:"scriptOutput,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LocalScriptList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LocalScript `json:"items"`
}

func (db *LocalScript) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *LocalScript) GetSpec() any {
	return &db.Spec
}
