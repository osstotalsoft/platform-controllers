package controllers

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PlatformNamespaceFilter(namespace, platform string) bool {
	return strings.HasPrefix(namespace, platform)
}

func GetPlatformNsAndDomain(obj metav1.Object) (platform, namespace, domain string, ok bool) {

	domain, domainLabelExists := obj.GetLabels()[DomainLabelName]
	if !domainLabelExists || len(domain) == 0 {
		return "", obj.GetNamespace(), domain, false
	}

	platform, platformLabelExists := obj.GetLabels()[PlatformLabelName]
	if !platformLabelExists || len(platform) == 0 {
		return platform, obj.GetNamespace(), domain, false
	}

	domain, namespace, found := strings.Cut(domain, ".")
	if !found {
		return platform, obj.GetNamespace(), domain, false
	}

	return platform, namespace, domain, true
}
