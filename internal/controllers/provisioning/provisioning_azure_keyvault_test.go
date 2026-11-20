package provisioning

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"totalsoft.ro/platform-controllers/internal/messaging"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func TestProvisioningPassesThePlatformsSecretStore(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	keyVault := &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-prod", ResourceGroupName: "rg-lsng-prod"}
	platformWithVault := func() *platformv1.Platform {
		platform := newPlatform("dev")
		platform.Spec.SecretStore = &platformv1.SecretStoreSpec{Type: platformv1.SecretStoreTypeAzureKeyVault, AzureKeyVault: keyVault}
		return platform
	}
	keyVaultStore := &SecretStore{Type: platformv1.SecretStoreTypeAzureKeyVault, AzureKeyVault: keyVault}
	vaultStore := &SecretStore{Type: platformv1.SecretStoreTypeHashicorpVault}

	t.Run("a tenant's provisioning gets its platform's store", func(t *testing.T) {
		domain := "my-domain"
		objects := []runtime.Object{platformWithVault(), newTenant("dev1", "dev", domain), newAzureDb("db1", "dev", domain)}

		var infras []*InfrastructureManifests
		c := runController(objects, fakeProvisioner{
			provision: func(_ ProvisioningTarget, _ string, infra *InfrastructureManifests) ProvisioningResult {
				infras = append(infras, infra)
				return ProvisioningResult{}
			},
		}, messaging.NilMessagingPublisher)

		assert.True(t, c.processNextWorkItem(1))
		if assert.Len(t, infras, 1) {
			assert.Equal(t, keyVaultStore, infras[0].SecretStore)
			assert.Len(t, infras[0].AzureDbs, 1)
		}
	})

	t.Run("resolves the store of the target's platform", func(t *testing.T) {
		c := runController([]runtime.Object{platformWithVault(), newPlatform("other")}, fakeProvisioner{}, messaging.NilMessagingPublisher)

		tests := []struct {
			name   string
			target ProvisioningTarget
			want   *SecretStore
		}{
			{"platform with a vault", platformWithVault(), keyVaultStore},
			{"platform without secretStore", newPlatform("other"), vaultStore},
			{"tenant of a platform with a vault", newTenant("dev1", "dev"), keyVaultStore},
			{"tenant of a platform without secretStore", newTenant("other1", "other"), vaultStore},
			{"tenant of a missing platform", newTenant("gone1", "gone"), nil},
		}
		for _, tt := range tests {
			got, err := c.resolveSecretStore(tt.target)
			assert.NoError(t, err, tt.name)
			assert.Equal(t, tt.want, got, tt.name)
		}
	})
}

func TestPlatformSecretStoreChangeReenqueuesItsDomains(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	objects := []runtime.Object{
		newPlatform("dev"), newPlatform("other"),
		newDomain("my-domain", "dev"), newDomain("other-domain", "other"),
		newTenant("dev1", "dev", "my-domain"), newTenant("other1", "other", "other-domain"),
	}
	c := runController(objects, fakeProvisioner{}, messaging.NilMessagingPublisher)
	if !cache.WaitForCacheSync(nil, c.handlerSyncFuncs()...) {
		t.Fatal("event handlers were not notified of the initial list")
	}
	c.drainWorkqueue()

	update := func(change func(*platformv1.Platform)) {
		platform, err := c.clientset.PlatformV1alpha1().Platforms().Get(context.TODO(), "dev", metav1.GetOptions{})
		assert.NoError(t, err)
		change(platform)
		_, err = c.clientset.PlatformV1alpha1().Platforms().Update(context.TODO(), platform, metav1.UpdateOptions{})
		assert.NoError(t, err)
	}
	enqueuedKeys := func() []string {
		var keys []string
		for c.workqueue.Len() > 0 {
			key, _ := c.workqueue.Get()
			keys = append(keys, key.(string))
			c.workqueue.Done(key)
		}
		return keys
	}

	update(func(p *platformv1.Platform) { p.Labels = map[string]string{"unrelated": "change"} })
	assert.Never(t, func() bool { return c.workqueue.Len() > 0 }, 300*time.Millisecond, 20*time.Millisecond,
		"a change other than the secret store should not re-enqueue anything")

	update(func(p *platformv1.Platform) {
		p.Spec.SecretStore = &platformv1.SecretStoreSpec{Type: platformv1.SecretStoreTypeAzureKeyVault, AzureKeyVault: &platformv1.AzureKeyVaultSpec{
			Name: "kv-lsng-prod", ResourceGroupName: "rg-lsng-prod", WorkloadIdentityClientId: "00000000-0000-0000-0000-000000000001"}}
	})
	assert.Eventually(t, func() bool { return c.workqueue.Len() == 2 }, 10*time.Second, 20*time.Millisecond)
	assert.ElementsMatch(t, []string{
		encodeKey("dev", "", "my-domain"),
		encodeKey("dev", "default/dev1", "my-domain"),
	}, enqueuedKeys(), "the platform's domain, for the platform and for its tenant, and nothing of the other platform")

	update(func(p *platformv1.Platform) {
		p.Spec.SecretStore.AzureKeyVault.WorkloadIdentityClientId = "00000000-0000-0000-0000-000000000002"
	})
	assert.Never(t, func() bool { return c.workqueue.Len() > 0 }, 300*time.Millisecond, 20*time.Millisecond,
		"the pods' identity is not used by any export")
}

func TestSecretStoreChanged(t *testing.T) {
	kv := func(name, rg, clientId string) platformv1.PlatformSpec {
		return platformv1.PlatformSpec{SecretStore: &platformv1.SecretStoreSpec{Type: platformv1.SecretStoreTypeAzureKeyVault,
			AzureKeyVault: &platformv1.AzureKeyVaultSpec{Name: name, ResourceGroupName: rg, WorkloadIdentityClientId: clientId}}}
	}
	vault := platformv1.PlatformSpec{SecretStore: &platformv1.SecretStoreSpec{Type: platformv1.SecretStoreTypeHashicorpVault}}

	assert.False(t, secretStoreChanged(platformv1.PlatformSpec{}, vault), "absent is HashicorpVault")
	assert.True(t, secretStoreChanged(vault, kv("kv", "rg", "a")))
	assert.True(t, secretStoreChanged(kv("kv", "rg", "a"), kv("kv2", "rg", "a")))
	assert.True(t, secretStoreChanged(kv("kv", "rg", "a"), kv("kv", "rg2", "a")))
	assert.False(t, secretStoreChanged(kv("kv", "rg", "a"), kv("kv", "rg", "b")), "the pods' identity only")
}
