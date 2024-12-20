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

// MsSqlDatabaseSpecApplyConfiguration represents an declarative configuration of the MsSqlDatabaseSpec type for use
// with apply.
type MsSqlDatabaseSpecApplyConfiguration struct {
	DbName                             *string                                      `json:"dbName,omitempty"`
	SqlServer                          *MsSqlServerSpecApplyConfiguration           `json:"sqlServer,omitempty"`
	RestoreFrom                        *MsSqlDatabaseRestoreSpecApplyConfiguration  `json:"restoreFrom,omitempty"`
	ImportDatabaseId                   *string                                      `json:"importDatabaseId,omitempty"`
	Exports                            []MsSqlDatabaseExportsSpecApplyConfiguration `json:"exports,omitempty"`
	ProvisioningMetaApplyConfiguration `json:",inline"`
}

// MsSqlDatabaseSpecApplyConfiguration constructs an declarative configuration of the MsSqlDatabaseSpec type for use with
// apply.
func MsSqlDatabaseSpec() *MsSqlDatabaseSpecApplyConfiguration {
	return &MsSqlDatabaseSpecApplyConfiguration{}
}

// WithDbName sets the DbName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DbName field is set to the value of the last call.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithDbName(value string) *MsSqlDatabaseSpecApplyConfiguration {
	b.DbName = &value
	return b
}

// WithSqlServer sets the SqlServer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SqlServer field is set to the value of the last call.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithSqlServer(value *MsSqlServerSpecApplyConfiguration) *MsSqlDatabaseSpecApplyConfiguration {
	b.SqlServer = value
	return b
}

// WithRestoreFrom sets the RestoreFrom field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RestoreFrom field is set to the value of the last call.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithRestoreFrom(value *MsSqlDatabaseRestoreSpecApplyConfiguration) *MsSqlDatabaseSpecApplyConfiguration {
	b.RestoreFrom = value
	return b
}

// WithImportDatabaseId sets the ImportDatabaseId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ImportDatabaseId field is set to the value of the last call.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithImportDatabaseId(value string) *MsSqlDatabaseSpecApplyConfiguration {
	b.ImportDatabaseId = &value
	return b
}

// WithExports adds the given value to the Exports field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Exports field.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithExports(values ...*MsSqlDatabaseExportsSpecApplyConfiguration) *MsSqlDatabaseSpecApplyConfiguration {
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
func (b *MsSqlDatabaseSpecApplyConfiguration) WithPlatformRef(value string) *MsSqlDatabaseSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithDomainRef sets the DomainRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DomainRef field is set to the value of the last call.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithDomainRef(value string) *MsSqlDatabaseSpecApplyConfiguration {
	b.DomainRef = &value
	return b
}

// WithTenantOverrides puts the entries into the TenantOverrides field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the TenantOverrides field,
// overwriting an existing map entries in TenantOverrides field with the same key.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithTenantOverrides(entries map[string]*v1.JSON) *MsSqlDatabaseSpecApplyConfiguration {
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
func (b *MsSqlDatabaseSpecApplyConfiguration) WithTarget(value *ProvisioningTargetApplyConfiguration) *MsSqlDatabaseSpecApplyConfiguration {
	b.Target = value
	return b
}

// WithDependsOn adds the given value to the DependsOn field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the DependsOn field.
func (b *MsSqlDatabaseSpecApplyConfiguration) WithDependsOn(values ...*ProvisioningResourceIdendtifierApplyConfiguration) *MsSqlDatabaseSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithDependsOn")
		}
		b.DependsOn = append(b.DependsOn, *values[i])
	}
	return b
}
