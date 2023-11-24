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

// AzureVirtualDesktopUsersSpecApplyConfiguration represents an declarative configuration of the AzureVirtualDesktopUsersSpec type for use
// with apply.
type AzureVirtualDesktopUsersSpecApplyConfiguration struct {
	Admins           []string `json:"admins,omitempty"`
	ApplicationUsers []string `json:"applicationUsers,omitempty"`
}

// AzureVirtualDesktopUsersSpecApplyConfiguration constructs an declarative configuration of the AzureVirtualDesktopUsersSpec type for use with
// apply.
func AzureVirtualDesktopUsersSpec() *AzureVirtualDesktopUsersSpecApplyConfiguration {
	return &AzureVirtualDesktopUsersSpecApplyConfiguration{}
}

// WithAdmins adds the given value to the Admins field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Admins field.
func (b *AzureVirtualDesktopUsersSpecApplyConfiguration) WithAdmins(values ...string) *AzureVirtualDesktopUsersSpecApplyConfiguration {
	for i := range values {
		b.Admins = append(b.Admins, values[i])
	}
	return b
}

// WithApplicationUsers adds the given value to the ApplicationUsers field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ApplicationUsers field.
func (b *AzureVirtualDesktopUsersSpecApplyConfiguration) WithApplicationUsers(values ...string) *AzureVirtualDesktopUsersSpecApplyConfiguration {
	for i := range values {
		b.ApplicationUsers = append(b.ApplicationUsers, values[i])
	}
	return b
}
