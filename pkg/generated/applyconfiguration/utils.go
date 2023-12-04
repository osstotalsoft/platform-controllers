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

package applyconfiguration

import (
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	configurationv1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/applyconfiguration/configuration/v1alpha1"
	applyconfigurationplatformv1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/applyconfiguration/platform/v1alpha1"
	applyconfigurationprovisioningv1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/applyconfiguration/provisioning/v1alpha1"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=configuration.totalsoft.ro, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomain"):
		return &configurationv1alpha1.ConfigurationDomainApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomainSpec"):
		return &configurationv1alpha1.ConfigurationDomainSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomainStatus"):
		return &configurationv1alpha1.ConfigurationDomainStatusApplyConfiguration{}

		// Group=platform.totalsoft.ro, Version=v1alpha1
	case platformv1alpha1.SchemeGroupVersion.WithKind("Domain"):
		return &applyconfigurationplatformv1alpha1.DomainApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("DomainSpec"):
		return &applyconfigurationplatformv1alpha1.DomainSpecApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("Platform"):
		return &applyconfigurationplatformv1alpha1.PlatformApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("PlatformSpec"):
		return &applyconfigurationplatformv1alpha1.PlatformSpecApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("PlatformStatus"):
		return &applyconfigurationplatformv1alpha1.PlatformStatusApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("Service"):
		return &applyconfigurationplatformv1alpha1.ServiceApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("ServiceSpec"):
		return &applyconfigurationplatformv1alpha1.ServiceSpecApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("Tenant"):
		return &applyconfigurationplatformv1alpha1.TenantApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("TenantSpec"):
		return &applyconfigurationplatformv1alpha1.TenantSpecApplyConfiguration{}
	case platformv1alpha1.SchemeGroupVersion.WithKind("TenantStatus"):
		return &applyconfigurationplatformv1alpha1.TenantStatusApplyConfiguration{}

		// Group=provisioning.totalsoft.ro, Version=v1alpha1
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureDatabase"):
		return &applyconfigurationprovisioningv1alpha1.AzureDatabaseApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureDatabaseExportsSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureDatabaseExportsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureDatabaseSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureDatabaseSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureManagedDatabase"):
		return &applyconfigurationprovisioningv1alpha1.AzureManagedDatabaseApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureManagedDatabaseExportsSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureManagedDatabaseExportsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureManagedDatabaseRestoreSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureManagedDatabaseRestoreSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureManagedDatabaseSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureManagedDatabaseSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureManagedInstanceSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureManagedInstanceSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureStorageContainerSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureStorageContainerSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktop"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktopApplication"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopApplicationApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktopExportsSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopExportsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktopGroupsSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopGroupsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktopSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualDesktopUsersSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualDesktopUsersSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualMachine"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualMachineApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualMachineExportsSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualMachineExportsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("AzureVirtualMachineSpec"):
		return &applyconfigurationprovisioningv1alpha1.AzureVirtualMachineSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("ConfigMapTemplate"):
		return &applyconfigurationprovisioningv1alpha1.ConfigMapTemplateApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("DependsOn"):
		return &applyconfigurationprovisioningv1alpha1.DependsOnApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("HelmRelease"):
		return &applyconfigurationprovisioningv1alpha1.HelmReleaseApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("HelmReleaseExportsSpec"):
		return &applyconfigurationprovisioningv1alpha1.HelmReleaseExportsSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("HelmReleaseSpec"):
		return &applyconfigurationprovisioningv1alpha1.HelmReleaseSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("InitScriptArgs"):
		return &applyconfigurationprovisioningv1alpha1.InitScriptArgsApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("ProvisioningMeta"):
		return &applyconfigurationprovisioningv1alpha1.ProvisioningMetaApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("ProvisioningTarget"):
		return &applyconfigurationprovisioningv1alpha1.ProvisioningTargetApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("ProvisioningTargetFilter"):
		return &applyconfigurationprovisioningv1alpha1.ProvisioningTargetFilterApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("SqlServerSpec"):
		return &applyconfigurationprovisioningv1alpha1.SqlServerSpecApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("ValueExport"):
		return &applyconfigurationprovisioningv1alpha1.ValueExportApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("VaultSecretTemplate"):
		return &applyconfigurationprovisioningv1alpha1.VaultSecretTemplateApplyConfiguration{}
	case provisioningv1alpha1.SchemeGroupVersion.WithKind("VirtualMachineGalleryApplication"):
		return &applyconfigurationprovisioningv1alpha1.VirtualMachineGalleryApplicationApplyConfiguration{}

	}
	return nil
}
