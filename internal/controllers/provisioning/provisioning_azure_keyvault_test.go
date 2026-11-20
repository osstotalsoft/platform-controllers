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

func TestProvisioningPassesThePlatformsAzureKeyVault(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	keyVault := &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-prod", ResourceGroupName: "rg-lsng-prod"}
	platformWithVault := func() *platformv1.Platform {
		platform := newPlatform("dev")
		platform.Spec.AzureKeyVault = keyVault
		return platform
	}

	t.Run("a tenant's provisioning gets its platform's vault", func(t *testing.T) {
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
			assert.Equal(t, keyVault, infras[0].AzureKeyVault)
			assert.Len(t, infras[0].AzureDbs, 1)
		}
	})

	t.Run("resolves the vault of the target's platform", func(t *testing.T) {
		c := runController([]runtime.Object{platformWithVault(), newPlatform("other")}, fakeProvisioner{}, messaging.NilMessagingPublisher)

		tests := []struct {
			name   string
			target ProvisioningTarget
			want   *platformv1.AzureKeyVaultSpec
		}{
			{"platform with a vault", platformWithVault(), keyVault},
			{"platform without a vault", newPlatform("other"), nil},
			{"tenant of a platform with a vault", newTenant("dev1", "dev"), keyVault},
			{"tenant of a platform without a vault", newTenant("other1", "other"), nil},
			{"tenant of a missing platform", newTenant("gone1", "gone"), nil},
		}
		for _, tt := range tests {
			got, err := c.resolveAzureKeyVault(tt.target)
			assert.NoError(t, err, tt.name)
			assert.Equal(t, tt.want, got, tt.name)
		}
	})
}

func TestPlatformAzureKeyVaultChangeReenqueuesItsDomains(t *testing.T) {
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

	update(func(p *platformv1.Platform) { p.Labels = map[string]string{"unrelated": "change"} })
	assert.Never(t, func() bool { return c.workqueue.Len() > 0 }, 300*time.Millisecond, 20*time.Millisecond,
		"a change other than the vault should not re-enqueue anything")

	update(func(p *platformv1.Platform) {
		p.Spec.AzureKeyVault = &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-prod", ResourceGroupName: "rg-lsng-prod"}
	})
	assert.Eventually(t, func() bool { return c.workqueue.Len() == 2 }, 10*time.Second, 20*time.Millisecond)

	var keys []string
	for c.workqueue.Len() > 0 {
		key, _ := c.workqueue.Get()
		keys = append(keys, key.(string))
		c.workqueue.Done(key)
	}
	assert.ElementsMatch(t, []string{
		encodeKey("dev", "", "my-domain"),
		encodeKey("dev", "default/dev1", "my-domain"),
	}, keys, "the platform's domain, for the platform and for its tenant, and nothing of the other platform")
}
