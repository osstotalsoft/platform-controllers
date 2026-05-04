package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type minioMocks struct {
	providerInputs []resource.PropertyMap
}

func (m *minioMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	if args.TypeToken == "pulumi:providers:minio" {
		m.providerInputs = append(m.providerInputs, args.Inputs)
	}
	return args.Name + "_id", args.Inputs, nil
}

func (m *minioMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestDeployMinioBucket(t *testing.T) {
	t.Run("uses default provider when server is omitted", func(t *testing.T) {
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
		mocks := &minioMocks{}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			user, err := deployMinioBucket(tenant, minioBucket, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, user)
			return nil

		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
		assert.Empty(t, mocks.providerInputs)
	})

	t.Run("uses explicit provider when server is configured", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		minioBucket := &provisioningv1.MinioBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-buc",
			},
			Spec: provisioningv1.MinioBucketSpec{
				BucketName: "buc1",
				MinioServer: &provisioningv1.MinioServerSpec{
					Server:   "minio.example.com:9000",
					User:     "minio-user",
					Password: "minio-password",
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
		mocks := &minioMocks{}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			bucket, err := deployMinioBucket(tenant, minioBucket, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, bucket)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
		assert.Len(t, mocks.providerInputs, 1)
		assert.Equal(t, "minio.example.com:9000", mocks.providerInputs[0]["minioServer"].StringValue())
		assert.Equal(t, "minio-user", mocks.providerInputs[0]["minioUser"].StringValue())
		assert.Equal(t, "minio-password", mocks.providerInputs[0]["minioPassword"].StringValue())
	})
}
