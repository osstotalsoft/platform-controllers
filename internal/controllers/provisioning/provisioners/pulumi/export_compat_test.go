package pulumi

import (
	"fmt"
	"sort"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// The tests in this file pin what specs using the single exports[] user and identity fields
// (userRef/username/password, identityRef/identityClientId/identityPrincipalId) deploy today: every
// Pulumi resource by type and name, every exported ConfigMap/Secret key and value, and every vault
// path and payload. Any change to the export code must leave these snapshots untouched, so existing
// specs keep the same resources (no replace or delete) and the same exported keys. The Azure kinds'
// exported dbName value is empty only because the mocks don't return the database's name output.

func configMapExport(key string) provisioningv1.ValueExport {
	return provisioningv1.ValueExport{ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: key}}
}

func kubeSecretExport(key string) provisioningv1.ValueExport {
	return provisioningv1.ValueExport{ToKubeSecret: provisioningv1.KubeSecretTemplate{KeyTemplate: key}}
}

func vaultExport(key string) provisioningv1.ValueExport {
	return provisioningv1.ValueExport{ToVault: provisioningv1.VaultSecretTemplate{KeyTemplate: key}}
}

// exportSnapshot lists every captured resource as "<type> <name>", followed by one indented line per
// exported ConfigMap/Secret key and per vault path/payload, sorted so the result doesn't depend on
// the order resources were registered in.
func exportSnapshot(capture *resourceCaptureMocks) []string {
	capture.mu.Lock()
	defer capture.mu.Unlock()

	var lines []string
	for typeToken, resources := range capture.byType {
		for _, r := range resources {
			id := fmt.Sprintf("%s %s", typeToken, r.Name)
			lines = append(lines, id)
			switch typeToken {
			case "kubernetes:core/v1:ConfigMap":
				lines = append(lines, snapshotMap(id, r.Inputs["data"])...)
			case "kubernetes:core/v1:Secret":
				lines = append(lines, snapshotMap(id, r.Inputs["stringData"])...)
			case "vault:generic/secret:Secret":
				lines = append(lines,
					fmt.Sprintf("%s  path=%s", id, snapshotValue(r.Inputs["path"])),
					fmt.Sprintf("%s  dataJson=%s", id, snapshotValue(r.Inputs["dataJson"])))
			}
		}
	}
	sort.Strings(lines)
	return lines
}

func snapshotMap(id string, v resource.PropertyValue) []string {
	v = unwrapSecret(v)
	if !v.IsObject() {
		return []string{fmt.Sprintf("%s  <%s>", id, snapshotValue(v))}
	}
	var lines []string
	for k, val := range v.ObjectValue() {
		lines = append(lines, fmt.Sprintf("%s  %s=%s", id, k, snapshotValue(val)))
	}
	return lines
}

func snapshotValue(v resource.PropertyValue) string {
	v = unwrapSecret(v)
	switch {
	case v.IsString():
		return v.StringValue()
	case v.IsComputed():
		return "<computed>"
	case v.IsNull():
		return "<null>"
	default:
		return v.String()
	}
}

func unwrapSecret(v resource.PropertyValue) resource.PropertyValue {
	for v.IsSecret() {
		v = v.SecretValue().Element
	}
	return v
}

