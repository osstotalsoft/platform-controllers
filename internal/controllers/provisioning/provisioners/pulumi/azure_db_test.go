package pulumi

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

// setAzureMssqlAuthEnv sets the ambient service-principal env vars newMssqlAzureAuthProvider reads
// when Workload Identity is disabled, mirroring how the real deployment environment configures them
// (see pulumi.go's createOrSelectStack). Scoped to the calling (sub)test via t.Setenv.
func setAzureMssqlAuthEnv(t *testing.T) {
	t.Setenv("AZURE_CLIENT_ID", "test-client-id")
	t.Setenv("AZURE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("AZURE_TENANT_ID", "test-tenant-id")
}

func TestDeployAzureDb(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)

	t.Run("no users, no managed identities — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db")
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// No Users/ManagedIdentities configured must mean no mssql provider, and no mssql-namespaced
		// resource of any kind, is ever registered.
		assert.False(t, capture.hasAnyTypeWithPrefix("pulumi:providers:mssql"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"))
	})

	t.Run("with one contained user, implicit userRef", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// AzureDatabase has no server-login concept — a contained user must go through the Script
		// resource, never a SqlLogin.
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/script:Script"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/sqlLogin:SqlLogin"))
	})

	t.Run("grants permissions and schema permissions to a contained user", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-permissions")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{
				Name:              "origination_app",
				Roles:             []string{"db_owner"},
				Permissions:       []string{"EXECUTE"},
				SchemaPermissions: map[string][]string{"dbo": {"EXECUTE"}},
			},
		}
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{
			"id": resource.NewStringProperty("1/1"),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/databasePermission:DatabasePermission"))
		assert.True(t, capture.hasAnyTypeWithPrefix("mssql:index/schemaPermission:SchemaPermission"))
	})

	t.Run("with managed identity, implicit identityRef", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-mi")
		azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
			{
				Name:              "origination_app_identity",
				ResourceGroupName: "SQL_RG",
				Location:          "westeurope",
				Roles:             []string{"db_owner"},
			},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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
		azureDb := newAzureDb("my-azure-db-exports")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
			{
				Name:              "origination_app_identity",
				ResourceGroupName: "SQL_RG",
				Location:          "westeurope",
				Roles:             []string{"db_owner"},
			},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
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
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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
		azureDb := newAzureDb("my-azure-db-multi-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "origination_app", Roles: []string{"db_owner"}},
			{Name: "reporting_app", Roles: []string{"db_datareader"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "origination_app",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
				Password: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "password"},
				},
			},
			{
				Domain:  "reporting",
				UserRef: "reporting_app",
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
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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

	t.Run("duplicate user name fails fast, before any pulumi resource is created", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-dup-user")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app1", Roles: []string{"db_datareader"}},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, `spec.users[].name "app1" is duplicated`)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"), "no mssql resource must be created once validation fails")
	})

	t.Run("unknown userRef fails", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-bad-ref")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "does_not_exist",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, `userRef "does_not_exist" does not match any spec.users[].name`)
	})

	t.Run("ambiguous userRef (more than one user, no ref given) fails", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-ambiguous-ref")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app2", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
			{
				Domain: "origination",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, "userRef is required when spec.users does not have exactly one entry")
	})

	t.Run("fails fast when azure auth creds are incomplete and workload identity is disabled", func(t *testing.T) {
		t.Setenv("AZURE_USE_WORKLOAD_IDENTITY", "false")
		t.Setenv("AZURE_CLIENT_ID", "")
		t.Setenv("AZURE_CLIENT_SECRET", "")
		t.Setenv("AZURE_TENANT_ID", "")

		azureDb := newAzureDb("my-azure-db-badauth")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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

		azureDb := newAzureDb("my-azure-db-wi")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
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

	t.Run("mssql provider resource name is unique per CR", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		dbA := newAzureDb("db-alpha")
		dbA.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}
		dbB := newAzureDb("db-beta")
		dbB.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app1", Roles: []string{"db_owner"}}}

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			if _, err := deployAzureDb(tenant, dbA, []pulumi.Resource{}, ctx); err != nil {
				return err
			}
			_, err := deployAzureDb(tenant, dbB, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		providers := capture.byType["pulumi:providers:mssql"]
		assert.Len(t, providers, 2, "each AzureDatabase must register its own mssql provider")
		names := map[string]bool{}
		for _, p := range providers {
			names[p.Name] = true
		}
		assert.True(t, names["db-alpha-mssql-provider"])
		assert.True(t, names["db-beta-mssql-provider"])
	})

	t.Run("duplicate export domain fails fast, before any pulumi resource is created", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-dup-domain")
		azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
			{Name: "app1", Roles: []string{"db_owner"}},
			{Name: "app2", Roles: []string{"db_owner"}},
		}
		azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
			{
				Domain:  "origination",
				UserRef: "app1",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
			{
				Domain:  "origination",
				UserRef: "app2",
				Username: provisioningv1.ValueExport{
					ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: "username"},
				},
			},
		}
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, "is duplicated")
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"), "no mssql resource must be created once validation fails")
	})
}

// TestDeployAzureDbIdentityNameIsTenantScoped guards against the regression where
// deployManagedIdentity's real UserAssignedIdentity resource name was managedIdentities[].name
// verbatim. The identity is an ARM resource in a fixed resource group, shared across every tenant
// this CR is provisioned for — only the database itself is tenant-scoped — so two tenants
// configuring the same managedIdentities[].name must not silently share (and fight over the
// lifecycle of) the same ARM resource.
func TestDeployAzureDbIdentityNameIsTenantScoped(t *testing.T) {
	setAzureMssqlAuthEnv(t)
	platform := "dev"
	tenantA := newTenant("tenant-a", platform)
	tenantB := newTenant("tenant-b", platform)

	dbA := newAzureDb("db-id-tenant-a")
	dbA.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
		{Name: "identity1", ResourceGroupName: "SQL_RG", Location: "westeurope", Roles: []string{"db_owner"}},
	}
	dbB := newAzureDb("db-id-tenant-b")
	dbB.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
		{Name: "identity1", ResourceGroupName: "SQL_RG", Location: "westeurope", Roles: []string{"db_owner"}},
	}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		if _, err := deployAzureDb(tenantA, dbA, []pulumi.Resource{}, ctx); err != nil {
			return err
		}
		_, err := deployAzureDb(tenantB, dbB, []pulumi.Resource{}, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	identities := capture.byType["azure-native:managedidentity:UserAssignedIdentity"]
	assert.Len(t, identities, 2, "each tenant must get its own UserAssignedIdentity")
	names := map[string]bool{}
	for _, i := range identities {
		names[i.Inputs["resourceName"].StringValue()] = true
	}
	assert.Len(t, names, 2, "the two tenants' managed identities must have different resource names despite both configuring managedIdentities[].name=\"identity1\"")
	for name := range names {
		assert.True(t, strings.HasPrefix(name, "identity1_"), "identity resource name %q must be tenant-scoped", name)
	}
}
