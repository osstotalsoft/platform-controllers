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

// TestDeployContainedUserTracksRoleMembershipNotJustPresence guards against a regression where
// deployContainedUser's idempotency check tracked only whether the contained user existed
// ("Present"/"Absent"), not which roles it held. With that bug, adding a role to userSpec.Roles on
// a later apply (the same logical user, same defaultName) would leave the Script's desired State
// unchanged ("Present" == "Present"), so UpdateScript would never re-run and the new role would
// never be granted.
//
// The Pulumi Go testing mocks don't simulate real state-diffing across two sequential `pulumi up`
// runs for the same resource (there is no "previous state" to diff against — NewResource is always
// treated as a create). So instead this test deploys two independently-named Script resources
// (simulating "apply 1" and "apply 2" of the same conceptual user, just with different role sets)
// and asserts directly on the rendered State/UpdateScript content:
//   - the desired State value differs once a role is added, which is exactly the signal Pulumi's
//     real diff engine uses to decide whether ReadScript's output still matches and, if not, to run
//     UpdateScript on the next actual apply;
//   - UpdateScript for the second deployment contains a guarded ALTER ROLE for the newly added role.
func TestDeployContainedUserTracksRoleMembershipNotJustPresence(t *testing.T) {
	capture := newScriptCaptureMocks()

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := newTestProvider(t, ctx, "test-provider-4")

		// "Apply 1": user provisioned with a single role.
		_, _, err := deployContainedUser(ctx, provider, "contained-db-v1",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader"}},
			"contained-db", []pulumi.Resource{}, false)
		assert.NoError(t, err)

		// "Apply 2": same logical user (same defaultName), but a role was added to the spec.
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

	// Roles are sorted, so the desired state string is deterministic regardless of spec order.
	assert.Equal(t, "Present:db_datareader", v1State)
	assert.Equal(t, "Present:db_datareader,db_datawriter", v2State)
	assert.NotEqual(t, v1State, v2State,
		"adding a role must change the tracked desired State so a later apply's ReadScript/State comparison detects the drift")

	v2Update := v2Inputs["updateScript"].StringValue()
	assert.Contains(t, v2Update, "ALTER ROLE [db_datawriter] ADD MEMBER [contained-db];")
	assert.Contains(t, v2Update, "ALTER ROLE [db_datareader] ADD MEMBER [contained-db];")
	assert.Contains(t, v2Update, "IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = 'contained-db')",
		"CREATE USER must be idempotency-guarded so updateScript is safe to re-run against an already-existing user")

	// readScript's shape depends on the managed role set (see
	// TestDeployContainedUserRoleRemovalConverges below) — for the same-role-count check here, just
	// confirm the readScript actually restricts aggregation to the roles each deployment manages.
	assert.Contains(t, v1Inputs["readScript"].StringValue(), "r.name IN ('db_datareader')")
	assert.Contains(t, v2Inputs["readScript"].StringValue(), "r.name IN ('db_datareader','db_datawriter')")
}

// TestDeployContainedUserRoleRemovalConverges proves the fix for the idempotency gap where
// updateScript only ever emits guarded "ALTER ROLE ... ADD MEMBER" statements and never "DROP
// MEMBER" for a role removed from userSpec.Roles (or granted out-of-band). Without a fix, readScript
// would keep reporting the actual (superset) role membership forever, permanently mismatching the
// narrower desired State and forcing UpdateScript to re-run on every single apply without ever
// converging.
//
// The fix taken here is option (b) from the review: readScript's role-membership aggregation is
// scoped to exactly the roles this deployment manages (userSpec.Roles at the time of THIS apply),
// via a "r.name IN (...)" filter in the join — so a role that is no longer desired (or was never
// managed to begin with) is excluded from the aggregate and can never cause read/desired drift. This
// intentionally means role *removal* is not enforced against the database: the role membership
// itself is left untouched, only its tracking stops. That limitation is what this test asserts.
func TestDeployContainedUserRoleRemovalConverges(t *testing.T) {
	capture := newScriptCaptureMocks()

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := newTestProvider(t, ctx, "test-provider-5")

		// "Apply 1": user provisioned with two roles.
		_, _, err := deployContainedUser(ctx, provider, "contained-db-removal-v1",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader", "db_datawriter"}},
			"contained-db-removal", []pulumi.Resource{}, false)
		assert.NoError(t, err)

		// "Apply 2": same logical user, but db_datawriter was removed from the spec. In the real
		// database (not simulated by these mocks — NewResource never executes ReadScript/
		// UpdateScript), db_datawriter membership granted by apply 1 is still actually present,
		// since no DROP MEMBER is ever emitted.
		_, _, err = deployContainedUser(ctx, provider, "contained-db-removal-v2",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader"}},
			"contained-db-removal", []pulumi.Resource{}, false)
		assert.NoError(t, err)
		return nil
	}, pulumi.WithMocks("project", "stack", capture))
	assert.NoError(t, err)

	v2Inputs := capture.resourceInputs["contained-db-removal-v2-contained-user"]
	v2State := v2Inputs["state"].ObjectValue()["UserStatus"].StringValue()
	v2Read := v2Inputs["readScript"].StringValue()
	v2Update := v2Inputs["updateScript"].StringValue()

	// Desired State no longer mentions the removed role.
	assert.Equal(t, "Present:db_datareader", v2State)

	// readScript's role filter is scoped to ONLY the roles still being managed — db_datawriter is
	// excluded, so even though it is (per the scenario) still actually granted in the real database,
	// readScript will never surface it, and therefore will report exactly "Present:db_datareader" —
	// matching v2State exactly. This is what makes the Script converge (no drift) despite the
	// database still holding the stale db_datawriter grant.
	assert.Contains(t, v2Read, "r.name IN ('db_datareader')")
	assert.NotContains(t, v2Read, "db_datawriter",
		"readScript must not aggregate a role that is no longer in userSpec.Roles, or the removed role would cause permanent read/desired drift")

	// updateScript never attempts to DROP the removed role's membership — role removal is not
	// enforced against the database by this path, only its tracking stops (the documented
	// limitation).
	assert.NotContains(t, v2Update, "DROP MEMBER",
		"deployContainedUser does not enforce role removal — this asserts the documented limitation, not a requirement to add DROP MEMBER")
	assert.NotContains(t, v2Update, "db_datawriter")
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
		capture := newScriptCaptureMocks()
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider := newTestProvider(t, ctx, "test-provider")
			return deploySchemaPermissionGrants(ctx, provider, "retain-db",
				pulumi.String("1").ToStringOutput(), pulumi.String("1/2").ToStringOutput(),
				map[string][]string{"dbo": {"EXECUTE"}}, []pulumi.Resource{}, true)
		}, pulumi.WithMocks("project", "stack", capture))
		assert.NoError(t, err)
		assert.True(t, capture.retainOnDelete("retain-db-schema-dbo-permission-EXECUTE"))
	})
}
