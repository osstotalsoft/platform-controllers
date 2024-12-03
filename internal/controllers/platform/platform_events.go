package platform

import (
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	SyncedSuccessfullyTopic        string = "PlatformControllers.PlatformController.SyncedSuccessfully"
	TenantCreatedSuccessfullyTopic string = "PlatformControllers.PlatformController.TenantCreatedSuccessfully"
	TenantUpdatedSuccessfullyTopic string = "PlatformControllers.PlatformController.TenantUpdatedSuccessfully"
	TenantDeletedSuccessfullyTopic string = "PlatformControllers.PlatformController.TenantDeletedSuccessfully"
)

type TenantCreated struct {
	TenantId   string
	TenantName string
}
type TenantUpdated struct {
	TenantId     string
	TenantName   string
	PlatformRef  string
	Enabled      bool
	DomainRefs   []string
	AdminEmail   string
	DeletePolicy platformv1.DeletePolicy
	Configs      map[string]string
}
type TenantDeleted struct {
	TenantId string
}
