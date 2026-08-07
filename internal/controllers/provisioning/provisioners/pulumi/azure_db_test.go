package pulumi

import (
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

	t.Run("no user, no managed identity — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db")
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		// No User/ManagedIdentity configured must mean no mssql provider, and no mssql-namespaced
		// resource of any kind, is ever registered.
		assert.False(t, capture.hasAnyTypeWithPrefix("pulumi:providers:mssql"))
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:"))
	})

	t.Run("with contained user", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
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

	t.Run("with managed identity", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-mi")
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQL_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
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

	t.Run("exports username, password, identityClientId and identityPrincipalId", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		azureDb := newAzureDb("my-azure-db-exports")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQL_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
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

	t.Run("fails fast when azure auth creds are incomplete and workload identity is disabled", func(t *testing.T) {
		t.Setenv("AZURE_USE_WORKLOAD_IDENTITY", "false")
		t.Setenv("AZURE_CLIENT_ID", "")
		t.Setenv("AZURE_CLIENT_SECRET", "")
		t.Setenv("AZURE_TENANT_ID", "")

		azureDb := newAzureDb("my-azure-db-badauth")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AZURE_CLIENT_ID")
	})

	t.Run("workload identity mode omits AzureAuth and does not require SP creds", func(t *testing.T) {
		t.Setenv("AZURE_USE_WORKLOAD_IDENTITY", "true")
		t.Setenv("AZURE_CLIENT_ID", "")
		t.Setenv("AZURE_CLIENT_SECRET", "")
		t.Setenv("AZURE_TENANT_ID", "")

		azureDb := newAzureDb("my-azure-db-wi")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
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
		_, hasAzureAuth := providers[0].Inputs["azureAuth"]
		assert.False(t, hasAzureAuth, "AzureAuth must be omitted under Workload Identity so the provider falls back to the default Azure credential chain")
	})

	t.Run("mssql provider resource name is unique per CR", func(t *testing.T) {
		setAzureMssqlAuthEnv(t)
		dbA := newAzureDb("db-alpha")
		dbA.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		dbB := newAzureDb("db-beta")
		dbB.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}

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
}
