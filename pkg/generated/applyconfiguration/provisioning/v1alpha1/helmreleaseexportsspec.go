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

// HelmReleaseExportsSpecApplyConfiguration represents an declarative configuration of the HelmReleaseExportsSpec type for use
// with apply.
type HelmReleaseExportsSpecApplyConfiguration struct {
	Domain      *string                        `json:"domain,omitempty"`
	ReleaseName *ValueExportApplyConfiguration `json:"releaseName,omitempty"`
}

// HelmReleaseExportsSpecApplyConfiguration constructs an declarative configuration of the HelmReleaseExportsSpec type for use with
// apply.
func HelmReleaseExportsSpec() *HelmReleaseExportsSpecApplyConfiguration {
	return &HelmReleaseExportsSpecApplyConfiguration{}
}

// WithDomain sets the Domain field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Domain field is set to the value of the last call.
func (b *HelmReleaseExportsSpecApplyConfiguration) WithDomain(value string) *HelmReleaseExportsSpecApplyConfiguration {
	b.Domain = &value
	return b
}

// WithReleaseName sets the ReleaseName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ReleaseName field is set to the value of the last call.
func (b *HelmReleaseExportsSpecApplyConfiguration) WithReleaseName(value *ValueExportApplyConfiguration) *HelmReleaseExportsSpecApplyConfiguration {
	b.ReleaseName = value
	return b
}