package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"totalsoft.ro/platform-controllers/internal/controllers"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

// keyVaultTestSecret is an enabled secret tagged as the provisioner tags the secrets it exports to.
func keyVaultTestSecret(name, platform, namespace, domain, configKey string) keyVaultSecret {
	return keyVaultSecret{
		Name: name,
		Tags: map[string]string{
			controllers.AzureKeyVaultTagPlatform:  platform,
			controllers.AzureKeyVaultTagNamespace: namespace,
			controllers.AzureKeyVaultTagDomain:    domain,
			controllers.AzureKeyVaultTagObject:    "some-object",
			controllers.AzureKeyVaultTagConfigKey: configKey,
		},
		Enabled: true,
	}
}

func TestSelectKeyVaultSecrets(t *testing.T) {
	const platform = "charismaonline.qa"
	qaPlatform := newPlatform(platform, "qa")
	origination := newConfigurationDomain("origination", "qa-lsng", platform, false, true)
	secret := keyVaultTestSecret

	t.Run("three groups, later wins", func(t *testing.T) {
		selected, conflicts := selectKeyVaultSecrets(qaPlatform, origination, []keyVaultSecret{
			secret("s3", platform, "qa-lsng", "origination", "A"),
			secret("s1", platform, "qa", "global", "A"),
			secret("s4", platform, "qa", "global", "B"),
			secret("s2", platform, "qa-lsng", "global", "A"),
		})
		assert.Equal(t, map[string]string{"A": "s3", "B": "s4"}, selected)
		assert.Empty(t, conflicts)
	})

	t.Run("other platforms and namespaces are ignored", func(t *testing.T) {
		selected, _ := selectKeyVaultSecrets(qaPlatform, origination, []keyVaultSecret{
			secret("prod", "charismaonline.prod", "qa-lsng", "origination", "A"),
			secret("other-ns", platform, "qa-other", "origination", "B"),
			secret("other-domain", platform, "qa-lsng", "payments", "C"),
			secret("platform-ns-domain", platform, "qa", "origination", "D"),
		})
		assert.Empty(t, selected)
	})

	t.Run("skips secrets without configKey and disabled ones", func(t *testing.T) {
		noKey := secret("no-key", platform, "qa-lsng", "origination", "A")
		delete(noKey.Tags, controllers.AzureKeyVaultTagConfigKey)
		disabled := secret("disabled", platform, "qa-lsng", "origination", "B")
		disabled.Enabled = false
		untagged := keyVaultSecret{Name: "untagged", Enabled: true}

		selected, _ := selectKeyVaultSecrets(qaPlatform, origination, []keyVaultSecret{noKey, disabled, untagged})
		assert.Empty(t, selected)
	})

	t.Run("tenant secrets are included", func(t *testing.T) {
		tenantSecret := secret("tenant-secret", platform, "qa-lsng", "origination", "MultiTenancy__Tenants__acme__Password")
		tenantSecret.Tags[controllers.AzureKeyVaultTagTenant] = "acme"

		selected, _ := selectKeyVaultSecrets(qaPlatform, origination, []keyVaultSecret{tenantSecret})
		assert.Equal(t, map[string]string{"MultiTenancy__Tenants__acme__Password": "tenant-secret"}, selected)
	})

	t.Run("same key twice in one group: alphabetically first wins", func(t *testing.T) {
		for _, secrets := range [][]keyVaultSecret{
			{secret("b-secret", platform, "qa-lsng", "origination", "K"), secret("a-secret", platform, "qa-lsng", "origination", "K")},
			{secret("a-secret", platform, "qa-lsng", "origination", "K"), secret("b-secret", platform, "qa-lsng", "origination", "K")},
		} {
			selected, conflicts := selectKeyVaultSecrets(qaPlatform, origination, secrets)
			assert.Equal(t, map[string]string{"K": "a-secret"}, selected)
			assert.Equal(t, []keyVaultConflict{{ConfigKey: "K", Winner: "a-secret", Loser: "b-secret"}}, conflicts)
		}
	})

	t.Run("domain in the target namespace: no conflict with itself", func(t *testing.T) {
		inTargetNamespace := newConfigurationDomain("origination", "qa", platform, false, true)
		selected, conflicts := selectKeyVaultSecrets(qaPlatform, inTargetNamespace, []keyVaultSecret{
			secret("global-secret", platform, "qa", "global", "G"),
		})
		assert.Equal(t, map[string]string{"G": "global-secret"}, selected)
		assert.Empty(t, conflicts)
	})
}

func TestAzureKeyVaultSpc(t *testing.T) {
	configDomain := newConfigurationDomain("origination", "qa-lsng", "charismaonline.qa", false, true)
	keyVault := &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-qa", ResourceGroupName: "rg-lsng-qa",
		WorkloadIdentityClientId: "00000000-0000-0000-0000-000000000001"}

	spc := azureKeyVaultSpc(configDomain, keyVault, "t-1", map[string]string{"B": "s4", "A": "s3"})

	assert.Equal(t, newSpcMeta(configDomain), spc.ObjectMeta)
	assert.Equal(t, csiv1.Provider("azure"), spc.Spec.Provider)
	want := "array:\n" +
		"  - |\n    objectName: \"s3\"\n    objectType: secret\n    objectAlias: \"A\"\n" +
		"  - |\n    objectName: \"s4\"\n    objectType: secret\n    objectAlias: \"B\"\n"
	assert.Equal(t, map[string]string{
		"keyvaultName": "kv-lsng-qa",
		"tenantId":     "t-1",
		"clientID":     "00000000-0000-0000-0000-000000000001",
		"objects":      want,
	}, spc.Spec.Parameters)

	t.Run("no secrets", func(t *testing.T) {
		spc := azureKeyVaultSpc(configDomain, keyVault, "t-1", map[string]string{})
		assert.Equal(t, "array: []\n", spc.Spec.Parameters["objects"])
	})
}

func TestSecretNameFromID(t *testing.T) {
	assert.Equal(t, "a-b-1", secretNameFromID("https://kv.vault.azure.net/secrets/a-b-1/0123abcd"))
	assert.Equal(t, "a-b-1", secretNameFromID("https://kv.vault.azure.net/secrets/a-b-1"))
}
