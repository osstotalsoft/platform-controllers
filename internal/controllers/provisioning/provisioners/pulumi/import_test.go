package pulumi

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func metaWithImport(importSetting *bool) *provisioningv1.ProvisioningMeta {
	return &provisioningv1.ProvisioningMeta{Import: importSetting}
}

func TestNewImportOptions(t *testing.T) {
	yes, no := true, false

	t.Run("reads the global setting", func(t *testing.T) {
		for value, expected := range map[string]bool{"true": true, "1": true, "false": false, "": false, "nope": false} {
			t.Setenv(EnvPulumiImportAll, value)
			options := newImportOptions(newTenant("tenant1", "dev"), &provisioning.InfrastructureManifests{})
			assert.Equal(t, expected, options.enabled, "%s=%q", EnvPulumiImportAll, value)
			assert.Equal(t, expected, options.stack.anyEnabled, "%s=%q", EnvPulumiImportAll, value)
		}
	})

	t.Run("loads the stack's state when a stateful resource opts in while the global setting is off", func(t *testing.T) {
		t.Setenv(EnvPulumiImportAll, "false")
		infra := &provisioning.InfrastructureManifests{MinioBuckets: []*provisioningv1.MinioBucket{
			{Spec: provisioningv1.MinioBucketSpec{ProvisioningMeta: *metaWithImport(&yes)}},
		}}
		options := newImportOptions(newTenant("tenant1", "dev"), infra)
		assert.False(t, options.enabled)
		assert.True(t, options.stack.anyEnabled)
	})

	t.Run("doesn't load the stack's state for resource kinds that don't import", func(t *testing.T) {
		t.Setenv(EnvPulumiImportAll, "false")
		infra := &provisioning.InfrastructureManifests{
			KeycloakClients: []*provisioningv1.KeycloakClient{{Spec: provisioningv1.KeycloakClientSpec{ProvisioningMeta: *metaWithImport(&yes)}}},
			MinioBuckets:    []*provisioningv1.MinioBucket{{Spec: provisioningv1.MinioBucketSpec{ProvisioningMeta: *metaWithImport(&no)}}},
		}
		assert.False(t, newImportOptions(newTenant("tenant1", "dev"), infra).stack.anyEnabled)
	})

	t.Run("a tenant's own setting overrides the global one", func(t *testing.T) {
		optingIn := newTenant("tenant1", "dev")
		optingIn.Spec.Import = &yes
		optingOut := newTenant("tenant2", "dev")
		optingOut.Spec.Import = &no

		t.Setenv(EnvPulumiImportAll, "false")
		options := newImportOptions(optingIn, &provisioning.InfrastructureManifests{})
		assert.True(t, options.enabled)
		assert.True(t, options.stack.anyEnabled, "the opting-in tenant's stack state must be loaded")
		assert.False(t, newImportOptions(newTenant("tenant3", "dev"), &provisioning.InfrastructureManifests{}).enabled,
			"other tenants keep the global setting")

		t.Setenv(EnvPulumiImportAll, "true")
		assert.False(t, newImportOptions(optingOut, &provisioning.InfrastructureManifests{}).enabled)
	})

	t.Run("a resource's own setting overrides its tenant's", func(t *testing.T) {
		t.Setenv(EnvPulumiImportAll, "false")
		tenant := newTenant("tenant1", "dev")
		tenant.Spec.Import = &yes

		options := newImportOptions(tenant, &provisioning.InfrastructureManifests{})
		assert.False(t, options.forResource(metaWithImport(&no)).enabled)
		assert.True(t, options.forResource(metaWithImport(nil)).enabled)
	})

	t.Run("platform stacks follow the global setting", func(t *testing.T) {
		platform := &platformv1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "dev"}}
		t.Setenv(EnvPulumiImportAll, "true")
		assert.True(t, newImportOptions(platform, &provisioning.InfrastructureManifests{}).enabled)
		t.Setenv(EnvPulumiImportAll, "false")
		assert.False(t, newImportOptions(platform, &provisioning.InfrastructureManifests{}).enabled)
	})
}

