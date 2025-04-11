package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Bucket name",type=string,JSONPath=`.spec.bucketName`
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

type MinioBucket struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MinioBucketSpec `json:"spec"`
}

type MinioBucketSpec struct {
	// UserPrincipalName represents the user principal name, e.g. "jdoe@domain.com"
	BucketName string `json:"bucketName"`

	// Export provisioning values spec.
	// +optional
	Exports          []MinioBucketExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

type MinioBucketExportsSpec struct {
	// The domain or bounded-context in which this user will be used.
	// +optional
	Domain string `json:"domain"`

	BucketName ValueExport `json:"bucketName,omitempty"`
	AccessKey  ValueExport `json:"accessKey,omitempty"`
	SecretKey  ValueExport `json:"secretKey,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MinioBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MinioBucket `json:"items"`
}

func (db *MinioBucket) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *MinioBucket) GetSpec() any {
	return &db.Spec
}
