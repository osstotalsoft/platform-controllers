package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

	t.Run("no users, no managed identities — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db")
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.False(t, capture.hasAnyTypeWithPrefix("pulumi:providers:mssql"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"))
	})

	t.Run("with login+user (ContainedUser false)", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-login-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// Non-contained mode goes through a real server-level login, never the contained-user
		// Script.
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlUser:SqlUser"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/script:Script"))
	})

	t.Run("with contained user (ContainedUser true)", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-contained-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.ContainedUser = true
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// ContainedUser:true must go through the Script resource, and must NOT also create a
		// server-level login (the two modes are mutually exclusive branches).
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/script:Script"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
	})

	t.Run("ContainedUser applies uniformly to every user in the list", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-multi-contained")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
			{Name: "reporting_app", Roles: []string{"db_datareader"}},
		}
		azureDb.Spec.ContainedUser = true
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		scripts := capture.byType["mssql:index/script:Script"]
		assert.Len(t, scripts, 2, "both users must go through the contained-user Script path")
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
	})

	t.Run("with managed identity", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-identity")
		azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
			{
				Name:              "origination_app_identity",
				ResourceGroupName: "SQLMI_RG",
				Location:          "westeurope",
				Roles:             []string{"db_owner"},
			},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/azureadServicePrincipal:AzureadServicePrincipal"))
		assert.True(t, capture.hasAnyTypeWithPrefix("azure-native:managedidentity"))
	})

	t.Run("exports username, password, identityClientId and identityPrincipalId (implicit refs)", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-exports")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
			{
				Name:              "origination_app_identity",
				ResourceGroupName: "SQLMI_RG",
				Location:          "westeurope",
				Roles:             []string{"db_owner"},
			},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{
			{
				Domain: "myDomain",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
				Password: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "password"},
				},
				IdentityClientId: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "identityClientId"},
				},
				IdentityPrincipalId: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "identityPrincipalId"},
				},
			},
		}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		configMaps := capture.byType["kubernetes:core/v1:ConfigMap"]
		assert.Len(t, configMaps, 1, "exactly one ConfigMap should be exported for the single Exports entry")

		data := configMaps[0].Inputs["data"].ObjectValue()
		for _, key := range []string{"username", "password", "identityClientId", "identityPrincipalId"} {
			val, ok := data[resource.PropertyKey(key)]
			assert.True(t, ok, "expected export key %q to be present in the exported ConfigMap data", key)
			assert.NotEmpty(t, val.StringValue(), "expected export key %q to have a non-empty value", key)
		}
	})

	t.Run("multiple users, each exported to its own domain via explicit userRef", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-multi-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
			{Name: "reporting_app", Roles: []string{"db_datareader"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{
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
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-dup-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app1", Roles: []string{"db_datareader"}},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, `spec.users[].name "app1" is duplicated`)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"))
	})

	t.Run("unknown userRef fails", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-bad-ref")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "does_not_exist",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, `userRef "does_not_exist" does not match any spec.users[].name`)
	})

	t.Run("ambiguous userRef fails", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-ambiguous-ref")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app2", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{
			{
				Domain: "origination",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, "userRef is required when spec.users does not have exactly one entry")
	})

	// TestDeployAzureManagedDbResolvesRealHostname guards against the fabricated-hostname bug where
	// the mssql connection hostname was built as "<mi-name>.<resourceGroup>.database.windows.net" —
	// a real SQL MI's FQDN is "<mi-name>.<dnsZone>.database.windows.net", where dnsZone is an
	// Azure-generated virtual-cluster identifier that has nothing to do with the resource group. This
	// asserts the mssql provider's hostname comes from LookupManagedInstance's
	// FullyQualifiedDomainName, not a hand-built resource-group-based string.
	t.Run("resolves the real MI hostname via LookupManagedInstance, not a resource-group guess", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureManagedDb("my-mi-db-hostname")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}

		capture := newResourceCaptureMocks()
		const realFqdn = "incubsqlmi.a1b2c3d4e5f6.database.windows.net"
		capture.stubCall("azure-native:sql:getManagedInstance", resource.PropertyMap{
			"fullyQualifiedDomainName": resource.NewStringProperty(realFqdn),
			"dnsZone":                  resource.NewStringProperty("a1b2c3d4e5f6"),
		})

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		providers := capture.byType["pulumi:providers:mssql"]
		assert.Len(t, providers, 1)
		hostname := providers[0].Inputs["hostname"].StringValue()
		assert.Equal(t, realFqdn, hostname)

		wrongGuess := "incubsqlmi.SQLMI_RG.database.windows.net"
		assert.NotEqual(t, wrongGuess, hostname,
			"hostname must not be built as <mi-name>.<resourceGroup>.database.windows.net — that is not a real MI FQDN")
	})

	t.Run("mssql provider resource name is unique per CR", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		dbA := newAzureManagedDb("mi-db-alpha")
		dbA.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		dbB := newAzureManagedDb("mi-db-beta")
		dbB.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			if _, err := deployAzureManagedDb(tenant, dbA, []pulumi.Resource{}, ctx); err != nil {
				return err
			}
			_, err := deployAzureManagedDb(tenant, dbB, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		providers := capture.byType["pulumi:providers:mssql"]
		assert.Len(t, providers, 2, "each AzureManagedDatabase must register its own mssql provider")
		names := map[string]bool{}
		for _, p := range providers {
			names[p.Name] = true
		}
		assert.True(t, names["mi-db-alpha-mssql-provider"])
		assert.True(t, names["mi-db-beta-mssql-provider"])
	})

	t.Run("fails fast when azure auth creds are incomplete and workload identity is disabled", func(t *testing.T) {
		t.Setenv("AZURE_USE_WORKLOAD_IDENTITY", "false")
		t.Setenv("AZURE_CLIENT_ID", "")
		t.Setenv("AZURE_CLIENT_SECRET", "")
		t.Setenv("AZURE_TENANT_ID", "")

		azureDb := newAzureManagedDb("my-mi-db-badauth")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AZURE_CLIENT_ID")
	})

	t.Run("workload identity mode sets an empty AzureAuth and does not require SP creds", func(t *testing.T) {
		t.Setenv("AZURE_USE_WORKLOAD_IDENTITY", "true")
		t.Setenv("AZURE_CLIENT_ID", "")
		t.Setenv("AZURE_CLIENT_SECRET", "")
		t.Setenv("AZURE_TENANT_ID", "")

		azureDb := newAzureManagedDb("my-mi-db-wi")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		providers := capture.byType["pulumi:providers:mssql"]
		assert.Len(t, providers, 1)
		azureAuthVal, hasAzureAuth := providers[0].Inputs["azureAuth"]
		assert.True(t, hasAzureAuth, "AzureAuth must be present (though empty) under Workload Identity so the provider selects AAD auth mode and falls back to the default Azure credential chain")
		assert.True(t, azureAuthVal.IsObject(), "azureAuth must be an object value")
		azureAuthObj := azureAuthVal.ObjectValue()
		for _, key := range []string{"clientId", "clientSecret", "tenantId"} {
			assert.NotContains(t, azureAuthObj, resource.PropertyKey(key), "azureAuth must have no credential fields set under Workload Identity")
		}
	})
}
