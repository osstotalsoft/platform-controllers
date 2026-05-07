package v1alpha1

import (
	flux "github.com/fluxcd/helm-controller/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
type HelmReleaseV2 struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HelmReleaseV2Spec `json:"spec"`
}

type HelmReleaseV2Spec struct {
	// helm release spec
	Release flux.HelmReleaseSpec `json:"release"`
	// +optional
	Exports          []HelmReleaseV2ExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type HelmReleaseV2ExportsSpec struct {
	// The domain or bounded-context for which HelmReleaseV2 values are exported.
	Domain string `json:"domain"`
	// +optional
	ReleaseName ValueExport `json:"releaseName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HelmReleaseV2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HelmReleaseV2 `json:"items"`
}

func (hr *HelmReleaseV2) GetProvisioningMeta() *ProvisioningMeta {
	return &hr.Spec.ProvisioningMeta
}

func (hr *HelmReleaseV2) GetSpec() any {
	return &hr.Spec
}
