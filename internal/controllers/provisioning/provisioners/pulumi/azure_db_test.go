package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func newAzureDb(name string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: provisioningv1.AzureDatabaseSpec{
			DbName: name,
			SqlServer: provisioningv1.SqlServerSpec{
				ResourceGroupName: "SQL_RG",
				ServerName:        "testsvr",
			},
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				DomainRef: "example-domain",
			},
		},
	}
}

func TestDeployAzureDb(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)

	t.Run("no user, no managed identity — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db")
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with contained user", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with managed identity", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db-mi")
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQL_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
