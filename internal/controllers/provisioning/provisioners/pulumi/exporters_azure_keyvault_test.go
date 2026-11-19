package pulumi

import (
	"regexp"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

const azureKeyVaultSecret = "azure-native:keyvault:Secret"

func azureKeyVaultExport(key string) provisioningv1.ValueExport {
	return provisioningv1.ValueExport{ToAzureKeyVault: provisioningv1.AzureKeyVaultSecretTemplate{KeyTemplate: key}}
}

// setAzureKeyVaultConfig sets the stack configuration that createOrSelectStack sets for a platform with a vault, for
// pulumi.WithMocks("project", ...).
func setAzureKeyVaultConfig(t *testing.T) {
	t.Setenv("PULUMI_CONFIG", `{"`+azureKeyVaultNameConfigKey("project")+`":"kv-lsng-prod","`+
		azureKeyVaultResourceGroupNameConfigKey("project")+`":"rg-lsng-prod"}`)
}

var validAzureKeyVaultSecretName = regexp.MustCompile(`^[0-9A-Za-z-]{1,127}$`)

func TestAzureKeyVaultSecretName(t *testing.T) {
	// Pinned: changing the naming renames, i.e. replaces, every exported secret.
	tests := []struct {
		domain, objectPath, key, want string
	}{
		{
			"origination",
			"charismaonline.qa/qa-lsng/origination/acme/origination-db",
			"MultiTenancy__Tenants__ACME__ConnectionStrings__Password",
			"origination-MultiTenancy--Tenants--ACME--ConnectionStrings--Password-75590350",
		},
		{ // same key, other platform and namespace
			"origination",
			"charismaonline.prod/prod-lsng/origination/acme/origination-db",
			"MultiTenancy__Tenants__ACME__ConnectionStrings__Password",
			"origination-MultiTenancy--Tenants--ACME--ConnectionStrings--Password-91581e5a",
		},
		{ // same key, other domain
			"payments",
			"charismaonline.qa/qa-lsng/payments/acme/shared-db",
			"MultiTenancy__Tenants__ACME__ConnectionStrings__Password",
			"payments-MultiTenancy--Tenants--ACME--ConnectionStrings--Password-b2fb9a00",
		},
		{ // single underscores
			"origination",
			"charismaonline.qa/qa-lsng/origination/acme/charisma-client-vm",
			"MultiTenancy__Tenants__ACME__CharismaClient_Admin_Password",
			"origination-MultiTenancy--Tenants--ACME--CharismaClient-Admin-Password-11cf997b",
		},
		{ // the end of the key is cut to fit 127 characters
			"payments",
			"charismaonline.qa/qa-lsng/payments/acme-insurance-romania/payments-gateway",
			"MultiTenancy__Tenants__ACME_INSURANCE_ROMANIA__Integrations__PaymentGateway__Providers__RaiffeisenBank__ClientCertificate__Password",
			"payments-MultiTenancy--Tenants--ACME-INSURANCE-ROMANIA--Integrations--PaymentGateway--Providers--RaiffeisenBank--Clien-a5ccf449",
		},
	}
	for _, tt := range tests {
		got := azureKeyVaultSecretName(tt.domain, tt.objectPath, tt.key)
		assert.Equal(t, tt.want, got)
		assert.Regexp(t, validAzureKeyVaultSecretName, got)
	}

	t.Run("a key without allowed characters is named by the domain and the hash", func(t *testing.T) {
		assert.Regexp(t, `^origination-[0-9a-f]{8}$`, azureKeyVaultSecretName("origination", "dev/qa/origination/obj", "__.__"))
	})

	t.Run("characters Key Vault doesn't allow in the domain are replaced", func(t *testing.T) {
		assert.Regexp(t, `^hr-admin-v2-Password-[0-9a-f]{8}$`, azureKeyVaultSecretName("hr.admin_v2", "dev/qa/hr.admin_v2/obj", "Password"))
	})

	t.Run("a cut never ends the name part with a dash", func(t *testing.T) {
		prefix := "origination-" + strings.Repeat("a", 105)
		got := azureKeyVaultSecretName("origination", "dev/qa/origination/obj", strings.Repeat("a", 105)+"__b")
		assert.Regexp(t, validAzureKeyVaultSecretName, got)
		assert.NotContains(t, got, "--")
		assert.Regexp(t, `^`+prefix+`-[0-9a-f]{8}$`, got)
	})
}

func TestAzureKeyVaultConfig(t *testing.T) {
	keyVault := &platformv1.AzureKeyVaultSpec{Name: "kv-lsng-prod", ResourceGroupName: "rg-lsng-prod"}
	config := func(name, resourceGroupName string) map[string]auto.ConfigValue {
		return map[string]auto.ConfigValue{
			"dev:azureKeyVaultName":              {Value: name},
			"dev:azureKeyVaultResourceGroupName": {Value: resourceGroupName},
		}
	}

	assert.Equal(t, config("kv-lsng-prod", "rg-lsng-prod"), azureKeyVaultConfig("dev", true, keyVault))
	// always set, so that a vault removed from the platform is cleared from the stack
	assert.Equal(t, config("", ""), azureKeyVaultConfig("dev", false, keyVault), "Azure not enabled")
	assert.Equal(t, config("", ""), azureKeyVaultConfig("dev", true, nil), "no vault on the platform")
}

func newAzureKeyVaultMsSqlDb() *provisioningv1.MsSqlDatabase {
	mssqlDb := newMsSqlDb("akv-mssql-db")
	mssqlDb.Namespace = "qa"
	mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app_user", Roles: []string{"db_owner"}}}
	mssqlDb.Spec.Exports = []provisioningv1.MsSqlDatabaseExportsSpec{{
		Domain:   "origination",
		DbName:   configMapExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database"),
		Username: azureKeyVaultExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Username"),
		Password: azureKeyVaultExport("MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password"),
	}}
	return mssqlDb
}

func capturedAzureKeyVaultSecrets(capture *resourceCaptureMocks) map[string]pulumi.MockResourceArgs {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	secrets := map[string]pulumi.MockResourceArgs{}
	for _, r := range capture.byType[azureKeyVaultSecret] {
		secrets[r.Name] = r
	}
	return secrets
}

func stringMapInput(v resource.PropertyValue) map[string]string {
	m := map[string]string{}
	for k, val := range unwrapSecret(v).ObjectValue() {
		m[string(k)] = snapshotValue(val)
	}
	return m
}

func TestExportToAzureKeyVault(t *testing.T) {
	t.Run("tenant: one secret per value, tagged with where it is exported from", func(t *testing.T) {
		setAzureKeyVaultConfig(t)
		tenant := newTenant("tenant1", "dev")

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, newAzureKeyVaultMsSqlDb(), []pulumi.Resource{}, nil, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		secrets := capturedAzureKeyVaultSecrets(capture)
		assert.Len(t, secrets, 2)

		objectPath := "dev/qa/origination/tenant1/akv-mssql-db"
		for key, wantValue := range map[string]string{
			"MultiTenancy__Tenants__tenant1__ConnectionStrings__Username": "app_user_akv-mssql-db_dev_tenant1",
			"MultiTenancy__Tenants__tenant1__ConnectionStrings__Password": "Test-Password-123!",
		} {
			name := azureKeyVaultSecretName("origination", objectPath, key)
			secret, found := secrets[name]
			if !assert.True(t, found, "no secret %s for %s", name, key) {
				continue
			}
			assert.Equal(t, name, snapshotValue(secret.Inputs["secretName"]))
			assert.Equal(t, "kv-lsng-prod", snapshotValue(secret.Inputs["vaultName"]))
			assert.Equal(t, "rg-lsng-prod", snapshotValue(secret.Inputs["resourceGroupName"]))

			value := unwrapSecret(secret.Inputs["properties"]).ObjectValue()["value"]
			assert.True(t, value.ContainsSecrets(), "the value must be a Pulumi secret")
			assert.Equal(t, wantValue, snapshotValue(value))

			assert.Equal(t, map[string]string{
				"platform":  "dev",
				"namespace": "qa",
				"domain":    "origination",
				"tenant":    "tenant1",
				"object":    "akv-mssql-db",
				"configKey": key,
			}, stringMapInput(secret.Inputs["tags"]))
		}

		// values routed elsewhere still go there
		assert.Len(t, capture.byType["kubernetes:core/v1:ConfigMap"], 1)
	})

	t.Run("platform: no tenant in the tags or the name hash", func(t *testing.T) {
		setAzureKeyVaultConfig(t)
		platform := &platformv1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "dev"}}
		mssqlDb := newAzureKeyVaultMsSqlDb()
		// a platform's template context has no .Tenant
		mssqlDb.Spec.Exports[0].DbName = configMapExport("ConnectionStrings__Database")
		mssqlDb.Spec.Exports[0].Username = azureKeyVaultExport("ConnectionStrings__Username")
		mssqlDb.Spec.Exports[0].Password = azureKeyVaultExport("ConnectionStrings__Password")

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(platform, mssqlDb, []pulumi.Resource{}, nil, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		key := "ConnectionStrings__Password"
		secret, found := capturedAzureKeyVaultSecrets(capture)[azureKeyVaultSecretName("origination", "dev/qa/origination/akv-mssql-db", key)]
		if assert.True(t, found) {
			tags := stringMapInput(secret.Inputs["tags"])
			assert.NotContains(t, tags, "tenant")
			assert.Equal(t, "dev", tags["platform"])
			assert.Equal(t, key, tags["configKey"])
		}
	})

	t.Run("fails when the platform has no vault", func(t *testing.T) {
		t.Setenv("PULUMI_CONFIG", `{"`+azureKeyVaultNameConfigKey("project")+`":"","`+
			azureKeyVaultResourceGroupNameConfigKey("project")+`":""}`)
		tenant := newTenant("tenant1", "dev")

		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, newAzureKeyVaultMsSqlDb(), []pulumi.Resource{}, nil, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, `platform "dev" has no spec.azureKeyVault`)
		assert.Empty(t, capturedAzureKeyVaultSecrets(capture))
	})

	t.Run("fails when two values of an export have the same key", func(t *testing.T) {
		setAzureKeyVaultConfig(t)
		tenant := newTenant("tenant1", "dev")
		mssqlDb := newAzureKeyVaultMsSqlDb()
		mssqlDb.Spec.Exports[0].Username = azureKeyVaultExport("Credentials")
		mssqlDb.Spec.Exports[0].Password = azureKeyVaultExport("Credentials")

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, nil, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", newResourceCaptureMocks()))
		assert.ErrorContains(t, err, `are both exported to the Azure Key Vault key "Credentials"`)
	})
}
