package pulumi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
			bucket, err := deployMinioBucket(tenant, minioBucket, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, bucket)
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

func TestDeployMinioBucketImport(t *testing.T) {
	newMinioBucket := func(importBucketName string) *provisioningv1.MinioBucket {
		return &provisioningv1.MinioBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-buc",
			},
			Spec: provisioningv1.MinioBucketSpec{
				BucketName:       "buc1",
				ImportBucketName: importBucketName,
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
	}

	deploy := func(t *testing.T, minioBucket *provisioningv1.MinioBucket) *resourceCaptureMocks {
		tenant := newTenant("tenant1", "dev")
		mocks := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMinioBucket(tenant, minioBucket, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
		return mocks
	}

	t.Run("imports the existing bucket and grants access to it", func(t *testing.T) {
		mocks := deploy(t, newMinioBucket("legacy-bucket"))

		bucket := mocks.byName["my-buc"]
		assert.Equal(t, "legacy-bucket", bucket.ID)
		assert.Equal(t, "legacy-bucket", bucket.Inputs["bucket"].StringValue())
		assert.Contains(t, bucket.RegisterRPC.GetIgnoreChanges(), "forceDestroy")

		sa := mocks.byType["minio:index/iamServiceAccount:IamServiceAccount"]
		assert.Len(t, sa, 1)
		policy := sa[0].Inputs["policy"].StringValue()
		assert.Contains(t, policy, "arn:aws:s3:::legacy-bucket")
		assert.NotContains(t, policy, "buc1-dev-tenant1")
	})

	t.Run("creates a new bucket when import is not requested", func(t *testing.T) {
		mocks := deploy(t, newMinioBucket(""))

		bucket := mocks.byName["my-buc"]
		assert.Empty(t, bucket.ID)
		assert.Equal(t, "buc1-dev-tenant1", bucket.Inputs["bucket"].StringValue())
		assert.NotContains(t, bucket.RegisterRPC.GetIgnoreChanges(), "forceDestroy")
	})
}

func TestCheckMinioReachable(t *testing.T) {
	newBucket := func(server string) *provisioningv1.MinioBucket {
		return &provisioningv1.MinioBucket{
			Spec: provisioningv1.MinioBucketSpec{
				MinioServer: &provisioningv1.MinioServerSpec{Server: server},
			},
		}
	}
	newMinio := func(status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/minio/health/live" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(status)
		}))
	}

	t.Run("passes when the server is live", func(t *testing.T) {
		srv := newMinio(http.StatusOK)
		defer srv.Close()

		assert.NoError(t, checkMinioReachable(context.Background(), []*provisioningv1.MinioBucket{newBucket(srv.URL)}))
	})

	t.Run("adds the http scheme to a host:port server", func(t *testing.T) {
		srv := newMinio(http.StatusOK)
		defer srv.Close()

		hostPort := strings.TrimPrefix(srv.URL, "http://")
		assert.NoError(t, checkMinioReachable(context.Background(), []*provisioningv1.MinioBucket{newBucket(hostPort)}))
	})

	t.Run("fails when the server is not healthy", func(t *testing.T) {
		srv := newMinio(http.StatusServiceUnavailable)
		defer srv.Close()

		err := checkMinioReachable(context.Background(), []*provisioningv1.MinioBucket{newBucket(srv.URL)})
		assert.ErrorContains(t, err, "is not healthy")
	})

	t.Run("fails when the server refuses the connection", func(t *testing.T) {
		srv := newMinio(http.StatusOK)
		srv.Close()

		err := checkMinioReachable(context.Background(), []*provisioningv1.MinioBucket{newBucket(srv.URL)})
		assert.ErrorContains(t, err, "is unreachable")
	})

	t.Run("checks every server the buckets use", func(t *testing.T) {
		live := newMinio(http.StatusOK)
		defer live.Close()
		down := newMinio(http.StatusOK)
		down.Close()

		err := checkMinioReachable(context.Background(), []*provisioningv1.MinioBucket{newBucket(live.URL), newBucket(down.URL)})
		assert.ErrorContains(t, err, down.URL)
	})

	t.Run("passes when there are no buckets", func(t *testing.T) {
		t.Setenv("MINIO_ENDPOINT", "127.0.0.1:1")

		assert.NoError(t, checkMinioReachable(context.Background(), nil))
	})
}

func TestMinioEndpoints(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "minio.default:9000")

	endpoints := minioEndpoints([]*provisioningv1.MinioBucket{
		{},
		{Spec: provisioningv1.MinioBucketSpec{MinioServer: &provisioningv1.MinioServerSpec{Server: "minio.custom:9000"}}},
		{},
		{Spec: provisioningv1.MinioBucketSpec{MinioServer: &provisioningv1.MinioServerSpec{Server: "minio.custom:9000"}}},
	})

	assert.Equal(t, []string{"minio.default:9000", "minio.custom:9000"}, endpoints)
}

func TestMinioEndpointsWithoutDefaultServer(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "")

	assert.Empty(t, minioEndpoints([]*provisioningv1.MinioBucket{{}}))
}
