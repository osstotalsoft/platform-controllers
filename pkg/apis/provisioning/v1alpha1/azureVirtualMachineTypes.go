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
	PlatformRef string `json:"platformRef"`
	// Virtual Machine name prefix. Will have platform and tenant suffix.
	VmName       string `json:"vmName"`
	VmSize       string `json:"vmSize"`
	ComputerName string `json:"computerName"`
	// Resource group of the resulting VM and associated resources
	ResourceGroup string `json:"resourceGroup"`

	// Source OS disk snapshot
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Compute/snapshots/myvm1_snapshot
	SourceSnapshotId string `json:"sourceSnapshot"`

	// Subnet of the VNet used by the virtual machine
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Network/virtualNetworks/myVm-vnet/subnets/default
	SubnetId string `json:"subnetId"`

	// Network Security Group
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Network/networkSecurityGroups/myVm-nsg
	NetworkSecurityGroupId string `json:"networkSecurityGroupId"`

	// Operating System type (Windows or Linux)
	// +kubebuilder:validation:Enum=Windows;Linux
	OsType string `json:"osType"`

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
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureVirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureVirtualMachine `json:"items"`
}
