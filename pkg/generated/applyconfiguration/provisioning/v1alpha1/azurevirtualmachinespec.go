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

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// AzureVirtualMachineSpecApplyConfiguration represents an declarative configuration of the AzureVirtualMachineSpec type for use
// with apply.
type AzureVirtualMachineSpecApplyConfiguration struct {
	VmName                             *string `json:"vmName,omitempty"`
	VmSize                             *string `json:"vmSize,omitempty"`
	OSDiskType                         *string `json:"osDiskType,omitempty"`
	SourceImageId                      *string `json:"sourceImageId,omitempty"`
	SubnetId                           *string `json:"subnetId,omitempty"`
	RdpSourceAddressPrefix             *string `json:"rdpSourceAddressPrefix,omitempty"`
	EnableTrustedLaunch                *bool   `json:"enableTrustedLaunch,omitempty"`
	ProvisioningMetaApplyConfiguration `json:",inline"`
	Exports                            []AzureVirtualMachineExportsSpecApplyConfiguration `json:"exports,omitempty"`
}

// AzureVirtualMachineSpecApplyConfiguration constructs an declarative configuration of the AzureVirtualMachineSpec type for use with
// apply.
func AzureVirtualMachineSpec() *AzureVirtualMachineSpecApplyConfiguration {
	return &AzureVirtualMachineSpecApplyConfiguration{}
}

// WithVmName sets the VmName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmName field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithVmName(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.VmName = &value
	return b
}

// WithVmSize sets the VmSize field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmSize field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithVmSize(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.VmSize = &value
	return b
}

// WithOSDiskType sets the OSDiskType field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the OSDiskType field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithOSDiskType(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.OSDiskType = &value
	return b
}

// WithSourceImageId sets the SourceImageId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SourceImageId field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithSourceImageId(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.SourceImageId = &value
	return b
}

// WithSubnetId sets the SubnetId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SubnetId field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithSubnetId(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.SubnetId = &value
	return b
}

// WithRdpSourceAddressPrefix sets the RdpSourceAddressPrefix field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RdpSourceAddressPrefix field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithRdpSourceAddressPrefix(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.RdpSourceAddressPrefix = &value
	return b
}

// WithEnableTrustedLaunch sets the EnableTrustedLaunch field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the EnableTrustedLaunch field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithEnableTrustedLaunch(value bool) *AzureVirtualMachineSpecApplyConfiguration {
	b.EnableTrustedLaunch = &value
	return b
}

// WithPlatformRef sets the PlatformRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PlatformRef field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithPlatformRef(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithDomainRef sets the DomainRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DomainRef field is set to the value of the last call.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithDomainRef(value string) *AzureVirtualMachineSpecApplyConfiguration {
	b.DomainRef = &value
	return b
}

// WithTenantOverrides puts the entries into the TenantOverrides field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the TenantOverrides field,
// overwriting an existing map entries in TenantOverrides field with the same key.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithTenantOverrides(entries map[string]*v1.JSON) *AzureVirtualMachineSpecApplyConfiguration {
	if b.TenantOverrides == nil && len(entries) > 0 {
		b.TenantOverrides = make(map[string]*v1.JSON, len(entries))
	}
	for k, v := range entries {
		b.TenantOverrides[k] = v
	}
	return b
}

// WithExports adds the given value to the Exports field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Exports field.
func (b *AzureVirtualMachineSpecApplyConfiguration) WithExports(values ...*AzureVirtualMachineExportsSpecApplyConfiguration) *AzureVirtualMachineSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithExports")
		}
		b.Exports = append(b.Exports, *values[i])
	}
	return b
}
