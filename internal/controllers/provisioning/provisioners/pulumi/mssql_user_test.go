package pulumi

import (
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"github.com/stretchr/testify/assert"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
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
				"my-db", []pulumi.Resource{}, false, nil)
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
				"my-db-2", []pulumi.Resource{}, false, nil)
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
				"retain-db", []pulumi.Resource{}, true, nil)
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
				"noretain-db", []pulumi.Resource{}, false, nil)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		assert.False(t, capture.retainOnDelete("noretain-db-login"))
		assert.False(t, capture.retainOnDelete("noretain-db-user"))
		assert.False(t, capture.retainOnDelete("noretain-db-role-db_owner"))
	})
}

// TestDeployLoginUserSessionsGuard checks the Script deployLoginUser pairs with the login so that
// DROP LOGIN doesn't fail with "Could not drop login ... as the user is currently logged in" when an
// application is still connected: Pulumi deletes the Script, which depends on the login, first, and
// its DeleteScript disables the login and kills its sessions.
func TestDeployLoginUserSessionsGuard(t *testing.T) {
	deploy := func(t *testing.T, retainOnDelete bool) *scriptCaptureMocks {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployLoginUser(ctx, provider, "my-db-app1", pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Name: "app1", Roles: []string{"db_owner"}},
				"my-db", []pulumi.Resource{}, retainOnDelete, nil)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		return capture
	}

	t.Run("delete disables the login and kills its sessions", func(t *testing.T) {
		capture := deploy(t, false)
		inputs, ok := capture.resourceInputs["my-db-app1-login-sessions"]
		if !assert.True(t, ok, "the login's sessions guard Script must be deployed") {
			return
		}
		deleteScript := inputs["deleteScript"].StringValue()
		assert.Contains(t, deleteScript, "ALTER LOGIN [app1_my-db] DISABLE")
		assert.Contains(t, deleteScript, "FROM sys.dm_exec_sessions WHERE login_name = N'app1_my-db'")
		assert.Contains(t, deleteScript, "KILL")
		assert.Contains(t, inputs["updateScript"].StringValue(), "ALTER LOGIN [app1_my-db] ENABLE")
		assert.Contains(t, inputs["readScript"].StringValue(), "N'app1_my-db'")
		assert.Equal(t, "Enabled", inputs["state"].ObjectValue()["LoginStatus"].StringValue())
	})

	t.Run("is deleted before the login and before being replaced", func(t *testing.T) {
		capture := deploy(t, false)
		rpc := capture.registerRPCs["my-db-app1-login-sessions"].RegisterRPC
		if !assert.NotNil(t, rpc) {
			return
		}
		assert.Contains(t, rpc.GetDependencies(), "urn:pulumi:stack::project::mssql:index/sqlLogin:SqlLogin::my-db-app1-login")
		assert.True(t, rpc.GetDeleteBeforeReplace(),
			"a replacement must disable the login before the new Script enables it again, not after")
	})

	t.Run("retainOnDelete propagates", func(t *testing.T) {
		assert.True(t, deploy(t, true).retainOnDelete("my-db-app1-login-sessions"))
		assert.False(t, deploy(t, false).retainOnDelete("my-db-app1-login-sessions"))
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

	// The provider runs UpdateScript when the Script is created, and on every update of its inputs.
	// So after a lost Pulumi state (Script created again, user already there) or a regenerated password
	// (UpdateScript's text changes), the user only gets the new password if UpdateScript sets it on an
	// existing user rather than skipping it.
	t.Run("UpdateScript sets the password of an existing user", func(t *testing.T) {
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider-2-password")

			_, _, err := deployContainedUser(ctx, provider, "contained-db-password",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{},
				"app1", []pulumi.Resource{}, false)
			return err
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)

		updateScript := capture.resourceInputs["contained-db-password-contained-user"]["updateScript"].StringValue()
		assert.Regexp(t, `(?s)IF NOT EXISTS \(SELECT 1 FROM sys.database_principals WHERE name = 'app1'\)\s+CREATE USER \[app1\] WITH PASSWORD = '[^']*';\s+ELSE\s+ALTER USER \[app1\] WITH PASSWORD = '[^']*';`,
			updateScript)
	})
}

