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

// AzurePowerShellScriptExportsSpecApplyConfiguration represents an declarative configuration of the AzurePowerShellScriptExportsSpec type for use
// with apply.
type AzurePowerShellScriptExportsSpecApplyConfiguration struct {
	Domain        *string                        `json:"domain,omitempty"`
	ScriptOutputs *ValueExportApplyConfiguration `json:"scriptOutputs,omitempty"`
}

// AzurePowerShellScriptExportsSpecApplyConfiguration constructs an declarative configuration of the AzurePowerShellScriptExportsSpec type for use with
// apply.
func AzurePowerShellScriptExportsSpec() *AzurePowerShellScriptExportsSpecApplyConfiguration {
	return &AzurePowerShellScriptExportsSpecApplyConfiguration{}
}

// WithDomain sets the Domain field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Domain field is set to the value of the last call.
func (b *AzurePowerShellScriptExportsSpecApplyConfiguration) WithDomain(value string) *AzurePowerShellScriptExportsSpecApplyConfiguration {
	b.Domain = &value
	return b
}

// WithScriptOutputs sets the ScriptOutputs field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ScriptOutputs field is set to the value of the last call.
func (b *AzurePowerShellScriptExportsSpecApplyConfiguration) WithScriptOutputs(value *ValueExportApplyConfiguration) *AzurePowerShellScriptExportsSpecApplyConfiguration {
	b.ScriptOutputs = value
	return b
}
