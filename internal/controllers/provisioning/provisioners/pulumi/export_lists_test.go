package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// The tests in this file cover the exports[] users and identities lists end to end: every value an
// entry's lists export lands in the domain's single ConfigMap/Secret/vault secret, next to the values
// of the deprecated single fields, without registering any extra resource.

// TestExportListsAzureManagedDbIdentities is the charisma-online shape: two managed identities whose
// client ids are both published into the hr-admin domain's one ConfigMap.
func TestExportListsAzureManagedDbIdentities(t *testing.T) {
	setAzureMssqlAuthEnv(t)
	tenant := newTenant("tenant1", "dev")

	azureDb := newAzureManagedDb("charisma-online-db")
	azureDb.Namespace = "qa"
	azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{
		{Name: "charisma-online", ResourceGroupName: "SQL_Managed_Instance", Location: "westeurope", Roles: []string{"db_datareader"}},
		{Name: "charisma-online-migrations", ResourceGroupName: "SQL_Managed_Instance", Location: "westeurope", Roles: []string{"db_owner"}},
	}
	azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{{
		Domain: "hr-admin",
		Identities: []provisioningv1.ExportedIdentitySpec{
			{
				IdentityRef:      "charisma-online",
				IdentityClientId: configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Hcm_Database__IdentityClientId"),
			},
			{
				IdentityRef:      "charisma-online-migrations",
				IdentityClientId: configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Hcm_Database__MigrationClientId"),
			},
		},
	}}

	capture := newResourceCaptureMocks()
	capture.stubIdentityIds("charisma-online-db-charisma-online-identity", "aaaaaaaa-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000001")
	capture.stubIdentityIds("charisma-online-db-charisma-online-migrations-identity", "aaaaaaaa-0000-0000-0000-000000000002", "bbbbbbbb-0000-0000-0000-000000000002")
	stubIdentityUserLookup(capture, "12/7")
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"azure-native:managedidentity:UserAssignedIdentity charisma-online-db-charisma-online-identity",
		"azure-native:managedidentity:UserAssignedIdentity charisma-online-db-charisma-online-migrations-identity",
		"azure-native:sql:ManagedDatabase charisma-online-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-charisma-online-db",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-charisma-online-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__Hcm_Database__IdentityClientId=aaaaaaaa-0000-0000-0000-000000000001",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-charisma-online-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__Hcm_Database__MigrationClientId=aaaaaaaa-0000-0000-0000-000000000002",
		"mssql:index/databaseRoleMember:DatabaseRoleMember charisma-online-db-charisma-online-identity-role-db_datareader",
		"mssql:index/databaseRoleMember:DatabaseRoleMember charisma-online-db-charisma-online-migrations-identity-role-db_owner",
		"mssql:index/script:Script charisma-online-db-charisma-online-identity-user",
		"mssql:index/script:Script charisma-online-db-charisma-online-migrations-identity-user",
		"pulumi:providers:mssql charisma-online-db-mssql-provider",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}

// TestExportListsAzureDbUsersAndSingleIdentity exports two users through the users list and an
// identity through the deprecated single fields, all in one domain and spread over every destination.
func TestExportListsAzureDbUsersAndSingleIdentity(t *testing.T) {
	setAzureMssqlAuthEnv(t)
	t.Setenv(EnvVaultEnabled, "true")
	tenant := newTenant("tenant1", "dev")

	azureDb := newAzureDb("lists-azure-db")
	azureDb.Namespace = "qa"
	azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
		{Name: "app_user", Roles: []string{"db_owner"}},
		{Name: "reporting_user", Roles: []string{"db_datareader"}},
	}
	azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{{
		Name:              "app_identity",
		ResourceGroupName: "SQL_RG",
		Location:          "westeurope",
		Roles:             []string{"db_datareader"},
	}}
	azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{{
		Domain: "origination",
		Users: []provisioningv1.ExportedUserSpec{
			{UserRef: "app_user", Username: configMapExport("App__Username"), Password: kubeSecretExport("App__Password")},
			{UserRef: "reporting_user", Username: configMapExport("Reporting__Username"), Password: vaultExport("Reporting__Password")},
		},
		IdentityClientId: configMapExport("IdentityClientId"),
	}}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"azure-native:managedidentity:UserAssignedIdentity lists-azure-db-app_identity-identity",
		"azure-native:sql:Database lists-azure-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-azure-db",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-azure-db  App__Username=app_user",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-azure-db  IdentityClientId=11111111-1111-1111-1111-111111111111",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-azure-db  Reporting__Username=reporting_user",
		"kubernetes:core/v1:Secret origination-tenant1-lists-azure-db",
		"kubernetes:core/v1:Secret origination-tenant1-lists-azure-db  App__Password=Test-Password-123!",
		"mssql:index/azureadServicePrincipal:AzureadServicePrincipal lists-azure-db-app_identity-identity-user",
		"mssql:index/databaseRoleMember:DatabaseRoleMember lists-azure-db-app_identity-identity-role-db_datareader",
		"mssql:index/databaseRoleMember:DatabaseRoleMember lists-azure-db-app_user-role-db_owner",
		"mssql:index/databaseRoleMember:DatabaseRoleMember lists-azure-db-reporting_user-role-db_datareader",
		"mssql:index/script:Script lists-azure-db-app_user-contained-user",
		"mssql:index/script:Script lists-azure-db-reporting_user-contained-user",
		"pulumi:providers:mssql lists-azure-db-mssql-provider",
		"random:index/randomPassword:RandomPassword lists-azure-db-app_user-contained-password",
		"random:index/randomPassword:RandomPassword lists-azure-db-reporting_user-contained-password",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/lists-azure-db",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/lists-azure-db  dataJson={\"Reporting__Password\":\"Test-Password-123!\"}",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/lists-azure-db  path=dev/qa/origination/tenant1/lists-azure-db",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}

