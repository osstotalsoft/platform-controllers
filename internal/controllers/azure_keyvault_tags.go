package controllers

// The tags of an Azure Key Vault secret that a value is exported to, which say where the value comes from, since the
// vault is flat. The provisioner writes them, and the configuration-domain controller finds a domain's secrets by them.
const (
	AzureKeyVaultTagPlatform  = "platform"
	AzureKeyVaultTagNamespace = "namespace"
	AzureKeyVaultTagDomain    = "domain"
	// AzureKeyVaultTagTenant is only set on the secrets of a tenant's values.
	AzureKeyVaultTagTenant = "tenant"
	AzureKeyVaultTagObject = "object"
	// AzureKeyVaultTagConfigKey is the configuration key the value is exported as, which the secret is mounted as.
	AzureKeyVaultTagConfigKey = "configKey"
)
