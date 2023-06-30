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

// AzureDatabaseSpecApplyConfiguration represents an declarative configuration of the AzureDatabaseSpec type for use
// with apply.
type AzureDatabaseSpecApplyConfiguration struct {
	DbName                             *string                                      `json:"dbName,omitempty"`
	SqlServer                          *SqlServerSpecApplyConfiguration             `json:"sqlServer,omitempty"`
	Sku                                *string                                      `json:"sku,omitempty"`
	SourceDatabaseId                   *string                                      `json:"sourceDatabaseId,omitempty"`
	Exports                            []AzureDatabaseExportsSpecApplyConfiguration `json:"exports,omitempty"`
	ProvisioningMetaApplyConfiguration `json:",inline"`
}

// AzureDatabaseSpecApplyConfiguration constructs an declarative configuration of the AzureDatabaseSpec type for use with
// apply.
func AzureDatabaseSpec() *AzureDatabaseSpecApplyConfiguration {
	return &AzureDatabaseSpecApplyConfiguration{}
}

// WithDbName sets the DbName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DbName field is set to the value of the last call.
func (b *AzureDatabaseSpecApplyConfiguration) WithDbName(value string) *AzureDatabaseSpecApplyConfiguration {
	b.DbName = &value
	return b
}

// WithSqlServer sets the SqlServer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SqlServer field is set to the value of the last call.
func (b *AzureDatabaseSpecApplyConfiguration) WithSqlServer(value *SqlServerSpecApplyConfiguration) *AzureDatabaseSpecApplyConfiguration {
	b.SqlServer = value
	return b
}

// WithSku sets the Sku field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Sku field is set to the value of the last call.
func (b *AzureDatabaseSpecApplyConfiguration) WithSku(value string) *AzureDatabaseSpecApplyConfiguration {
	b.Sku = &value
	return b
}

// WithSourceDatabaseId sets the SourceDatabaseId field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SourceDatabaseId field is set to the value of the last call.
func (b *AzureDatabaseSpecApplyConfiguration) WithSourceDatabaseId(value string) *AzureDatabaseSpecApplyConfiguration {
	b.SourceDatabaseId = &value
	return b
}

// WithExports adds the given value to the Exports field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Exports field.
func (b *AzureDatabaseSpecApplyConfiguration) WithExports(values ...*AzureDatabaseExportsSpecApplyConfiguration) *AzureDatabaseSpecApplyConfiguration {
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
func (b *AzureDatabaseSpecApplyConfiguration) WithPlatformRef(value string) *AzureDatabaseSpecApplyConfiguration {
	b.PlatformRef = &value
	return b
}

// WithDomainRef sets the DomainRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DomainRef field is set to the value of the last call.
func (b *AzureDatabaseSpecApplyConfiguration) WithDomainRef(value string) *AzureDatabaseSpecApplyConfiguration {
	b.DomainRef = &value
	return b
}

// WithTenantOverrides puts the entries into the TenantOverrides field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the TenantOverrides field,
// overwriting an existing map entries in TenantOverrides field with the same key.
func (b *AzureDatabaseSpecApplyConfiguration) WithTenantOverrides(entries map[string]*v1.JSON) *AzureDatabaseSpecApplyConfiguration {
	if b.TenantOverrides == nil && len(entries) > 0 {
		b.TenantOverrides = make(map[string]*v1.JSON, len(entries))
	}
	for k, v := range entries {
		b.TenantOverrides[k] = v
	}
	return b
}
