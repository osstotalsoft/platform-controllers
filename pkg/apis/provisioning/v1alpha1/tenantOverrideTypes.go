package v1alpha1

import apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

type HasTenantOverrides1 interface {
	GetTenantOverrides() map[string]*apiextensionsv1.JSON
}
