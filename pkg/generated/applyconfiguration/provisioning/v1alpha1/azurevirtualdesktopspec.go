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

// AzureVirtualDesktopSpecApplyConfiguration represents an declarative configuration of the AzureVirtualDesktopSpec type for use
// with apply.
type AzureVirtualDesktopSpecApplyConfiguration struct {
	PlatformRef           *string                                            `json:"platformRef,omitempty"`
	HostPoolName          *string                                            `json:"hostPoolName,omitempty"`
	VmNamePrefix          *string                                            `json:"vmNamePrefix,omitempty"`
	VmSize                *string                                            `json:"vmSize,omitempty"`
	VmNumberOfInstances   *int                                               `json:"vmNumberOfInstances,omitempty"`
	OSDiskType            *string                                            `json:"osDiskType,omitempty"`
	SourceImageId         *string                                            `json:"sourceImageId,omitempty"`
	SubnetId              *string                                            `json:"subnetId,omitempty"`
	EnableTrustedLaunch   *bool                                              `json:"enableTrustedLaunch,omitempty"`
	InitScript            *string                                            `json:"initScript,omitempty"`
	InitScriptArguments   []InitScriptArgsApplyConfiguration                 `json:"initScriptArgs,omitempty"`
	WorkspaceFriendlyName *string                                            `json:"workspaceFriendlyName,omitempty"`
	Applications          []AzureVirtualDesktopApplicationApplyConfiguration `json:"applications,omitempty"`
	Users                 *AzureVirtualDesktopUsersSpecApplyConfiguration    `json:"users,omitempty"`
	Exports               []AzureVirtualDesktopExportsSpecApplyConfiguration `json:"exports,omitempty"`
}

// AzureVirtualDesktopSpecApplyConfiguration constructs an declarative configuration of the AzureVirtualDesktopSpec type for use with
// apply.
func AzureVirtualDesktopSpec() *AzureVirtualDesktopSpecApplyConfiguration {
	return &AzureVirtualDesktopSpecApplyConfiguration{}
}

// WithPlatformRef sets the PlatformRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PlatformRef field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithPlatformRef(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithHostPoolName sets the HostPoolName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the HostPoolName field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithHostPoolName(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.HostPoolName = &value
	return b
}

// WithVmNamePrefix sets the VmNamePrefix field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmNamePrefix field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithVmNamePrefix(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.VmNamePrefix = &value
	return b
}

// WithVmSize sets the VmSize field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmSize field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithVmSize(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.VmSize = &value
	return b
}

// WithVmNumberOfInstances sets the VmNumberOfInstances field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the VmNumberOfInstances field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithVmNumberOfInstances(value int) *AzureVirtualDesktopSpecApplyConfiguration {
	b.VmNumberOfInstances = &value
	return b
}

// WithOSDiskType sets the OSDiskType field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the OSDiskType field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithOSDiskType(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.OSDiskType = &value
	return b
}

// WithSourceImageId sets the SourceImageId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SourceImageId field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithSourceImageId(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.SourceImageId = &value
	return b
}

// WithSubnetId sets the SubnetId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SubnetId field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithSubnetId(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.SubnetId = &value
	return b
}

// WithEnableTrustedLaunch sets the EnableTrustedLaunch field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the EnableTrustedLaunch field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithEnableTrustedLaunch(value bool) *AzureVirtualDesktopSpecApplyConfiguration {
	b.EnableTrustedLaunch = &value
	return b
}

// WithInitScript sets the InitScript field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the InitScript field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithInitScript(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.InitScript = &value
	return b
}

// WithInitScriptArguments adds the given value to the InitScriptArguments field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the InitScriptArguments field.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithInitScriptArguments(values ...*InitScriptArgsApplyConfiguration) *AzureVirtualDesktopSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithInitScriptArguments")
		}
		b.InitScriptArguments = append(b.InitScriptArguments, *values[i])
	}
	return b
}

// WithWorkspaceFriendlyName sets the WorkspaceFriendlyName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the WorkspaceFriendlyName field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithWorkspaceFriendlyName(value string) *AzureVirtualDesktopSpecApplyConfiguration {
	b.WorkspaceFriendlyName = &value
	return b
}

// WithApplications adds the given value to the Applications field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Applications field.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithApplications(values ...*AzureVirtualDesktopApplicationApplyConfiguration) *AzureVirtualDesktopSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithApplications")
		}
		b.Applications = append(b.Applications, *values[i])
	}
	return b
}

// WithUsers sets the Users field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Users field is set to the value of the last call.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithUsers(value *AzureVirtualDesktopUsersSpecApplyConfiguration) *AzureVirtualDesktopSpecApplyConfiguration {
	b.Users = value
	return b
}

// WithExports adds the given value to the Exports field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Exports field.
func (b *AzureVirtualDesktopSpecApplyConfiguration) WithExports(values ...*AzureVirtualDesktopExportsSpecApplyConfiguration) *AzureVirtualDesktopSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithExports")
		}
		b.Exports = append(b.Exports, *values[i])
	}
	return b
}
