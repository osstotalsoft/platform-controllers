package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
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