func TestExportCompatAzureManagedDb(t *testing.T) {
	setAzureMssqlAuthEnv(t)
	t.Setenv(EnvVaultEnabled, "true")
	tenant := newTenant("tenant1", "dev")

	azureDb := newAzureManagedDb("compat-mi-db")
	azureDb.Namespace = "qa"
	azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
		{Name: "app_user", Roles: []string{"db_owner"}},
		{Name: "reporting_user", Roles: []string{"db_datareader"}},
	}
	azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{{
		Name:              "charisma-hcm",
		ResourceGroupName: "SQL_Managed_Instance",
		Location:          "westeurope",
		Roles:             []string{"db_datareader"},
	}}
	azureDb.Spec.Exports = []provisioningv1.AzureManagedDatabaseExportsSpec{
		{
			Domain:              "hr-admin",
			DbName:              configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database"),
			IdentityClientId:    configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityClientId"),
			IdentityPrincipalId: kubeSecretExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityPrincipalId"),
		},
		{
			Domain:   "origination",
			UserRef:  "app_user",
			Username: configMapExport("Username"),
			Password: vaultExport("Password"),
		},
		{
			Domain:   "reporting",
			DbName:   vaultExport("Database"),
			UserRef:  "reporting_user",
			Username: kubeSecretExport("Username"),
			Password: kubeSecretExport("Password"),
		},
	}

	capture := newResourceCaptureMocks()
	stubIdentityUserLookup(capture, "12/7")
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"azure-native:managedidentity:UserAssignedIdentity compat-mi-db-charisma-hcm-identity",
		"azure-native:sql:ManagedDatabase compat-mi-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-mi-db",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-mi-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__Database=",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-mi-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__IdentityClientId=11111111-1111-1111-1111-111111111111",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-mi-db",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-mi-db  Username=app_user_compat-mi-db_dev_tenant1",
		"kubernetes:core/v1:Secret hr-admin-tenant1-compat-mi-db",
		"kubernetes:core/v1:Secret hr-admin-tenant1-compat-mi-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__IdentityPrincipalId=22222222-2222-2222-2222-222222222222",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mi-db",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mi-db  Password=Test-Password-123!",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mi-db  Username=reporting_user_compat-mi-db_dev_tenant1",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-mi-db-app_user-role-db_owner",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-mi-db-charisma-hcm-identity-role-db_datareader",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-mi-db-reporting_user-role-db_datareader",
		"mssql:index/script:Script compat-mi-db-charisma-hcm-identity-user",
		"mssql:index/sqlLogin:SqlLogin compat-mi-db-app_user-login",
		"mssql:index/sqlLogin:SqlLogin compat-mi-db-reporting_user-login",
		"mssql:index/sqlUser:SqlUser compat-mi-db-app_user-user",
		"mssql:index/sqlUser:SqlUser compat-mi-db-reporting_user-user",
		"pulumi:providers:mssql compat-mi-db-mssql-provider",
		"random:index/randomPassword:RandomPassword compat-mi-db-app_user-login-password",
		"random:index/randomPassword:RandomPassword compat-mi-db-reporting_user-login-password",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mi-db",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mi-db  dataJson={\"Password\":\"Test-Password-123!\"}",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mi-db  path=dev/qa/origination/tenant1/compat-mi-db",
		"vault:generic/secret:Secret dev/qa/reporting/tenant1/compat-mi-db",
		"vault:generic/secret:Secret dev/qa/reporting/tenant1/compat-mi-db  dataJson={\"Database\":\"\"}",
		"vault:generic/secret:Secret dev/qa/reporting/tenant1/compat-mi-db  path=dev/qa/reporting/tenant1/compat-mi-db",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}

