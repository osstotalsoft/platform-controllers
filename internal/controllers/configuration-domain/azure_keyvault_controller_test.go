package configuration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func newAzureKeyVaultPlatform(name, targetNamespace string) *platformv1.Platform {
	platform := newPlatform(name, targetNamespace)
	platform.Spec.SecretStore = &platformv1.SecretStoreSpec{
		Type: platformv1.SecretStoreTypeAzureKeyVault,
		AzureKeyVault: &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-qa", ResourceGroupName: "rg-lsng-qa",
			WorkloadIdentityClientId: "00000000-0000-0000-0000-000000000001"},
	}
	return platform
}

// withKeyVaultSecrets makes the listing of a vault return secrets, or err.
func withKeyVaultSecrets(t *testing.T, err error, secrets ...keyVaultSecret) {
	old := listKeyVaultSecrets
	t.Cleanup(func() { listKeyVaultSecrets = old })
	listKeyVaultSecrets = func(_ context.Context, vaultName string) ([]keyVaultSecret, error) {
		assert.Equal(t, "kv-lsng-qa", vaultName)
		return secrets, err
	}
}

func hasEventContaining(events []string, substr string) bool {
	for _, e := range events {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

func TestConfigurationDomainController_azureKeyVault(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)
	oldRequeueInterval := requeueInterval
	requeueInterval = 2 * time.Minute
	t.Cleanup(func() { requeueInterval = oldRequeueInterval })

	const platform, namespace, domain = "qa", "qa-lsng", "origination"
	run := func(spcs ...runtime.Object) *ConfigurationDomainController {
		return runController(
			[]runtime.Object{newAzureKeyVaultPlatform(platform, "qa")},
			[]runtime.Object{newConfigurationDomain(domain, namespace, platform, false, true)},
			nil, nil, spcs)
	}
	getSpc := func(c *ConfigurationDomainController) (*csiv1.SecretProviderClass, error) {
		return c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), getOutputSpcName(domain), metav1.GetOptions{})
	}

	t.Run("aggregate Key Vault secrets", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "true")
		t.Setenv("AZURE_TENANT_ID", "t-1")
		withKeyVaultSecrets(t, nil,
			keyVaultTestSecret("s1", platform, namespace, domain, "A"),
			keyVaultTestSecret("other", "prod", namespace, domain, "B"))
		c := run()

		assert.True(t, c.processNextWorkItem())

		spc, err := getSpc(c)
		if assert.NoError(t, err) {
			assert.Equal(t, csiv1.Provider("azure"), spc.Spec.Provider)
			assert.Equal(t, "t-1", spc.Spec.Parameters["tenantId"])
			assert.Equal(t, "00000000-0000-0000-0000-000000000001", spc.Spec.Parameters["clientID"])
			assert.Equal(t, "array:\n  - |\n    objectName: \"s1\"\n    objectType: secret\n    objectAlias: \"A\"\n", spc.Spec.Parameters["objects"])
		}
	})

	t.Run("switching store rewrites the Vault SPC", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "true")
		withKeyVaultSecrets(t, nil, keyVaultTestSecret("s1", platform, namespace, domain, "A"))
		configDomain := newConfigurationDomain(domain, namespace, platform, false, true)
		vaultSpc := (&secretsHandler{}).aggregateSecrets(configDomain, []secretSpec{{Key: "A", Path: "p"}}, platform+"-readonly")
		c := run(vaultSpc)

		assert.True(t, c.processNextWorkItem())

		spc, err := getSpc(c)
		if assert.NoError(t, err) {
			assert.Equal(t, csiv1.Provider("azure"), spc.Spec.Provider)
			assert.NotContains(t, spc.Spec.Parameters, "roleName")
		}
	})

	t.Run("AzureKeyVault without AZURE_ENABLED", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "false")
		withKeyVaultSecrets(t, errors.New("must not be listed"))
		c := run()
		recorder := useFakeRecorder(c)

		assert.True(t, c.processNextWorkItem())

		assert.True(t, hasEventContaining(recordedEvents(recorder), `platform "qa" uses Azure Key Vault, but AZURE_ENABLED is not true`))
		_, err := getSpc(c)
		assert.Error(t, err, "no SPC")
	})

	t.Run("listing fails", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "true")
		withKeyVaultSecrets(t, errors.New("403 Forbidden"))
		c := run()
		recorder := useFakeRecorder(c)

		assert.True(t, c.processNextWorkItem())

		assert.True(t, hasEventContaining(recordedEvents(recorder), "403 Forbidden"))
		_, err := getSpc(c)
		assert.Error(t, err, "no SPC")
	})

	t.Run("duplicate key event", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "true")
		withKeyVaultSecrets(t, nil,
			keyVaultTestSecret("b-secret", platform, namespace, domain, "K"),
			keyVaultTestSecret("a-secret", platform, namespace, domain, "K"))
		c := run()
		recorder := useFakeRecorder(c)

		assert.True(t, c.processNextWorkItem())

		events := recordedEvents(recorder)
		assert.True(t, hasEventContaining(events, `configuration key "K" is in Azure Key Vault secrets "a-secret" and "b-secret"; using "a-secret"`), "events: %v", events)
	})

	t.Run("re-queues after a Key Vault sync", func(t *testing.T) {
		t.Setenv(EnvAzureEnabled, "true")
		withKeyVaultSecrets(t, nil)
		requeueInterval = time.Millisecond
		t.Cleanup(func() { requeueInterval = 2 * time.Minute })
		c := run()

		assert.True(t, c.processNextWorkItem())

		assert.Eventually(t, func() bool { return c.workqueue.Len() == 1 }, 5*time.Second, 5*time.Millisecond)
		key, _ := c.workqueue.Get()
		assert.Equal(t, encodeDomainKey(namespace, domain), key)
	})
}

