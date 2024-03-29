package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="LastResync",type=date,JSONPath=`.status.lastResyncTime`

// Tenant describes an Application tenant component type.
type Tenant struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   TenantSpec   `json:"spec"`
	Status TenantStatus `json:"status,omitempty"`
}

// TenantSpec is the spec for a tenant.
type TenantSpec struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	// PlatformRef is the target platform.
	// +required
	PlatformRef string `json:"platformRef"`

	// +kubebuilder:default:=true
	Enabled bool `json:"enabled"`
	// DomainRefs are the business domains associated to this tenant.
	// +optional
	DomainRefs []string `json:"domainRefs,omitempty"`

	// Tenant administrator email address.
	// +required
	AdminEmail string `json:"adminEmail"`

	// Possible values are RetainStatefulResources (retain stateful provisioned resources), DeleteAll (delete all resources).
	// +kubebuilder:validation:Enum=RetainStatefulResources;DeleteAll
	// +kubebuilder:default:=RetainStatefulResources
	DeletePolicy DeletePolicy `json:"deletePolicy"`

	// Tenant specific configs.
	// +optional
	Configs map[string]string `json:"configs,omitempty"`
}

// TenantStatus is the status for a tenant.
type TenantStatus struct {

	// LastResyncTime contains a timestamp for the last time a resync of the tenant took place.
	// +optional
	LastResyncTime metav1.Time `json:"lastResyncTime,omitempty"`

	// Condition contains details for one aspect of the current state of this API Resource
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TenantList is a list of Tenants.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Tenant `json:"items"`
}

type DeletePolicy string

const (
	DeletePolicyRetainStatefulResources = DeletePolicy("RetainStatefulResources")
	DeletePolicyDeleteAll               = DeletePolicy("DeleteAll")
)

func (tenant *Tenant) GetDescription() string {
	return tenant.Spec.Description
}

func (tenant *Tenant) GetPlatformName() string {
	return tenant.Spec.PlatformRef
}
