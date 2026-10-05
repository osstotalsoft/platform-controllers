package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// importAll returns globally enabled importOptions under which only the given resources are
// already managed.
func importAll(managed ...string) *importOptions {
	return importsWith(true, managed...)
}

// importsWith returns importOptions with the given global setting, under which only the given
// resources are already managed.
func importsWith(enabled bool, managed ...string) *importOptions {
	options := &importOptions{enabled: enabled, stack: &stackImports{anyEnabled: true, managed: map[string]bool{}}}
	for _, key := range managed {
		options.stack.managed[key] = true
	}
	return options
}

// importId returns the import ID the engine received for the resource registered as name ("" when
// it isn't imported).
func (m *resourceCaptureMocks) importId(t *testing.T, name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	args, ok := m.byName[name]
	if !assert.True(t, ok, "resource %s was not registered", name) {
		return ""
	}
	return args.RegisterRPC.GetImportId()
}

// importIdOfType is importId for a name several resource types share.
func (m *resourceCaptureMocks) importIdOfType(t *testing.T, typeToken, name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, args := range m.byType[typeToken] {
		if args.Name == name {
			return args.RegisterRPC.GetImportId()
		}
	}
	assert.Fail(t, "resource not registered", "%s %s", typeToken, name)
	return ""
}

func (m *resourceCaptureMocks) ignoreChanges(name string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byName[name].RegisterRPC.GetIgnoreChanges()
}

func stubSubscription(capture *resourceCaptureMocks) {
	capture.stubCall("azure-native:authorization:getClientConfig", resource.PropertyMap{
		"subscriptionId": resource.NewStringProperty("sub-1"),
	})
}

