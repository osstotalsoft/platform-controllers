package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// scriptCaptureMocks records each resource's resolved Inputs, keyed by the Pulumi resource name
// (rather than by TypeToken, as the shared `mocks` type would implicitly do), so a test can deploy
// more than one mssql.Script resource and inspect each one's ReadScript/UpdateScript/State
// independently.
type scriptCaptureMocks struct {
	resourceInputs map[string]resource.PropertyMap
	registerRPCs   map[string]pulumi.MockResourceArgs
}

func newScriptCaptureMocks() *scriptCaptureMocks {
	return &scriptCaptureMocks{
		resourceInputs: map[string]resource.PropertyMap{},
		registerRPCs:   map[string]pulumi.MockResourceArgs{},
	}
}

func (m *scriptCaptureMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.resourceInputs[args.Name] = args.Inputs
	m.registerRPCs[args.Name] = args
	return args.Name + "_id", args.Inputs, nil
}

func (m *scriptCaptureMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// retainOnDelete reports the RetainOnDelete flag the engine received for the resource registered
// under name (false if the resource wasn't captured or the flag wasn't set).
func (m *scriptCaptureMocks) retainOnDelete(name string) bool {
	args, ok := m.registerRPCs[name]
	if !ok || args.RegisterRPC == nil {
		return false
	}
	return args.RegisterRPC.GetRetainOnDelete()
}

func newTestProvider(t *testing.T, ctx *pulumi.Context, name string) *mssql.Provider {
	provider, err := mssql.NewProvider(ctx, name, &mssql.ProviderArgs{
		Hostname: pulumi.String("localhost"),
		Port:     pulumi.Int(1433),
		SqlAuth: &mssql.ProviderSqlAuthArgs{
			Username: pulumi.String("admin"),
			Password: pulumi.String("password"),
		},
	})
	assert.NoError(t, err)
	return provider
}

func TestDeployLoginUser(t *testing.T) {
	t.Run("creates login, user and role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")

			username, password, err := deployLoginUser(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Name: "app1", Roles: []string{"db_owner"}},
				"my-db", []pulumi.Resource{}, false)
			assert.NoError(t, err)
			assert.Equal(t, "app1_my-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	// The real login/user name combines userSpec.Name with tenantScope (the caller's tenant-scoped
	// dbName) — see deployLoginUser's doc comment. userSpec.Name alone stays the stable,
	// tenant-independent key used for users[].name uniqueness/exports[].userRef matching.
	t.Run("real name combines spec name with tenant scope", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")

			username, _, err := deployLoginUser(ctx, provider, "my-db-2",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Name: "custom-user"},
				"my-db-2", []pulumi.Resource{}, false)
			assert.NoError(t, err)
			assert.Equal(t, "custom-user_my-db-2", username)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("retainOnDelete propagates to login, user and role-grant resources", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-retain")

			_, _, err := deployLoginUser(ctx, provider, "retain-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"retain-db", []pulumi.Resource{}, true)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.True(t, capture.retainOnDelete("retain-db-login"))
		assert.True(t, capture.retainOnDelete("retain-db-user"))
		assert.True(t, capture.retainOnDelete("retain-db-role-db_owner"))
	})

	t.Run("retainOnDelete false leaves resources non-retained", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-noretain")

			_, _, err := deployLoginUser(ctx, provider, "noretain-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"noretain-db", []pulumi.Resource{}, false)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.False(t, capture.retainOnDelete("noretain-db-login"))
		assert.False(t, capture.retainOnDelete("noretain-db-user"))
		assert.False(t, capture.retainOnDelete("noretain-db-role-db_owner"))
	})
}

func TestDeployContainedUser(t *testing.T) {
	t.Run("creates contained user with role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-2")

			username, password, err := deployContainedUser(ctx, provider, "contained-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"contained-db", []pulumi.Resource{}, false)
			assert.NoError(t, err)
			assert.Equal(t, "contained-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("retainOnDelete propagates to the contained-user Script", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-2-retain")

			_, _, err := deployContainedUser(ctx, provider, "contained-db-retain",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"contained-db-retain", []pulumi.Resource{}, true)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.True(t, capture.retainOnDelete("contained-db-retain-contained-user"))
	})
}

