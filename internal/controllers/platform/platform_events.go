package platform

import (
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	SyncedSuccessfullyTopic         string = "PlatformControllers.PlatformController.SyncedSuccessfully"
	TenantCreatedSuccessfullyTopic  string = "PlatformControllers.PlatformController.TenantCreatedSuccessfully"
	TenantUpdatedSuccessfullyTopic  string = "PlatformControllers.PlatformController.TenantUpdatedSuccessfully"
	TenantDeletedSuccessfullyTopic  string = "PlatformControllers.PlatformController.TenantDeletedSuccessfully"
	DomainCreatedSuccessfullyTopic  string = "PlatformControllers.PlatformController.DomainCreatedSuccessfully"
	DomainUpdatedSuccessfullyTopic  string = "PlatformControllers.PlatformController.DomainUpdatedSuccessfully"
	DomainDeletedSuccessfullyTopic  string = "PlatformControllers.PlatformController.DomainDeletedSuccessfully"
	ServiceCreatedSuccessfullyTopic string = "PlatformControllers.PlatformController.ServiceCreatedSuccessfully"
	ServiceUpdatedSuccessfullyTopic string = "PlatformControllers.PlatformController.ServiceUpdatedSuccessfully"
	ServiceDeletedSuccessfullyTopic string = "PlatformControllers.PlatformController.ServiceDeletedSuccessfully"
)

type TenantCreated struct {
	TenantId   string
	TenantName string
}
type TenantUpdated struct {
	TenantId     string
	TenantName   string
	Description  string
	PlatformRef  string
	Enabled      bool
	DomainRefs   []string
	AdminEmail   string
	DeletePolicy platformv1.DeletePolicy
	Configs      map[string]string
}
type TenantDeleted struct {
	TenantId   string
	TenantName string
}

type DomainCreated struct {
	DomainName  string
	Namespace   string
	PlatformRef string
}

type DomainUpdated struct {
	oldValue Domain
	newValue Domain
}

type Domain struct {
	DomainName          string
	Namespace           string
	PlatformRef         string
	ExportActiveDomains bool
}

type DomainDeleted struct {
	DomainName string
	Namespace  string
}

type ServiceCreated struct {
	ServiceName string
	Namespace   string
	PlatformRef string
}

type ServiceUpdated struct {
	ServiceName        string
	Namespace          string
	PlatformRef        string
	RequiredDomainRefs []string
	OptionalDomainRefs []string
}

type ServiceDeleted struct {
	ServiceName string
	Namespace   string
}
