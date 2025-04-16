package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestDeployMinioBucket(t *testing.T) {
	t.Run("minio bucket test", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		minioBucket := &provisioningv1.MinioBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-buc",
			},
			Spec: provisioningv1.MinioBucketSpec{
				BucketName: "buc1",
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			user, err := deployMinioBucket(tenant, minioBucket, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, user)
			return nil

		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