// TestDeployContainedUserScriptTracksOnlyExistence guards against role membership creeping back into
// the contained-user Script's tracked State/ReadScript/UpdateScript. Roles are managed entirely
// through separate mssql.DatabaseRoleMember resources now (see TestDeployContainedUserRoleGrants), so
// the Script itself must track only whether the contained user exists — changing userSpec.Roles must
// not change the Script's desired State, or an unrelated role edit would spuriously redrive the
// Script's UpdateScript (which only creates the user) on every apply.
func TestDeployContainedUserScriptTracksOnlyExistence(t *testing.T) {
	capture := newScriptCaptureMocks()

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := newTestProvider(t, ctx, "test-provider-4")

		_, _, err := deployContainedUser(ctx, provider, "contained-db-v1",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader"}},
			"contained-db", []pulumi.Resource{}, false)
		assert.NoError(t, err)

		_, _, err = deployContainedUser(ctx, provider, "contained-db-v2",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datawriter", "db_datareader"}},
			"contained-db", []pulumi.Resource{}, false)
		assert.NoError(t, err)
		return nil
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	v1Inputs := capture.resourceInputs["contained-db-v1-contained-user"]
	v2Inputs := capture.resourceInputs["contained-db-v2-contained-user"]

	v1State := v1Inputs["state"].ObjectValue()["UserStatus"].StringValue()
	v2State := v2Inputs["state"].ObjectValue()["UserStatus"].StringValue()

	assert.Equal(t, "Present", v1State)
	assert.Equal(t, "Present", v2State,
		"the Script's tracked State must not vary with userSpec.Roles — role changes are handled entirely by separate DatabaseRoleMember resources")

	v2Update := v2Inputs["updateScript"].StringValue()
	assert.Contains(t, v2Update, "IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = 'contained-db')",
		"CREATE USER must be idempotency-guarded so updateScript is safe to re-run against an already-existing user")
	assert.NotContains(t, v2Update, "ALTER ROLE",
		"role grants must not be embedded in the Script's UpdateScript anymore")

	assert.NotContains(t, v1Inputs["readScript"].StringValue(), "r.name IN",
		"readScript must not aggregate role membership anymore — it only reports user existence")
}

// TestDeployContainedUserRoleGrants proves role removal now actually revokes: roles are managed as
// individually-named mssql.DatabaseRoleMember resources (the same mechanism deployLoginUser and
// deployManagedIdentity already rely on), so a role dropped from userSpec.Roles is simply absent from
// the next apply's desired resource graph — which is what causes Pulumi's own engine to delete
// (revoke) it, exactly like removing a permission already does (see
// deployDatabasePermissionGrants's doc comment).
func TestDeployContainedUserRoleGrants(t *testing.T) {
	t.Run("no roles is a no-op", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployContainedUser(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{}, "app1", []pulumi.Resource{}, false)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/databaseRoleMember:DatabaseRoleMember"))
	})

	t.Run("grants each named role as a typed resource", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployContainedUser(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader", "db_datawriter"}},
				"app1", []pulumi.Resource{}, false)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		grants := capture.byType["mssql:index/databaseRoleMember:DatabaseRoleMember"]
		assert.Len(t, grants, 2)
		_, hasReader := capture.byName["my-db-role-db_datareader"]
		_, hasWriter := capture.byName["my-db-role-db_datawriter"]
		assert.True(t, hasReader)
		assert.True(t, hasWriter)
	})

	t.Run("retainOnDelete propagates to role-grant resources", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployContainedUser(ctx, provider, "contained-db-retain",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"contained-db-retain", []pulumi.Resource{}, true)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.True(t, capture.retainOnDelete("contained-db-retain-role-db_owner"))
	})

	// The Pulumi Go testing mocks don't simulate real state-diffing across two sequential `pulumi up`
	// runs for the same resource (there is no "previous state" to diff against — NewResource is
	// always treated as a create; see the older tests this replaced for the same caveat). So this
	// proves the underlying mechanism instead: a role dropped from userSpec.Roles is not part of the
	// resources this deployment registers at all — which is exactly the condition that makes Pulumi's
	// engine delete a same-named resource left over from a prior apply.
	t.Run("role removed from spec is absent from the next apply's resource graph", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")

			// "Apply 1": two roles.
			_, _, err := deployContainedUser(ctx, provider, "contained-db-removal-v1",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader", "db_datawriter"}},
				"contained-db-removal", []pulumi.Resource{}, false)
			assert.NoError(t, err)

			// "Apply 2": db_datawriter removed from the spec.
			_, _, err = deployContainedUser(ctx, provider, "contained-db-removal-v2",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader"}},
				"contained-db-removal", []pulumi.Resource{}, false)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		_, v2HasReader := capture.byName["contained-db-removal-v2-role-db_datareader"]
		_, v2HasWriter := capture.byName["contained-db-removal-v2-role-db_datawriter"]
		assert.True(t, v2HasReader)
		assert.False(t, v2HasWriter,
			"a role removed from userSpec.Roles must not be registered on the next apply, or the real resource left over from the prior apply would never be deleted/revoked")
	})
}

