package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			user, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, user)
			return nil

		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// No User configured must mean no mssql-namespaced user/login/role resource is registered
		// (the mssql.Database resource itself is a "mssql:" type, so this specifically checks for
		// login/user/role types, not the database).
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlUser:SqlUser"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/databaseRoleMember:DatabaseRoleMember"))
	})

	t.Run("msSqlDatabase spec with user", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant2", platform)
		mssqlDb := &provisioningv1.MsSqlDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-db-with-user",
			},
			Spec: provisioningv1.MsSqlDatabaseSpec{
				DbName: "my-db-with-user",
				SqlServer: provisioningv1.MsSqlServerSpec{
					HostName: "localhost",
					Port:     1433,
					SqlAuth: provisioningv1.MsSqlServerAuth{
						Username: "admin",
						Password: "password",
					},
				},
				User: &provisioningv1.DatabaseUserSpec{
					Roles: []string{"db_owner"},
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// A User set on MsSqlDatabase always goes through the login+user chain (no contained-user
		// option exists for on-prem MsSqlDatabase).
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlUser:SqlUser"))
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/databaseRoleMember:DatabaseRoleMember"))
	})

	t.Run("msSqlDatabase exports username and password", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant3", platform)
		mssqlDb := &provisioningv1.MsSqlDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-db-with-exports",
			},
			Spec: provisioningv1.MsSqlDatabaseSpec{
				DbName: "my-db-with-exports",
				SqlServer: provisioningv1.MsSqlServerSpec{
					HostName: "localhost",
					Port:     1433,
					SqlAuth: provisioningv1.MsSqlServerAuth{
						Username: "admin",
						Password: "password",
					},
				},
				User: &provisioningv1.DatabaseUserSpec{
					Roles: []string{"db_owner"},
				},
				Exports: []provisioningv1.MsSqlDatabaseExportsSpec{
					{
						Domain: "myDomain",
						Username: provisioningv1.ValueExport{
							ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
						},
						Password: provisioningv1.ValueExport{
							ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "password"},
						},
					},
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		configMaps := capture.byType["kubernetes:core/v1:ConfigMap"]
		assert.Len(t, configMaps, 1, "exactly one ConfigMap should be exported for the single Exports entry")

		data := configMaps[0].Inputs["data"].ObjectValue()
		for _, key := range []string{"username", "password"} {
			val, ok := data[resource.PropertyKey(key)]
			assert.True(t, ok, "expected export key %q to be present in the exported ConfigMap data", key)
			assert.NotEmpty(t, val.StringValue(), "expected export key %q to have a non-empty value", key)
		}
	})
}
