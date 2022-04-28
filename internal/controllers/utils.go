package controllers

import "strings"

func PlatformNamespaceFilter(namespace, platform string) bool {
	return strings.HasPrefix(namespace, platform)
}