func TestDeployManagedIdentity(t *testing.T) {
	t.Run("creates identity, wires it in, grants roles", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-3")

			clientId, principalId, err := deployManagedIdentity(ctx, provider, "my-db-identity",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.ManagedIdentitySpec{
					ResourceGroupName: "my-rg",
					Location:          "westeurope",
					Roles:             []string{"db_owner"},
				},
				"my-db", []pulumi.Resource{}, false)
			assert.NoError(t, err)
			assert.NotNil(t, clientId)
			assert.NotNil(t, principalId)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("retainOnDelete propagates to identity, service-principal-user and role-grant resources", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-3-retain")

			_, _, err := deployManagedIdentity(ctx, provider, "my-db-identity-retain",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.ManagedIdentitySpec{
					ResourceGroupName: "my-rg",
					Location:          "westeurope",
					Roles:             []string{"db_owner"},
				},
				"my-db-retain", []pulumi.Resource{}, true)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.True(t, capture.retainOnDelete("my-db-identity-retain-identity"))
		assert.True(t, capture.retainOnDelete("my-db-identity-retain-identity-user"))
		assert.True(t, capture.retainOnDelete("my-db-identity-retain-identity-role-db_owner"))
	})
}

func TestValidateUniqueNames(t *testing.T) {
	t.Run("empty list is valid", func(t *testing.T) {
		err := validateUniqueNames([]provisioningv1.DatabaseUserSpec{}, "users")
		assert.NoError(t, err)
	})

	t.Run("unique names are valid", func(t *testing.T) {
		err := validateUniqueNames([]provisioningv1.DatabaseUserSpec{
			{Name: "app1"},
			{Name: "app2"},
		}, "users")
		assert.NoError(t, err)
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		err := validateUniqueNames([]provisioningv1.DatabaseUserSpec{{Name: ""}}, "users")
		assert.ErrorContains(t, err, "spec.users[].name is required")
	})

	t.Run("duplicate name is rejected", func(t *testing.T) {
		err := validateUniqueNames([]provisioningv1.DatabaseUserSpec{
			{Name: "app1"},
			{Name: "app1"},
		}, "users")
		assert.ErrorContains(t, err, `spec.users[].name "app1" is duplicated`)
	})

	t.Run("works against ManagedIdentitySpec too", func(t *testing.T) {
		err := validateUniqueNames([]provisioningv1.ManagedIdentitySpec{
			{Name: "id1"},
			{Name: "id1"},
		}, "managedIdentities")
		assert.ErrorContains(t, err, `spec.managedIdentities[].name "id1" is duplicated`)
	})
}

func TestResolveRef(t *testing.T) {
	byName := map[string]string{"app1": "value1"}

	t.Run("empty ref defaults to the sole entry", func(t *testing.T) {
		v, err := resolveRef(byName, "", "myDomain", "userRef", "users")
		assert.NoError(t, err)
		assert.Equal(t, "value1", v)
	})

	t.Run("explicit ref resolves by name", func(t *testing.T) {
		v, err := resolveRef(byName, "app1", "myDomain", "userRef", "users")
		assert.NoError(t, err)
		assert.Equal(t, "value1", v)
	})

	t.Run("unknown ref is an error", func(t *testing.T) {
		_, err := resolveRef(byName, "nope", "myDomain", "userRef", "users")
		assert.ErrorContains(t, err, `userRef "nope" does not match any spec.users[].name`)
	})

	t.Run("empty ref is ambiguous with more than one entry", func(t *testing.T) {
		multi := map[string]string{"app1": "value1", "app2": "value2"}
		_, err := resolveRef(multi, "", "myDomain", "userRef", "users")
		assert.ErrorContains(t, err, "userRef is required when spec.users does not have exactly one entry")
	})

	t.Run("empty ref is an error with zero entries", func(t *testing.T) {
		_, err := resolveRef(map[string]string{}, "", "myDomain", "userRef", "users")
		assert.ErrorContains(t, err, "userRef is required when spec.users does not have exactly one entry")
	})
}

