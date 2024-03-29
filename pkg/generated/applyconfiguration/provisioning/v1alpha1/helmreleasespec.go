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
	v2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// HelmReleaseSpecApplyConfiguration represents an declarative configuration of the HelmReleaseSpec type for use
// with apply.
type HelmReleaseSpecApplyConfiguration struct {
	Release                            *v2beta1.HelmReleaseSpec                   `json:"release,omitempty"`
	Exports                            []HelmReleaseExportsSpecApplyConfiguration `json:"exports,omitempty"`
	ProvisioningMetaApplyConfiguration `json:",inline"`
}

// HelmReleaseSpecApplyConfiguration constructs an declarative configuration of the HelmReleaseSpec type for use with
// apply.
func HelmReleaseSpec() *HelmReleaseSpecApplyConfiguration {
	return &HelmReleaseSpecApplyConfiguration{}
}

// WithRelease sets the Release field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Release field is set to the value of the last call.
func (b *HelmReleaseSpecApplyConfiguration) WithRelease(value v2beta1.HelmReleaseSpec) *HelmReleaseSpecApplyConfiguration {
	b.Release = &value
	return b
}

// WithExports adds the given value to the Exports field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Exports field.
func (b *HelmReleaseSpecApplyConfiguration) WithExports(values ...*HelmReleaseExportsSpecApplyConfiguration) *HelmReleaseSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithExports")
		}
		b.Exports = append(b.Exports, *values[i])
	}
	return b
}

// WithPlatformRef sets the PlatformRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PlatformRef field is set to the value of the last call.
func (b *HelmReleaseSpecApplyConfiguration) WithPlatformRef(value string) *HelmReleaseSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithDomainRef sets the DomainRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DomainRef field is set to the value of the last call.
func (b *HelmReleaseSpecApplyConfiguration) WithDomainRef(value string) *HelmReleaseSpecApplyConfiguration {
	b.DomainRef = &value
	return b
}

// WithTenantOverrides puts the entries into the TenantOverrides field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the TenantOverrides field,
// overwriting an existing map entries in TenantOverrides field with the same key.
func (b *HelmReleaseSpecApplyConfiguration) WithTenantOverrides(entries map[string]*v1.JSON) *HelmReleaseSpecApplyConfiguration {
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
func (b *HelmReleaseSpecApplyConfiguration) WithTarget(value *ProvisioningTargetApplyConfiguration) *HelmReleaseSpecApplyConfiguration {
	b.Target = value
	return b
}

// WithDependsOn adds the given value to the DependsOn field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the DependsOn field.
func (b *HelmReleaseSpecApplyConfiguration) WithDependsOn(values ...*ProvisioningResourceIdendtifierApplyConfiguration) *HelmReleaseSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithDependsOn")
		}
		b.DependsOn = append(b.DependsOn, *values[i])
	}
	return b
}
