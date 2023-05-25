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

// TenantSpecApplyConfiguration represents an declarative configuration of the TenantSpec type for use
// with apply.
type TenantSpecApplyConfiguration struct {
	Id          *string  `json:"id,omitempty"`
	Description *string  `json:"description,omitempty"`
	PlatformRef *string  `json:"platformRef,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	ServiceRefs []string `json:"serviceRefs,omitempty"`
}

// TenantSpecApplyConfiguration constructs an declarative configuration of the TenantSpec type for use with
// apply.
func TenantSpec() *TenantSpecApplyConfiguration {
	return &TenantSpecApplyConfiguration{}
}

// WithId sets the Id field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Id field is set to the value of the last call.
func (b *TenantSpecApplyConfiguration) WithId(value string) *TenantSpecApplyConfiguration {
	b.Id = &value
	return b
}

// WithDescription sets the Description field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Description field is set to the value of the last call.
func (b *TenantSpecApplyConfiguration) WithDescription(value string) *TenantSpecApplyConfiguration {
	b.Description = &value
	return b
}

// WithPlatformRef sets the PlatformRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PlatformRef field is set to the value of the last call.
func (b *TenantSpecApplyConfiguration) WithPlatformRef(value string) *TenantSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithEnabled sets the Enabled field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Enabled field is set to the value of the last call.
func (b *TenantSpecApplyConfiguration) WithEnabled(value bool) *TenantSpecApplyConfiguration {
	b.Enabled = &value
	return b
}

// WithServiceRefs adds the given value to the ServiceRefs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ServiceRefs field.
func (b *TenantSpecApplyConfiguration) WithServiceRefs(values ...string) *TenantSpecApplyConfiguration {
	for i := range values {
		b.ServiceRefs = append(b.ServiceRefs, values[i])
	}
	return b
}
