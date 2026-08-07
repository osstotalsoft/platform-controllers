package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func newAzureManagedDb(name string) *provisioningv1.AzureManagedDatabase {
	return &provisioningv1.AzureManagedDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: provisioningv1.AzureManagedDatabaseSpec{
			DbName: name,
			ManagedInstance: provisioningv1.AzureManagedInstanceSpec{
				Name:          "incubsqlmi",
				ResourceGroup: "SQLMI_RG",
			},
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				DomainRef: "example-domain",
			},
		},
	}
}

func TestDeployAzureManagedDb(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)

	t.Run("no user, no managed identity — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db")
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with login+user (ContainedUser false)", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-login-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with contained user (ContainedUser true)", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-contained-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		azureDb.Spec.ContainedUser = true
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with managed identity", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-identity")
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQLMI_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
