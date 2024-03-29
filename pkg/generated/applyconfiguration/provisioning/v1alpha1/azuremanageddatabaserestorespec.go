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

// AzureManagedDatabaseRestoreSpecApplyConfiguration represents an declarative configuration of the AzureManagedDatabaseRestoreSpec type for use
// with apply.
type AzureManagedDatabaseRestoreSpecApplyConfiguration struct {
	BackupFileName   *string                                      `json:"backupFileName,omitempty"`
	StorageContainer *AzureStorageContainerSpecApplyConfiguration `json:"storageContainer,omitempty"`
}

// AzureManagedDatabaseRestoreSpecApplyConfiguration constructs an declarative configuration of the AzureManagedDatabaseRestoreSpec type for use with
// apply.
func AzureManagedDatabaseRestoreSpec() *AzureManagedDatabaseRestoreSpecApplyConfiguration {
	return &AzureManagedDatabaseRestoreSpecApplyConfiguration{}
}

// WithBackupFileName sets the BackupFileName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BackupFileName field is set to the value of the last call.
func (b *AzureManagedDatabaseRestoreSpecApplyConfiguration) WithBackupFileName(value string) *AzureManagedDatabaseRestoreSpecApplyConfiguration {
	b.BackupFileName = &value
	return b
}

// WithStorageContainer sets the StorageContainer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the StorageContainer field is set to the value of the last call.
func (b *AzureManagedDatabaseRestoreSpecApplyConfiguration) WithStorageContainer(value *AzureStorageContainerSpecApplyConfiguration) *AzureManagedDatabaseRestoreSpecApplyConfiguration {
	b.StorageContainer = value
	return b
}
