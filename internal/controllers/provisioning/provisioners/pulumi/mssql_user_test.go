package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestDeployLoginUser(t *testing.T) {
	t.Run("creates login, user and role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, password, err := deployLoginUser(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"my-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "my-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("explicit name overrides default", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, _, err := deployLoginUser(ctx, provider, "my-db-2",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Name: "custom-user"},
				"my-db-2", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "custom-user", username)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func TestDeployContainedUser(t *testing.T) {
	t.Run("creates contained user with role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider-2", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, password, err := deployContainedUser(ctx, provider, "contained-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"contained-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "contained-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func TestDeployManagedIdentity(t *testing.T) {
	t.Run("creates identity, wires it in, grants roles", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider-3", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			clientId, principalId, err := deployManagedIdentity(ctx, provider, "my-db-identity",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.ManagedIdentitySpec{
					ResourceGroupName: "my-rg",
					Location:          "westeurope",
					Roles:             []string{"db_owner"},
				},
				"my-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.NotNil(t, clientId)
			assert.NotNil(t, principalId)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
