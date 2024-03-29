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

// ProvisioningMetaApplyConfiguration represents an declarative configuration of the ProvisioningMeta type for use
// with apply.
type ProvisioningMetaApplyConfiguration struct {
	PlatformRef     *string                                             `json:"platformRef,omitempty"`
	DomainRef       *string                                             `json:"domainRef,omitempty"`
	TenantOverrides map[string]*v1.JSON                                 `json:"tenantOverrides,omitempty"`
	Target          *ProvisioningTargetApplyConfiguration               `json:"target,omitempty"`
	DependsOn       []ProvisioningResourceIdendtifierApplyConfiguration `json:"dependsOn,omitempty"`
}

// ProvisioningMetaApplyConfiguration constructs an declarative configuration of the ProvisioningMeta type for use with
// apply.
func ProvisioningMeta() *ProvisioningMetaApplyConfiguration {
	return &ProvisioningMetaApplyConfiguration{}
}

// WithPlatformRef sets the PlatformRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PlatformRef field is set to the value of the last call.
func (b *ProvisioningMetaApplyConfiguration) WithPlatformRef(value string) *ProvisioningMetaApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithDomainRef sets the DomainRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DomainRef field is set to the value of the last call.
func (b *ProvisioningMetaApplyConfiguration) WithDomainRef(value string) *ProvisioningMetaApplyConfiguration {
	b.DomainRef = &value
	return b
}

// WithTenantOverrides puts the entries into the TenantOverrides field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the TenantOverrides field,
// overwriting an existing map entries in TenantOverrides field with the same key.
func (b *ProvisioningMetaApplyConfiguration) WithTenantOverrides(entries map[string]*v1.JSON) *ProvisioningMetaApplyConfiguration {
	if b.TenantOverrides == nil && len(entries) > 0 {
		b.TenantOverrides = make(map[string]*v1.JSON, len(entries))
	}
	for k, v := range entries {
		b.TenantOverrides[k] = v
	}
	return b
}

// WithTarget sets the Target field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Target field is set to the value of the last call.
func (b *ProvisioningMetaApplyConfiguration) WithTarget(value *ProvisioningTargetApplyConfiguration) *ProvisioningMetaApplyConfiguration {
	b.Target = value
	return b
}

// WithDependsOn adds the given value to the DependsOn field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the DependsOn field.
func (b *ProvisioningMetaApplyConfiguration) WithDependsOn(values ...*ProvisioningResourceIdendtifierApplyConfiguration) *ProvisioningMetaApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithDependsOn")
		}
		b.DependsOn = append(b.DependsOn, *values[i])
	}
	return b
}