func TestImportOptionsForResource(t *testing.T) {
	yes, no := true, false

	assert.True(t, importsWith(false).forResource(metaWithImport(&yes)).enabled)
	assert.False(t, importsWith(true).forResource(metaWithImport(&no)).enabled)
	assert.True(t, importsWith(true).forResource(metaWithImport(nil)).enabled)
	assert.False(t, importsWith(false).forResource(metaWithImport(nil)).enabled)

	var nilOptions *importOptions
	assert.Nil(t, nilOptions.forResource(metaWithImport(&yes)))

	parent := importsWith(false, managedResourceKey(minioBucketType, "managed-bucket"))
	assert.False(t, parent.forResource(metaWithImport(&yes)).shouldImport(minioBucketType, "managed-bucket"),
		"a resource's own setting must still skip what the stack already manages")
}

func TestImportOptionsShouldImport(t *testing.T) {
	t.Run("never imports when disabled or nil", func(t *testing.T) {
		var nilOptions *importOptions
		assert.False(t, nilOptions.shouldImport(minioBucketType, "bucket"))
		assert.False(t, importsWith(false).shouldImport(minioBucketType, "bucket"))
	})

	t.Run("imports only resources the stack doesn't manage yet", func(t *testing.T) {
		options := importAll(managedResourceKey(minioBucketType, "managed-bucket"))
		assert.False(t, options.shouldImport(minioBucketType, "managed-bucket"))
		assert.True(t, options.shouldImport(minioBucketType, "new-bucket"))
		assert.True(t, options.shouldImport(mssqlDatabaseType, "managed-bucket"), "keys must include the type")
	})

	t.Run("a resource managed under one of its aliases isn't imported", func(t *testing.T) {
		options := importAll(managedResourceKey(azureSqlDatabaseType, "db_dev_tenant.1"))
		assert.False(t, options.shouldImport(azureSqlDatabaseType, "db_dev_tenant_1", "db_dev_tenant.1"))
		assert.True(t, options.shouldImport(azureSqlDatabaseType, "db_dev_tenant_1"))
	})
}

func TestManagedResources(t *testing.T) {
	t.Run("keys resources by their own type and name, ignoring the parent chain", func(t *testing.T) {
		deployment, err := json.Marshal(map[string]any{
			"resources": []map[string]any{
				{"urn": "urn:pulumi:tenant1-domain::dev::pulumi:pulumi:Stack::dev-tenant1-domain", "type": "pulumi:pulumi:Stack"},
				{"urn": "urn:pulumi:tenant1-domain::dev::mssql:index/database:Database::my-db", "type": mssqlDatabaseType},
				{
					"urn":  "urn:pulumi:tenant1-domain::dev::mssql:index/database:Database$mssql:index/sqlLogin:SqlLogin::my-db-app-login",
					"type": mssqlSqlLoginType,
				},
			},
		})
		assert.NoError(t, err)

		managed, err := managedResources(apitype.UntypedDeployment{Version: 3, Deployment: deployment})
		assert.NoError(t, err)
		assert.True(t, managed[managedResourceKey(mssqlDatabaseType, "my-db")])
		assert.True(t, managed[managedResourceKey(mssqlSqlLoginType, "my-db-app-login")])
		assert.Len(t, managed, 3)
	})

	t.Run("a stack without state manages nothing", func(t *testing.T) {
		managed, err := managedResources(apitype.UntypedDeployment{})
		assert.NoError(t, err)
		assert.Empty(t, managed)
	})
}

func TestNeedsPostImportUpdate(t *testing.T) {
	var nilOptions *importOptions
	assert.False(t, nilOptions.needsPostImportUpdate())

	options := importAll()
	options.importResource(pulumi.ID("id"))
	assert.False(t, options.needsPostImportUpdate(), "an import without ignored inputs needs no second update")

	options.importResource(pulumi.ID("id"), "password")
	assert.True(t, options.needsPostImportUpdate())
	assert.False(t, options.needsPostImportUpdate(), "it must reset, so the post-import update doesn't trigger another one")
}
