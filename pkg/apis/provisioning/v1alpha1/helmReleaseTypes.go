package v1alpha1

import (
	flux "github.com/fluxcd/helm-controller/api/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
type HelmRelease struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HelmReleaseSpec `json:"spec"`
}

type HelmReleaseSpec struct {
	// helm release spec
	Release flux.HelmReleaseSpec `json:"release"`
	// +optional
	Exports          []HelmReleaseExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type HelmReleaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	ReleaseName ValueExport `json:"releaseName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HelmRelease `json:"items"`
}

func (db *HelmRelease) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *HelmRelease) GetSpec() any {
	return &db.Spec
}

func (db *HelmRelease) Clone() any {
	return db.DeepCopy()
}
