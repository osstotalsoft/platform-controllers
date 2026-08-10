package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func newMsSqlDb(name string) *provisioningv1.MsSqlDatabase {
	return &provisioningv1.MsSqlDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: provisioningv1.MsSqlDatabaseSpec{
			DbName: name,
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
}

func TestDeployMsSqlDatabase(t *testing.T) {
	t.Run("maximal msSqlDatabase spec", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		mssqlDb := newMsSqlDb("my-db")

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// No Users configured must mean no mssql-namespaced user/login/role resource is registered
		// (the mssql.Database resource itself is a "mssql:" type, so this specifically checks for
		// login/user/role types, not the database).
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlUser:SqlUser"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/databaseRoleMember:DatabaseRoleMember"))
	})

	t.Run("msSqlDatabase spec with one user, implicit userRef", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant2", platform)
		mssqlDb := newMsSqlDb("my-db-with-user")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
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

	t.Run("msSqlDatabase exports username and password (implicit userRef)", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant3", platform)
		mssqlDb := newMsSqlDb("my-db-with-exports")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{
			{
				Domain: "myDomain",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
				Password: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "password"},
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

	t.Run("multiple users, each exported to its own domain via explicit userRef", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant4", platform)
		mssqlDb := newMsSqlDb("my-db-multi-user")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
			{Name: "reporting_app", Roles: []string{"db_datareader"}},
		}
		mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "origination_app",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
			{
				Domain:  "reporting",
				UserRef: "reporting_app",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
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
		assert.Len(t, configMaps, 2, "each exports[] entry (one per app) must produce its own ConfigMap")
		usernames := map[string]bool{}
		for _, cm := range configMaps {
			data := cm.Inputs["data"].ObjectValue()
			usernames[data[resource.PropertyKey("username")].StringValue()] = true
		}
		assert.True(t, usernames["origination_app"])
		assert.True(t, usernames["reporting_app"])
	})

	t.Run("duplicate user name fails fast", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant5", platform)
		mssqlDb := newMsSqlDb("my-db-dup-user")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app1", Roles: []string{"db_datareader"}},
		}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, `spec.users[].name "app1" is duplicated`)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
	})

	t.Run("unknown userRef fails", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant6", platform)
		mssqlDb := newMsSqlDb("my-db-bad-ref")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
		}
		mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "does_not_exist",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, `userRef "does_not_exist" does not match any spec.users[].name`)
	})

	t.Run("ambiguous userRef fails", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant7", platform)
		mssqlDb := newMsSqlDb("my-db-ambiguous-ref")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app2", Roles: []string{"db_owner"}},
		}
		mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{
			{
				Domain: "origination",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, "userRef is required when spec.users does not have exactly one entry")
	})
}
