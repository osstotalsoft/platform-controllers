package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
type AzureVirtualDesktop struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzureVirtualDesktopSpec `json:"spec,omitempty"`
}

type AzureVirtualDesktopSpec struct {
	// Virtual Desktop name prefix. Will have platform and tenant suffix.
	HostPoolName string `json:"hostPoolName"`

	// Session Host VM name prefix. Will have platform and tenant suffix.
	VmNamePrefix string `json:"vmNamePrefix"`

	// The virtual machine size. Options available: https://learn.microsoft.com/en-us/azure/virtual-machines/sizes
	VmSize string `json:"vmSize"`

	// The number of virtual machines to be added to the host pool
	VmNumberOfInstances int `json:"vmNumberOfInstances"`

	// Virtual Machine Gallery Applications
	VmApplications []VirtualMachineGalleryApplication `json:"vmApplications"`

	// Possible values are Standard_LRS, StandardSSD_LRS or Premium_LRS.
	// +kubebuilder:validation:Enum=Standard_LRS;StandardSSD_LRS;Premium_LRS
	OSDiskType string `json:"osDiskType"`

	// Source OS disk snapshot
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Compute/galleries/MyGallery/images/ch-client-base/versions/2.0.0
	SourceImageId string `json:"sourceImageId"`

	// Subnet of the VNet used by the virtual machine
	// eg: /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/charismaonline.qa/providers/Microsoft.Network/virtualNetworks/charismaonline-vnet/subnets/default
	SubnetId string `json:"subnetId"`

	// Enable Trusted Launch security
	EnableTrustedLaunch bool `json:"enableTrustedLaunch"`

	// Initialization script
	InitScript string `json:"initScript"`

	// Initialization script arguments
	InitScriptArguments []InitScriptArgs `json:"initScriptArgs"`

	// The name of the workspace to be displayed in the client application
	WorkspaceFriendlyName string `json:"workspaceFriendlyName"`

	// Applications
	Applications []AzureVirtualDesktopApplication `json:"applications"`

	// +optional
	Users AzureVirtualDesktopUsersSpec `json:"users"`

	// +optional
	Exports []AzureVirtualDesktopExportsSpec `json:"exports,omitempty"`

	ProvisioningMeta `json:",inline"`
}

type AzureVirtualDesktopExportsSpec struct {
	// The domain or bounded-context in which this virtual desktop will be used.
	Domain string `json:"domain"`
	// +optional
	HostPoolName ValueExport `json:"hostPoolName,omitempty"`
	// +optional
	ComputerName ValueExport `json:"computerName,omitempty"`
	// // +optional
	// PublicAddress ValueExport `json:"publicAddress,omitempty"`
	// +optional
	AdminUserName ValueExport `json:"adminUserName,omitempty"`
	// +optional
	AdminPassword ValueExport `json:"adminPassword,omitempty"`
}

type AzureVirtualDesktopUsersSpec struct {
	// +optional
	Admins []string `json:"admins,omitempty"`
	// +optional
	ApplicationUsers []string `json:"applicationUsers,omitempty"`
}

type AzureVirtualDesktopApplication struct {
	Name         string `json:"name"`
	FriendlyName string `json:"friendlyName"`
	Path         string `json:"path"`
}

type VirtualMachineGalleryApplication struct {

	// Source gallery application id or application version id
	// eg: subscriptions/15b38e46-ef41-4f5b-bdba-7d9354568c2d/resourceGroups/test-vm/providers/Microsoft.Compute/galleries/lfgalery/applications/charisma-client/versions/4.33.0
	PackageId string `json:"packageId"`

	// Installation order index. Eg: 1, 2, 3
	InstallOrderIndex int `json:"installOrderIndex"`
}

type InitScriptArgs struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureVirtualDesktopList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureVirtualDesktop `json:"items"`
}

func (db *AzureVirtualDesktop) GetProvisioningMeta() *ProvisioningMeta {
	return &db.Spec.ProvisioningMeta
}

func (db *AzureVirtualDesktop) GetSpec() any {
	return &db.Spec
}
