package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestDeployMsSqlDatabase(t *testing.T) {
	t.Run("maximal msSqlDatabase spec", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		mssqlDb := &provisioningv1.MsSqlDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-db",
			},
			Spec: provisioningv1.MsSqlDatabaseSpec{
				DbName: "my-db",
				SqlServer: provisioningv1.MsSqlServerSpec{
					HostName: "localhost",
					Port:     1433,
					SqlAuth: provisioningv1.MsSqlServerAuth{
						Username: "admin",
						Password: "password",
					},
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			user, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, user)
			return nil

		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