func TestImportMinioBucket(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	newBucket := func() *provisioningv1.MinioBucket {
		return &provisioningv1.MinioBucket{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
			Spec: provisioningv1.MinioBucketSpec{
				BucketName:       "bucket",
				ProvisioningMeta: provisioningv1.ProvisioningMeta{DomainRef: "example-domain"},
			},
		}
	}
	run := func(t *testing.T, bucket *provisioningv1.MinioBucket, imports *importOptions) *resourceCaptureMocks {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMinioBucket(tenant, bucket, []pulumi.Resource{}, imports, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		return capture
	}

	t.Run("imports the bucket by its generated name", func(t *testing.T) {
		capture := run(t, newBucket(), importAll())
		assert.Equal(t, "bucket-dev-tenant1", capture.importId(t, "my-bucket"))
		assert.Contains(t, capture.ignoreChanges("my-bucket"), "forceDestroy")
	})

	t.Run("an explicit importBucketName wins", func(t *testing.T) {
		bucket := newBucket()
		bucket.Spec.ImportBucketName = "legacy-bucket"
		capture := run(t, bucket, importAll())
		assert.Equal(t, "legacy-bucket", capture.importId(t, "my-bucket"))
	})

	t.Run("a managed bucket isn't imported", func(t *testing.T) {
		capture := run(t, newBucket(), importAll(managedResourceKey(minioBucketType, "my-bucket")))
		assert.Empty(t, capture.importId(t, "my-bucket"))
		assert.NotContains(t, capture.ignoreChanges("my-bucket"), "forceDestroy")
	})

	t.Run("nothing is imported when import is disabled", func(t *testing.T) {
		capture := run(t, newBucket(), nil)
		assert.Empty(t, capture.importId(t, "my-bucket"))
	})
}

func TestImportAzureResourceGroup(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	run := func(t *testing.T, imports *importOptions) *resourceCaptureMocks {
		capture := newResourceCaptureMocks()
		stubSubscription(capture)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureRG(tenant, "domain", imports)(ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		return capture
	}

	t.Run("imports the resource group by its ARM ID", func(t *testing.T) {
		capture := run(t, importAll())
		assert.Equal(t, "/subscriptions/sub-1/resourceGroups/dev-tenant1-domain", capture.importId(t, "dev-tenant1-domain"))
	})

	t.Run("a managed resource group isn't imported", func(t *testing.T) {
		capture := run(t, importAll(managedResourceKey(azureResourceGroupType, "dev-tenant1-domain")))
		assert.Empty(t, capture.importId(t, "dev-tenant1-domain"))
	})
}

func TestImportAzureDb(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	const serverId = "/subscriptions/sub-1/resourceGroups/SQL_RG/providers/Microsoft.Sql/servers/testsvr"
	run := func(t *testing.T, azureDb *provisioningv1.AzureDatabase, imports *importOptions) *resourceCaptureMocks {
		setAzureMssqlAuthEnv(t)
		capture := newResourceCaptureMocks()
		stubSubscription(capture)
		capture.stubCall("azure-native:sql:getServer", resource.PropertyMap{
			"id":   resource.NewStringProperty(serverId),
			"name": resource.NewStringProperty("testsvr"),
		})
		capture.stubCall("mssql:index/getAzureadServicePrincipal:getAzureadServicePrincipal", resource.PropertyMap{
			"id": resource.NewStringProperty("12/7"),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, imports, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		return capture
	}

	t.Run("imports the database and its managed identity", func(t *testing.T) {
		azureDb := newAzureDb("my-db")
		azureDb.Spec.ManagedIdentities = []provisioningv1.ManagedIdentitySpec{{Name: "app", ResourceGroupName: "ID_RG", Location: "westeurope"}}

		imports := importAll()
		capture := run(t, azureDb, imports)
		assert.Equal(t, serverId+"/databases/my-db_dev_tenant1", capture.importId(t, "my-db_dev_tenant1"))
		assert.Equal(t, "/subscriptions/sub-1/resourceGroups/ID_RG/providers/Microsoft.ManagedIdentity/userAssignedIdentities/app_my-db_dev_tenant1",
			capture.importId(t, "my-db-app-identity"))
		assert.Equal(t, "12/7", capture.importId(t, "my-db-app-identity-user"))
		assert.False(t, imports.needsPostImportUpdate(), "nothing was imported with ignored inputs, so there's nothing for a second update to apply")
	})

	t.Run("a database managed under its pre-rename alias isn't imported", func(t *testing.T) {
		tenant := newTenant("tenant.1", "dev")
		azureDb := newAzureDb("my-db")
		capture := newResourceCaptureMocks()
		capture.stubCall("azure-native:sql:getServer", resource.PropertyMap{"id": resource.NewStringProperty(serverId)})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, importAll(managedResourceKey(azureSqlDatabaseType, "my-db_dev_tenant.1")), ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.Empty(t, capture.importId(t, "my-db_dev_tenant_1"))
	})

	t.Run("an explicit importDatabaseId wins", func(t *testing.T) {
		azureDb := newAzureDb("my-db")
		azureDb.Spec.ImportDatabaseId = "/explicit/id"
		capture := run(t, azureDb, importAll())
		assert.Equal(t, "/explicit/id", capture.importId(t, "my-db_dev_tenant1"))
	})
}

func TestImportAzureManagedDb(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	setAzureMssqlAuthEnv(t)

	azureDb := newAzureManagedDb("my-db")
	azureDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app"}}

	capture := newResourceCaptureMocks()
	stubSubscription(capture)
	capture.stubCall("azure-native:sql:getManagedInstance", resource.PropertyMap{
		"fullyQualifiedDomainName": resource.NewStringProperty("incubsqlmi.zone.database.windows.net"),
	})
	capture.stubCall("mssql:index/getSqlLogin:getSqlLogin", resource.PropertyMap{"id": resource.NewStringProperty("0xABC")})
	capture.stubCall("mssql:index/getSqlUser:getSqlUser", resource.PropertyMap{"id": resource.NewStringProperty("12/5")})

	imports := importAll()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, imports, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)
	assert.True(t, imports.needsPostImportUpdate(), "the login's new password must be applied by a post-import update")

	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/SQLMI_RG/providers/Microsoft.Sql/managedInstances/incubsqlmi/databases/my-db_dev_tenant1",
		capture.importId(t, "my-db_dev_tenant1"))
	assert.Equal(t, "0xABC", capture.importId(t, "my-db-app-login"))
	assert.Equal(t, []string{"password"}, capture.ignoreChanges("my-db-app-login"),
		"the regenerated password can't match the login's current one, so the import must ignore it")
	assert.Equal(t, "12/5", capture.importId(t, "my-db-app-user"))
}

func TestImportMsSqlDb(t *testing.T) {
	tenant := newTenant("tenant1", "dev")

	t.Run("imports the database, login and user", func(t *testing.T) {
		mssqlDb := newMsSqlDb("my-db")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app"}}

		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getDatabase:getDatabase", resource.PropertyMap{"id": resource.NewStringProperty("12")})
		capture.stubCall("mssql:index/getSqlLogin:getSqlLogin", resource.PropertyMap{"id": resource.NewStringProperty("0xABC")})
		capture.stubCall("mssql:index/getSqlUser:getSqlUser", resource.PropertyMap{"id": resource.NewStringProperty("12/5")})

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, importAll(), ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.Equal(t, "12", capture.importId(t, "my-db"))
		assert.Equal(t, "0xABC", capture.importId(t, "my-db-app-login"))
		assert.Equal(t, "12/5", capture.importId(t, "my-db-app-user"))
	})

	t.Run("fails when the database to import doesn't exist", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getDatabase:getDatabase", resource.PropertyMap{"id": resource.NewStringProperty("")})

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, newMsSqlDb("my-db"), []pulumi.Resource{}, importAll(), ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, "database my-db_dev_tenant1 to import not found")
	})

	t.Run("fails when the user to import doesn't exist", func(t *testing.T) {
		mssqlDb := newMsSqlDb("my-db")
		mssqlDb.Spec.Users = []provisioningv1.DatabaseUserSpec{{Name: "app"}}

		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getDatabase:getDatabase", resource.PropertyMap{"id": resource.NewStringProperty("12")})
		capture.stubCall("mssql:index/getSqlLogin:getSqlLogin", resource.PropertyMap{"id": resource.NewStringProperty("0xABC")})
		capture.stubCall("mssql:index/getSqlUser:getSqlUser", resource.PropertyMap{"id": resource.NewStringProperty("")})

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, importAll(), ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, "user app to import not found")
	})
}

