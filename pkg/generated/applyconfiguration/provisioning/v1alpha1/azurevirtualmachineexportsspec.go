/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

// AzureVirtualMachineExportsSpecApplyConfiguration represents an declarative configuration of the AzureVirtualMachineExportsSpec type for use
// with apply.
type AzureVirtualMachineExportsSpecApplyConfiguration struct {
	Domain        *string                        `json:"domain,omitempty"`
	VmName        *ValueExportApplyConfiguration `json:"vmName,omitempty"`
	ComputerName  *ValueExportApplyConfiguration `json:"computerName,omitempty"`
	PublicAddress *ValueExportApplyConfiguration `json:"publicAddress,omitempty"`
	AdminUserName *ValueExportApplyConfiguration `json:"adminUserName,omitempty"`
	AdminPassword *ValueExportApplyConfiguration `json:"adminPassword,omitempty"`
}

// AzureVirtualMachineExportsSpecApplyConfiguration constructs an declarative configuration of the AzureVirtualMachineExportsSpec type for use with
// apply.
func AzureVirtualMachineExportsSpec() *AzureVirtualMachineExportsSpecApplyConfiguration {
	return &AzureVirtualMachineExportsSpecApplyConfiguration{}
}

// WithDomain sets the Domain field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Domain field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithDomain(value string) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.Domain = &value
	return b
}

// WithVmName sets the VmName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmName field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithVmName(value *ValueExportApplyConfiguration) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.VmName = value
	return b
}

// WithComputerName sets the ComputerName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ComputerName field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithComputerName(value *ValueExportApplyConfiguration) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.ComputerName = value
	return b
}

// WithPublicAddress sets the PublicAddress field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PublicAddress field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithPublicAddress(value *ValueExportApplyConfiguration) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.PublicAddress = value
	return b
}

// WithAdminUserName sets the AdminUserName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the AdminUserName field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithAdminUserName(value *ValueExportApplyConfiguration) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.AdminUserName = value
	return b
}

// WithAdminPassword sets the AdminPassword field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the AdminPassword field is set to the value of the last call.
func (b *AzureVirtualMachineExportsSpecApplyConfiguration) WithAdminPassword(value *ValueExportApplyConfiguration) *AzureVirtualMachineExportsSpecApplyConfiguration {
	b.AdminPassword = value
	return b
}