// TestExportListsMsSqlDbUsers exports two users' credentials into one domain through the users list.
func TestExportListsMsSqlDbUsers(t *testing.T) {
	tenant := newTenant("tenant1", "dev")

	mssqlDb := newMsSqlDb("lists-mssql-db")
	mssqlDb.Namespace = "qa"
	mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
		{Name: "app_user", Roles: []string{"db_owner"}},
		{Name: "reporting_user", Roles: []string{"db_datareader"}},
	}
	mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{{
		Domain: "origination",
		DbName: configMapExport("Database"),
		Users: []provisioningv1.ExportedUserSpec{
			{UserRef: "app_user", Username: configMapExport("App__Username"), Password: kubeSecretExport("App__Password")},
			{UserRef: "reporting_user", Username: configMapExport("Reporting__Username"), Password: kubeSecretExport("Reporting__Password")},
		},
	}}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-mssql-db",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-mssql-db  App__Username=app_user_lists-mssql-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-mssql-db  Database=lists-mssql-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap origination-tenant1-lists-mssql-db  Reporting__Username=reporting_user_lists-mssql-db_dev_tenant1",
		"kubernetes:core/v1:Secret origination-tenant1-lists-mssql-db",
		"kubernetes:core/v1:Secret origination-tenant1-lists-mssql-db  App__Password=Test-Password-123!",
		"kubernetes:core/v1:Secret origination-tenant1-lists-mssql-db  Reporting__Password=Test-Password-123!",
		"mssql:index/database:Database lists-mssql-db",
		"mssql:index/databaseRoleMember:DatabaseRoleMember lists-mssql-db-app_user-role-db_owner",
		"mssql:index/databaseRoleMember:DatabaseRoleMember lists-mssql-db-reporting_user-role-db_datareader",
		"mssql:index/script:Script lists-mssql-db-app_user-login-sessions",
		"mssql:index/script:Script lists-mssql-db-reporting_user-login-sessions",
		"mssql:index/sqlLogin:SqlLogin lists-mssql-db-app_user-login",
		"mssql:index/sqlLogin:SqlLogin lists-mssql-db-reporting_user-login",
		"mssql:index/sqlUser:SqlUser lists-mssql-db-app_user-user",
		"mssql:index/sqlUser:SqlUser lists-mssql-db-reporting_user-user",
		"pulumi:providers:mssql provider-mssql",
		"random:index/randomPassword:RandomPassword lists-mssql-db-app_user-login-password",
		"random:index/randomPassword:RandomPassword lists-mssql-db-reporting_user-login-password",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}

// TestExportListsRejectsKeyCollision checks that the provisioners run validateExportKeys: two list
// items rendering the same ConfigMap key fail the deployment before anything is exported.
func TestExportListsRejectsKeyCollision(t *testing.T) {
	tenant := newTenant("tenant1", "dev")

	mssqlDb := newMsSqlDb("collision-mssql-db")
	mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app_user"}, {Name: "reporting_user"}}
	mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{{
		Domain: "origination",
		Users: []provisioningv1.ExportedUserSpec{
			{UserRef: "app_user", Username: configMapExport("{{ .Tenant.Code }}__Username")},
			{UserRef: "reporting_user", Username: configMapExport("{{ .Tenant.Code }}__Username")},
		},
	}}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.ErrorContains(t, err, `exports[domain=origination]: users[0].username and users[1].username both render the toConfigMap key "tenant1__Username"`)
	assert.False(t, capture.hasAnyTypeWithPrefix("kubernetes:"))
}
