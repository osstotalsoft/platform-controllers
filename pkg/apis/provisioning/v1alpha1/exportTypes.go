package v1alpha1

type ValueExport struct {
	// +optional
	ToConfigMap ConfigMapTemplate `json:"toConfigMap,omitempty"`
	// +optional
	ToVault VaultSecretTemplate `json:"toVault,omitempty"`
}

type ConfigMapTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}

type VaultSecretTemplate struct {
	KeyTemplate string `json:"keyTemplate"`
}
