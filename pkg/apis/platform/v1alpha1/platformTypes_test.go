package v1alpha1

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestPlatformSpecSecretStore(t *testing.T) {
	kv := &AzureKeyVaultSpec{Name: "kv-lsng-qa", ResourceGroupName: "rg-lsng-qa", WorkloadIdentityClientId: "00000000-0000-0000-0000-000000000001"}
	tests := []struct {
		name     string
		spec     PlatformSpec
		wantType SecretStoreType
		wantKV   *AzureKeyVaultSpec
	}{
		{"absent", PlatformSpec{}, SecretStoreTypeHashicorpVault, nil},
		{"no type", PlatformSpec{SecretStore: &SecretStoreSpec{}}, SecretStoreTypeHashicorpVault, nil},
		{"vault", PlatformSpec{SecretStore: &SecretStoreSpec{Type: SecretStoreTypeHashicorpVault}}, SecretStoreTypeHashicorpVault, nil},
		{"key vault", PlatformSpec{SecretStore: &SecretStoreSpec{Type: SecretStoreTypeAzureKeyVault, AzureKeyVault: kv}}, SecretStoreTypeAzureKeyVault, kv},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.wantType, tt.spec.SecretStoreType(), tt.name)
		assert.Equal(t, tt.wantKV, tt.spec.AzureKeyVault(), tt.name)
	}
}

// TestPlatformCRD checks the generated CRD carries the secret store's enum, default and rules, so a removed
// kubebuilder marker or a stale helm/crds file fails here.
func TestPlatformCRD(t *testing.T) {
	data, err := os.ReadFile("../../../../helm/crds/platform.totalsoft.ro_platforms.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]

	assert.NotContains(t, spec.Properties, "azureKeyVault", "spec.azureKeyVault is replaced by spec.secretStore")

	secretStore, ok := spec.Properties["secretStore"]
	if !ok {
		t.Fatal("expected spec.secretStore in the CRD schema")
	}

	storeType := secretStore.Properties["type"]
	var enum []string
	for _, e := range storeType.Enum {
		enum = append(enum, string(e.Raw))
	}
	assert.ElementsMatch(t, []string{`"HashicorpVault"`, `"AzureKeyVault"`}, enum)
	if assert.NotNil(t, storeType.Default, "secretStore.type default") {
		assert.Equal(t, `"HashicorpVault"`, string(storeType.Default.Raw))
	}

	var rules []string
	for _, v := range secretStore.XValidations {
		rules = append(rules, v.Rule)
	}
	assert.Contains(t, rules, "(has(self.type) && self.type == 'AzureKeyVault') == has(self.azureKeyVault)")

	keyVault := secretStore.Properties["azureKeyVault"]
	assert.Subset(t, keyVault.Required, []string{"name", "resourceGroupName", "workloadIdentityClientId"})
	assert.Equal(t, "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$",
		keyVault.Properties["workloadIdentityClientId"].Pattern)
}