func TestImportAzureVirtualDesktopGroups(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	rg := pulumi.String("my-rg").ToStringOutput()

	avd := newVirtualDesktop("my-avd", "dev")
	avd.Spec.Users.Admins = []string{"admin@contoso.com"}
	avd.Spec.Groups.Admins = []string{"child-group"}

	capture := newResourceCaptureMocks()
	// Every group lookup — the AVD's own groups and the child group — resolves to this one group,
	// whose only member is the admin user.
	capture.stubCall("azuread:index/getGroup:getGroup", resource.PropertyMap{
		"id":       resource.NewStringProperty("group-1"),
		"objectId": resource.NewStringProperty("group-1"),
		"members":  resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("user-1")}),
	})
	capture.stubCall("azuread:index/getUser:getUser", resource.PropertyMap{
		"id":       resource.NewStringProperty("user-1"),
		"objectId": resource.NewStringProperty("user-1"),
	})

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureVirtualDesktop(tenant, rg, avd, []pulumi.Resource{}, importAll(), ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	// The AVD's application group is named "test-vm-apps" too.
	assert.Equal(t, "group-1", capture.importIdOfType(t, azureadGroupType, "test-vm-apps"))
	assert.Equal(t, "group-1", capture.importId(t, "test-vm-admin"))
	assert.Equal(t, []string{"owners"}, capture.ignoreChanges("test-vm-admin"))
	assert.Equal(t, "group-1/member/user-1", capture.importId(t, "admin@contoso.com-admin-test-vm"),
		"an existing membership of an imported group must be imported, or creating it fails")
	assert.Empty(t, capture.importId(t, "child-group-admin-group-test-vm"),
		"a membership the imported group doesn't have yet must be created")
}

func TestImportResourceLevelOverride(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	newBucket := func(importSetting *bool) *provisioningv1.MinioBucket {
		return &provisioningv1.MinioBucket{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
			Spec: provisioningv1.MinioBucketSpec{
				BucketName:       "bucket",
				ProvisioningMeta: provisioningv1.ProvisioningMeta{DomainRef: "example-domain", Import: importSetting},
			},
		}
	}
	run := func(t *testing.T, bucket *provisioningv1.MinioBucket, imports *importOptions) *resourceCaptureMocks {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployResource(tenant, nil, bucket, []pulumi.Resource{}, imports, ctx)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		return capture
	}
	yes, no := true, false

	t.Run("a resource opting in is imported while the global setting is off", func(t *testing.T) {
		capture := run(t, newBucket(&yes), importsWith(false))
		assert.Equal(t, "bucket-dev-tenant1", capture.importId(t, "my-bucket"))
	})

	t.Run("a resource opting out isn't imported while the global setting is on", func(t *testing.T) {
		capture := run(t, newBucket(&no), importsWith(true))
		assert.Empty(t, capture.importId(t, "my-bucket"))
	})

	t.Run("a resource without its own setting follows the global one", func(t *testing.T) {
		assert.Equal(t, "bucket-dev-tenant1", run(t, newBucket(nil), importsWith(true)).importId(t, "my-bucket"))
		assert.Empty(t, run(t, newBucket(nil), importsWith(false)).importId(t, "my-bucket"))
	})

	t.Run("a resource opting in still isn't re-imported once managed", func(t *testing.T) {
		capture := run(t, newBucket(&yes), importsWith(false, managedResourceKey(minioBucketType, "my-bucket")))
		assert.Empty(t, capture.importId(t, "my-bucket"))
	})
}