func TestExportCompatAzureDb(t *testing.T) {
	setAzureMssqlAuthEnv(t)
	t.Setenv(EnvVaultEnabled, "true")
	tenant := newTenant("tenant1", "dev")

	azureDb := newAzureDb("compat-azure-db")
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
	azureDb.Spec.Exports = []provisioningv1.AzureDatabaseExportsSpec{
		{
			Domain:              "hr-admin",
			DbName:              configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database"),
			IdentityRef:         "app_identity",
			IdentityClientId:    configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityClientId"),
			IdentityPrincipalId: vaultExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityPrincipalId"),
		},
		{
			Domain:   "origination",
			UserRef:  "app_user",
			Username: configMapExport("Username"),
			Password: vaultExport("Password"),
		},
		{
			Domain:   "reporting",
			UserRef:  "reporting_user",
			Username: kubeSecretExport("Username"),
			Password: kubeSecretExport("Password"),
		},
	}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"azure-native:managedidentity:UserAssignedIdentity compat-azure-db-app_identity-identity",
		"azure-native:sql:Database compat-azure-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-azure-db",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-azure-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__Database=",
		"kubernetes:core/v1:ConfigMap hr-admin-tenant1-compat-azure-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__IdentityClientId=11111111-1111-1111-1111-111111111111",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-azure-db",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-azure-db  Username=app_user",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-azure-db",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-azure-db  Password=Test-Password-123!",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-azure-db  Username=reporting_user",
		"mssql:index/azureadServicePrincipal:AzureadServicePrincipal compat-azure-db-app_identity-identity-user",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-azure-db-app_identity-identity-role-db_datareader",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-azure-db-app_user-role-db_owner",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-azure-db-reporting_user-role-db_datareader",
		"mssql:index/script:Script compat-azure-db-app_user-contained-user",
		"mssql:index/script:Script compat-azure-db-reporting_user-contained-user",
		"pulumi:providers:mssql compat-azure-db-mssql-provider",
		"random:index/randomPassword:RandomPassword compat-azure-db-app_user-contained-password",
		"random:index/randomPassword:RandomPassword compat-azure-db-reporting_user-contained-password",
		"vault:generic/secret:Secret dev/qa/hr-admin/tenant1/compat-azure-db",
		"vault:generic/secret:Secret dev/qa/hr-admin/tenant1/compat-azure-db  dataJson={\"MultiTenancy__Tenants__tenant1__ConnectionStrings__IdentityPrincipalId\":\"22222222-2222-2222-2222-222222222222\"}",
		"vault:generic/secret:Secret dev/qa/hr-admin/tenant1/compat-azure-db  path=dev/qa/hr-admin/tenant1/compat-azure-db",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-azure-db",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-azure-db  dataJson={\"Password\":\"Test-Password-123!\"}",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-azure-db  path=dev/qa/origination/tenant1/compat-azure-db",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}

func TestExportCompatMsSqlDb(t *testing.T) {
	t.Setenv(EnvVaultEnabled, "true")
	tenant := newTenant("tenant1", "dev")

	mssqlDb := newMsSqlDb("compat-mssql-db")
	mssqlDb.Namespace = "qa"
	mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{
		{Name: "app_user", Roles: []string{"db_owner"}},
		{Name: "reporting_user", Roles: []string{"db_datareader"}},
	}
	mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{
		{
			Domain:   "origination",
			DbName:   configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database"),
			UserRef:  "app_user",
			Username: configMapExport("Username"),
			Password: vaultExport("Password"),
		},
		{
			Domain:   "reporting",
			UserRef:  "reporting_user",
			Username: kubeSecretExport("Username"),
			Password: kubeSecretExport("Password"),
		},
	}

	capture := newResourceCaptureMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, nil, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	want := []string{
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-mssql-db",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-mssql-db  MultiTenancy__Tenants__tenant1__ConnectionStrings__Database=compat-mssql-db_dev_tenant1",
		"kubernetes:core/v1:ConfigMap origination-tenant1-compat-mssql-db  Username=app_user_compat-mssql-db_dev_tenant1",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mssql-db",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mssql-db  Password=Test-Password-123!",
		"kubernetes:core/v1:Secret reporting-tenant1-compat-mssql-db  Username=reporting_user_compat-mssql-db_dev_tenant1",
		"mssql:index/database:Database compat-mssql-db",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-mssql-db-app_user-role-db_owner",
		"mssql:index/databaseRoleMember:DatabaseRoleMember compat-mssql-db-reporting_user-role-db_datareader",
		"mssql:index/sqlLogin:SqlLogin compat-mssql-db-app_user-login",
		"mssql:index/sqlLogin:SqlLogin compat-mssql-db-reporting_user-login",
		"mssql:index/sqlUser:SqlUser compat-mssql-db-app_user-user",
		"mssql:index/sqlUser:SqlUser compat-mssql-db-reporting_user-user",
		"pulumi:providers:mssql provider-mssql",
		"random:index/randomPassword:RandomPassword compat-mssql-db-app_user-login-password",
		"random:index/randomPassword:RandomPassword compat-mssql-db-reporting_user-login-password",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mssql-db",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mssql-db  dataJson={\"Password\":\"Test-Password-123!\"}",
		"vault:generic/secret:Secret dev/qa/origination/tenant1/compat-mssql-db  path=dev/qa/origination/tenant1/compat-mssql-db",
	}
	assert.Equal(t, want, exportSnapshot(capture))
}