func TestDeployDatabasePermissionGrants(t *testing.T) {
	t.Run("no permissions is a no-op", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deployDatabasePermissionGrants(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				nil, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/databasePermission:DatabasePermission"))
	})

	t.Run("grants each named permission", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deployDatabasePermissionGrants(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				[]string{"EXECUTE", "SELECT"}, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		grants := capture.byType["mssql:index/databasePermission:DatabasePermission"]
		assert.Len(t, grants, 2)
		permissions := map[string]bool{}
		for _, g := range grants {
			permissions[g.Inputs["permission"].StringValue()] = true
			assert.Equal(t, "1/2", g.Inputs["principalId"].StringValue())
		}
		assert.True(t, permissions["EXECUTE"])
		assert.True(t, permissions["SELECT"])
	})

	t.Run("retainOnDelete propagates", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deployDatabasePermissionGrants(ctx, provider, "retain-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				[]string{"EXECUTE"}, []pulumi.Resource{}, true)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.True(t, capture.retainOnDelete("retain-db-permission-EXECUTE"))
	})

	t.Run("sanitizes multi-word permission names for the Pulumi resource name, not the actual GRANT", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deployDatabasePermissionGrants(ctx, provider, "my-db-multiword",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				[]string{"VIEW DEFINITION"}, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		inputs, ok := capture.resourceInputs["my-db-multiword-permission-VIEW-DEFINITION"]
		assert.True(t, ok, "expected a resource named with hyphens in place of the space")
		assert.Equal(t, "VIEW DEFINITION", inputs["permission"].StringValue(), "the actual GRANT permission string must stay unsanitized")
	})
}

func TestDeploySchemaPermissionGrants(t *testing.T) {
	t.Run("no schema permissions is a no-op", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				nil, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.False(t, capture.hasAnyTypeWithPrefix("mssql:index/schemaPermission:SchemaPermission"))
	})

	t.Run("grants each named permission on each named schema", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{
			"id": resource.NewStringProperty("1/1"),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				map[string][]string{"dbo": {"EXECUTE"}}, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		grants := capture.byType["mssql:index/schemaPermission:SchemaPermission"]
		assert.Len(t, grants, 1)
		assert.Equal(t, "EXECUTE", grants[0].Inputs["permission"].StringValue())
		assert.Equal(t, "1/2", grants[0].Inputs["principalId"].StringValue())
	})

	t.Run("grants multiple permissions across multiple schemas", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{
			"id": resource.NewStringProperty("1/1"),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "my-db-2",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				map[string][]string{"dbo": {"EXECUTE", "SELECT"}, "reporting": {"SELECT"}},
				[]pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		grants := capture.byType["mssql:index/schemaPermission:SchemaPermission"]
		assert.Len(t, grants, 3)
	})

	t.Run("retainOnDelete propagates", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{
			"id": resource.NewStringProperty("1/1"),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "retain-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				map[string][]string{"dbo": {"EXECUTE"}}, []pulumi.Resource{}, true)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.True(t, capture.retainOnDelete("retain-db-schema-dbo-permission-EXECUTE"))
	})

	t.Run("clear error when the schema doesn't exist", func(t *testing.T) {
		capture := newResourceCaptureMocks()
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{
			"id": resource.NewStringProperty(""),
		})
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				map[string][]string{"nonexistent_schema": {"EXECUTE"}}, []pulumi.Resource{}, false)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.ErrorContains(t, err, `schema "nonexistent_schema" not found`)
	})
}

// orderingCheckMocks wraps resourceCaptureMocks to additionally assert, on the getSqlUser invoke,
// that the contained-user Script has already been registered by the time the invoke fires — the
// regression this guards against is the lookup racing ahead of user creation (see gateOn's doc
// comment in mssql_user.go for why the invoke-level DependsOn option can't be relied on for this).
type orderingCheckMocks struct {
	*resourceCaptureMocks
	t *testing.T
}

func (m *orderingCheckMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == "mssql:index/getSqlUser:getSqlUser" {
		assert.True(m.t, m.hasAnyTypeWithPrefix("mssql:index/script:Script"),
			"getSqlUser was invoked before the contained-user Script was registered — the lookup is racing ahead of user creation")
	}
	return m.resourceCaptureMocks.Call(args)
}

func TestDeployContainedUserPermissionLookupWaitsForScript(t *testing.T) {
	mocks := &orderingCheckMocks{resourceCaptureMocks: newResourceCaptureMocks(), t: t}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := newTestProvider(t, ctx, "test-provider")
		_, _, err := deployContainedUser(ctx, provider, "my-db",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Name: "app1", Permissions: []string{"EXECUTE"}},
			"app1", []pulumi.Resource{}, false)
		return err
	}, pulumi.WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}
