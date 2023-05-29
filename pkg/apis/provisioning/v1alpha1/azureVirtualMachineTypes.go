package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
type AzureVirtualMachine struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureVirtualMachineSpec `json:"spec,omitempty"`
}

type AzureVirtualMachineSpec struct {
	// Target platform (custom resource name).
	// +required
	PlatformRef string `json:"platformRef"`
	// Business Domain that this resource is provision for.
	// +required
	DomainRef string `json:"domainRef"`
	// Virtual Machine name prefix. Will have platform and tenant suffix.
	VmName string `json:"vmName"`

	// The virtual machine size. Options available: https://learn.microsoft.com/en-us/azure/virtual-machines/sizes
	VmSize string `json:"vmSize"`

	// Possible values are Standard_LRS, StandardSSD_LRS or Premium_LRS.
	// +kubebuilder:validation:Enum=Standard_LRS;StandardSSD_LRS;Premium_LRS
	OSDiskType string `json:"osDiskType"`

	// Source OS disk snapshot
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Compute/galleries/MyGallery/images/ch-client-base/versions/2.0.0
	SourceImageId string `json:"sourceImageId"`

	// Subnet of the VNet used by the virtual machine
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/charismaonline.qa/providers/Microsoft.Network/virtualNetworks/charismaonline-vnet/subnets/default
	SubnetId string `json:"subnetId"`

	// RDP inbound connection CIDR or source IP range or * to match any IP. Tags such as ‘VirtualNetwork’, ‘AzureLoadBalancer’ and ‘Internet’ can also be used.
	// eg: 128.0.57.0/25
	RdpSourceAddressPrefix string `json:"rdpSourceAddressPrefix"`

	// Enable Trusted Launch security
	EnableTrustedLaunch bool `json:"enableTrustedLaunch"`

	// +optional
	Exports []AzureVirtualMachineExportsSpec `json:"exports,omitempty"`
}

type AzureVirtualMachineExportsSpec struct {
	// The domain or bounded-context in which this virtual machine will be used.
	Domain string `json:"domain"`
	// +optional
	VmName ValueExport `json:"vmName,omitempty"`
	// +optional
	ComputerName ValueExport `json:"computerName,omitempty"`
	// +optional
	PublicAddress ValueExport `json:"publicAddress,omitempty"`
	// +optional
	AdminUserName ValueExport `json:"adminUserName,omitempty"`
	// +optional
	AdminPassword ValueExport `json:"adminPassword,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureVirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureVirtualMachine `json:"items"`
}