// TestDeployContainedUserScriptTracksOnlyExistence guards against role membership creeping back into
// the contained-user Script's tracked State/ReadScript/UpdateScript. Roles are managed entirely
// through separate mssql.DatabaseRoleMember resources now (see TestDeployContainedUserRoleGrants), so
// the Script itself must track only whether the contained user exists — changing userSpec.Roles must
// not change the Script's desired State, or an unrelated role edit would spuriously redrive the
// Script's UpdateScript (which only creates the user or sets its password) on every apply.
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
				"my-db", "", identityUserBySid, []pulumi.Resource{}, false, nil)
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
				"my-db-retain", "", identityUserBySid, []pulumi.Resource{}, true, nil)
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

func valueKeys(values map[string]exportTemplateWithValue) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestAddUserExports(t *testing.T) {
	user := func(name string) deployedUser {
		return deployedUser{username: name, password: pulumi.String("pwd").ToStringOutput()}
	}
	one := map[string]deployedUser{"app": user("app")}
	two := map[string]deployedUser{"app": user("app"), "migrations": user("migrations")}

	t.Run("nothing exported adds nothing", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		assert.NoError(t, addUserExports(values, two, userExports{}, "d"))
		assert.Empty(t, values)
	})

	t.Run("single fields keep their value keys", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addUserExports(values, one, userExports{username: configMapExport("Username")}, "d")
		assert.NoError(t, err)
		assert.Equal(t, []string{"password", "username"}, valueKeys(values))
		assert.Equal(t, configMapExport("Username"), values["username"].valueExport)
	})

	t.Run("users items get indexed value keys", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addUserExports(values, two, userExports{users: []provisioningv1.ExportedUserSpec{
			{UserRef: "app", Username: configMapExport("Username")},
			{UserRef: "migrations", Password: vaultExport("MigrationPassword")},
		}}, "d")
		assert.NoError(t, err)
		assert.Equal(t, []string{"users[0].password", "users[0].username", "users[1].password", "users[1].username"}, valueKeys(values))
		assert.Equal(t, vaultExport("MigrationPassword"), values["users[1].password"].valueExport)
	})

	t.Run("single fields and users combine", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addUserExports(values, two, userExports{
			userRef: "app", username: configMapExport("Username"),
			users: []provisioningv1.ExportedUserSpec{{UserRef: "migrations", Username: configMapExport("MigrationUsername")}},
		}, "d")
		assert.NoError(t, err)
		assert.Equal(t, []string{"password", "username", "users[0].password", "users[0].username"}, valueKeys(values))
	})

	t.Run("empty userRef defaults to the sole user", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addUserExports(values, one, userExports{users: []provisioningv1.ExportedUserSpec{{Username: configMapExport("Username")}}}, "d")
		assert.NoError(t, err)
		assert.Contains(t, values, "users[0].username")
	})

	t.Run("empty userRef is ambiguous with more than one user", func(t *testing.T) {
		err := addUserExports(map[string]exportTemplateWithValue{}, two,
			userExports{users: []provisioningv1.ExportedUserSpec{{Username: configMapExport("Username")}}}, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: users[].userRef is required when spec.users does not have exactly one entry")
	})

	t.Run("unknown userRef", func(t *testing.T) {
		err := addUserExports(map[string]exportTemplateWithValue{}, two,
			userExports{users: []provisioningv1.ExportedUserSpec{{UserRef: "nope", Username: configMapExport("Username")}}}, "d")
		assert.ErrorContains(t, err, `exports[domain=d]: users[].userRef "nope" does not match any spec.users[].name`)
	})

	t.Run("users item without values", func(t *testing.T) {
		err := addUserExports(map[string]exportTemplateWithValue{}, two,
			userExports{users: []provisioningv1.ExportedUserSpec{{UserRef: "app"}}}, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: users[0] must export username or password")
	})

	t.Run("same user twice in users", func(t *testing.T) {
		err := addUserExports(map[string]exportTemplateWithValue{}, two, userExports{users: []provisioningv1.ExportedUserSpec{
			{UserRef: "app", Username: configMapExport("Username")},
			{UserRef: "app", Username: configMapExport("OtherUsername")},
		}}, "d")
		assert.ErrorContains(t, err, `exports[domain=d]: users[1] exports spec.users[].name "app", which this exports[] entry already exports`)
	})

	t.Run("same user in userRef and users, after defaulting", func(t *testing.T) {
		err := addUserExports(map[string]exportTemplateWithValue{}, one, userExports{
			username: configMapExport("Username"),
			users:    []provisioningv1.ExportedUserSpec{{UserRef: "app", Password: vaultExport("Password")}},
		}, "d")
		assert.ErrorContains(t, err, `users[0] exports spec.users[].name "app", which this exports[] entry already exports`)
	})
}

func TestAddIdentityExports(t *testing.T) {
	identity := deployedIdentity{
		clientId:    pulumi.String("client").ToStringOutput(),
		principalId: pulumi.String("principal").ToStringOutput(),
	}
	one := map[string]deployedIdentity{"charisma-online": identity}
	two := map[string]deployedIdentity{"charisma-online": identity, "charisma-online-migrations": identity}

	t.Run("nothing exported adds nothing", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		assert.NoError(t, addIdentityExports(values, two, identityExports{}, "d"))
		assert.Empty(t, values)
	})

	t.Run("single fields keep their value keys", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addIdentityExports(values, one, identityExports{identityClientId: configMapExport("IdentityClientId")}, "d")
		assert.NoError(t, err)
		assert.Equal(t, []string{"identityClientId", "identityPrincipalId"}, valueKeys(values))
	})

	t.Run("identities items get indexed value keys", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addIdentityExports(values, two, identityExports{identities: []provisioningv1.ExportedIdentitySpec{
			{IdentityRef: "charisma-online", IdentityClientId: configMapExport("IdentityClientId")},
			{IdentityRef: "charisma-online-migrations", IdentityClientId: configMapExport("MigrationClientId")},
		}}, "hr-admin")
		assert.NoError(t, err)
		assert.Equal(t, []string{
			"identities[0].identityClientId", "identities[0].identityPrincipalId",
			"identities[1].identityClientId", "identities[1].identityPrincipalId",
		}, valueKeys(values))
		assert.Equal(t, configMapExport("MigrationClientId"), values["identities[1].identityClientId"].valueExport)
	})

	t.Run("single fields and identities combine", func(t *testing.T) {
		values := map[string]exportTemplateWithValue{}
		err := addIdentityExports(values, two, identityExports{
			identityRef: "charisma-online", identityClientId: configMapExport("IdentityClientId"),
			identities: []provisioningv1.ExportedIdentitySpec{
				{IdentityRef: "charisma-online-migrations", IdentityClientId: configMapExport("MigrationClientId")},
			},
		}, "d")
		assert.NoError(t, err)
		assert.Len(t, values, 4)
	})

	t.Run("empty identityRef is ambiguous with more than one identity", func(t *testing.T) {
		err := addIdentityExports(map[string]exportTemplateWithValue{}, two, identityExports{identities: []provisioningv1.ExportedIdentitySpec{
			{IdentityClientId: configMapExport("IdentityClientId")},
		}}, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: identities[].identityRef is required when spec.managedIdentities does not have exactly one entry")
	})

	t.Run("unknown identityRef", func(t *testing.T) {
		err := addIdentityExports(map[string]exportTemplateWithValue{}, two, identityExports{identities: []provisioningv1.ExportedIdentitySpec{
			{IdentityRef: "nope", IdentityClientId: configMapExport("IdentityClientId")},
		}}, "d")
		assert.ErrorContains(t, err, `identities[].identityRef "nope" does not match any spec.managedIdentities[].name`)
	})

	t.Run("identities item without values", func(t *testing.T) {
		err := addIdentityExports(map[string]exportTemplateWithValue{}, two, identityExports{identities: []provisioningv1.ExportedIdentitySpec{
			{IdentityRef: "charisma-online"},
		}}, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: identities[0] must export identityClientId or identityPrincipalId")
	})

	t.Run("same identity in identityRef and identities", func(t *testing.T) {
		err := addIdentityExports(map[string]exportTemplateWithValue{}, two, identityExports{
			identityRef: "charisma-online", identityClientId: configMapExport("IdentityClientId"),
			identities: []provisioningv1.ExportedIdentitySpec{
				{IdentityRef: "charisma-online", IdentityPrincipalId: configMapExport("PrincipalId")},
			},
		}, "d")
		assert.ErrorContains(t, err, `identities[0] exports spec.managedIdentities[].name "charisma-online", which this exports[] entry already exports`)
	})
}

func TestValidateExportKeys(t *testing.T) {
	templateContext := provisioning.GetTemplateContext(newTenant("tenant1", "dev"))
	value := func(v provisioningv1.ValueExport) exportTemplateWithValue {
		return exportTemplateWithValue{v, pulumi.String("x")}
	}

	t.Run("distinct keys are valid", func(t *testing.T) {
		err := validateExportKeys(map[string]exportTemplateWithValue{
			"identities[0].identityClientId":    value(configMapExport("T__{{ .Tenant.Code }}__IdentityClientId")),
			"identities[1].identityClientId":    value(configMapExport("T__{{ .Tenant.Code }}__MigrationClientId")),
			"identities[0].identityPrincipalId": value(provisioningv1.ValueExport{}),
		}, templateContext, "hr-admin")
		assert.NoError(t, err)
	})

	t.Run("the same key in different destinations is valid", func(t *testing.T) {
		err := validateExportKeys(map[string]exportTemplateWithValue{
			"users[0].username": value(configMapExport("Username")),
			"users[0].password": value(kubeSecretExport("Username")),
			"users[1].password": value(vaultExport("Username")),
		}, templateContext, "d")
		assert.NoError(t, err)
	})

	for _, tt := range []struct {
		destination string
		export      func(string) provisioningv1.ValueExport
	}{
		{"toConfigMap", configMapExport},
		{"toKubeSecret", kubeSecretExport},
		{"toVault", vaultExport},
	} {
		t.Run("colliding rendered keys in "+tt.destination, func(t *testing.T) {
			err := validateExportKeys(map[string]exportTemplateWithValue{
				"identityClientId":               value(tt.export("T__{{ .Tenant.Code }}__ClientId")),
				"identities[0].identityClientId": value(tt.export("T__tenant1__ClientId")),
			}, templateContext, "hr-admin")
			assert.ErrorContains(t, err, `exports[domain=hr-admin]: identities[0].identityClientId and identityClientId both render the `+tt.destination+` key "T__tenant1__ClientId"`)
		})
	}

	t.Run("key rendering empty", func(t *testing.T) {
		err := validateExportKeys(map[string]exportTemplateWithValue{
			"users[0].username": value(configMapExport("{{ if false }}x{{ end }}")),
		}, templateContext, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: users[0].username.toConfigMap.keyTemplate renders an empty key")
	})

	t.Run("invalid template", func(t *testing.T) {
		err := validateExportKeys(map[string]exportTemplateWithValue{
			"users[0].username": value(configMapExport("{{ .Tenant.Code")),
		}, templateContext, "d")
		assert.ErrorContains(t, err, "exports[domain=d]: users[0].username.toConfigMap.keyTemplate:")
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

// identityLookupOrderingMocks asserts, on the getAzureadServicePrincipal invoke, that the identity's
// user Script has already been registered: the member id lookup must not race ahead of the user's
// creation (see gateOn's doc comment in mssql_user.go). The Script's registration is delayed, so an
// ungated lookup, whose args are already known, reliably runs first.
type identityLookupOrderingMocks struct {
	*resourceCaptureMocks
	t      *testing.T
	looked atomic.Bool
}

func (m *identityLookupOrderingMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	if args.TypeToken == "mssql:index/script:Script" {
		time.Sleep(200 * time.Millisecond)
	}
	return m.resourceCaptureMocks.NewResource(args)
}

func (m *identityLookupOrderingMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == "mssql:index/getAzureadServicePrincipal:getAzureadServicePrincipal" {
		m.looked.Store(true)
		assert.True(m.t, m.hasAnyTypeWithPrefix("mssql:index/script:Script"),
			"getAzureadServicePrincipal was invoked before the identity's user Script was registered — the lookup is racing ahead of user creation")
	}
	return m.resourceCaptureMocks.Call(args)
}

func TestDeployManagedIdentityFromExternalProvider(t *testing.T) {
	identitySpec := &provisioningv1.ManagedIdentitySpec{
		ResourceGroupName: "my-rg",
		Location:          "westeurope",
		Roles:             []string{"db_owner"},
		Permissions:       []string{"EXECUTE"},
		SchemaPermissions: map[string][]string{"dbo": {"SELECT"}},
	}
	deploy := func(t *testing.T, mocks pulumi.MockResourceMonitor, retainOnDelete bool) error {
		return pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployManagedIdentity(ctx, provider, "my-db-app", pulumi.String("1").ToStringOutput(),
				identitySpec, "my-db", "", identityUserFromExternalProvider, []pulumi.Resource{}, retainOnDelete, nil)
			return err
		}, pulumi.WithMocks("project", "stack", mocks))
	}
	stub := func(capture *resourceCaptureMocks) *resourceCaptureMocks {
		stubIdentityUserLookup(capture, "1/5")
		capture.stubCall("mssql:index/getSchema:getSchema", resource.PropertyMap{"id": resource.NewStringProperty("1/1")})
		return capture
	}

	t.Run("the member id lookup waits for the script", func(t *testing.T) {
		mocks := &identityLookupOrderingMocks{resourceCaptureMocks: stub(newResourceCaptureMocks()), t: t}
		assert.NoError(t, deploy(t, mocks, false))
		assert.True(t, mocks.looked.Load(), "the grants' member id must be looked up")
	})

	t.Run("no grants, no lookup", func(t *testing.T) {
		mocks := &identityLookupOrderingMocks{resourceCaptureMocks: newResourceCaptureMocks(), t: t}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			_, _, err := deployManagedIdentity(ctx, provider, "my-db-app", pulumi.String("1").ToStringOutput(),
				&provisioningv1.ManagedIdentitySpec{ResourceGroupName: "my-rg", Location: "westeurope"},
				"my-db", "", identityUserFromExternalProvider, []pulumi.Resource{}, false, nil)
			return err
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
		assert.Len(t, mocks.byType["mssql:index/script:Script"], 1)
		assert.False(t, mocks.looked.Load(), "without grants the member id isn't needed")
	})

	t.Run("retainOnDelete propagates to the identity, the script and the grants", func(t *testing.T) {
		capture := stub(newResourceCaptureMocks())
		assert.NoError(t, deploy(t, capture, true))
		for _, name := range []string{
			"my-db-app-identity",
			"my-db-app-identity-user",
			"my-db-app-identity-role-db_owner",
			"my-db-app-identity-permission-EXECUTE",
			"my-db-app-identity-schema-dbo-permission-SELECT",
		} {
			assert.True(t, capture.retainOnDelete(name), name)
		}
	})

	t.Run("retainOnDelete false leaves the script and the grants non-retained", func(t *testing.T) {
		capture := stub(newResourceCaptureMocks())
		assert.NoError(t, deploy(t, capture, false))
		assert.False(t, capture.retainOnDelete("my-db-app-identity-user"))
		assert.False(t, capture.retainOnDelete("my-db-app-identity-role-db_owner"))
	})
}

func TestSqlQuoting(t *testing.T) {
	tests := []struct {
		name, identifier, literal string
	}{
		{"app_my-db_dev_mf", "[app_my-db_dev_mf]", "N'app_my-db_dev_mf'"},
		{"a]b", "[a]]b]", "N'a]b'"},
		{"o'neil", "[o'neil]", "N'o''neil'"},
		{"x]'];DROP USER y;--", "[x]]']];DROP USER y;--]", "N'x]''];DROP USER y;--'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.identifier, sqlQuotedIdentifier(tt.name))
			assert.Equal(t, tt.literal, sqlStringLiteral(tt.name))
		})
	}
}
