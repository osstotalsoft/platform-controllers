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
}

func newScriptCaptureMocks() *scriptCaptureMocks {
	return &scriptCaptureMocks{resourceInputs: map[string]resource.PropertyMap{}}
}

func (m *scriptCaptureMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.resourceInputs[args.Name] = args.Inputs
	return args.Name + "_id", args.Inputs, nil
}

func (m *scriptCaptureMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestDeployLoginUser(t *testing.T) {
	t.Run("creates login, user and role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, password, err := deployLoginUser(ctx, provider, "my-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"my-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "my-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("explicit name overrides default", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, _, err := deployLoginUser(ctx, provider, "my-db-2",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Name: "custom-user"},
				"my-db-2", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "custom-user", username)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func TestDeployContainedUser(t *testing.T) {
	t.Run("creates contained user with role grants", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider-2", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			username, password, err := deployContainedUser(ctx, provider, "contained-db",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}},
				"contained-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.Equal(t, "contained-db", username)
			assert.NotNil(t, password)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
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
		provider, err := mssql.NewProvider(ctx, "test-provider-4", &mssql.ProviderArgs{
			Hostname: pulumi.String("localhost"),
			Port:     pulumi.Int(1433),
			SqlAuth: &mssql.ProviderSqlAuthArgs{
				Username: pulumi.String("admin"),
				Password: pulumi.String("password"),
			},
		})
		assert.NoError(t, err)

		// "Apply 1": user provisioned with a single role.
		_, _, err = deployContainedUser(ctx, provider, "contained-db-v1",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datareader"}},
			"contained-db", []pulumi.Resource{})
		assert.NoError(t, err)

		// "Apply 2": same logical user (same defaultName), but a role was added to the spec.
		_, _, err = deployContainedUser(ctx, provider, "contained-db-v2",
			pulumi.String("1").ToStringOutput(),
			&provisioningv1.DatabaseUserSpec{Roles: []string{"db_datawriter", "db_datareader"}},
			"contained-db", []pulumi.Resource{})
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

	// readScript's shape must not itself depend on the role list (only on username) — it derives
	// the actual role membership from the database at apply time.
	assert.Equal(t, v1Inputs["readScript"].StringValue(), v2Inputs["readScript"].StringValue())

	v2Update := v2Inputs["updateScript"].StringValue()
	assert.Contains(t, v2Update, "ALTER ROLE [db_datawriter] ADD MEMBER [contained-db];")
	assert.Contains(t, v2Update, "ALTER ROLE [db_datareader] ADD MEMBER [contained-db];")
	assert.Contains(t, v2Update, "IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = 'contained-db')",
		"CREATE USER must be idempotency-guarded so updateScript is safe to re-run against an already-existing user")
}

func TestDeployManagedIdentity(t *testing.T) {
	t.Run("creates identity, wires it in, grants roles", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			provider, err := mssql.NewProvider(ctx, "test-provider-3", &mssql.ProviderArgs{
				Hostname: pulumi.String("localhost"),
				Port:     pulumi.Int(1433),
				SqlAuth: &mssql.ProviderSqlAuthArgs{
					Username: pulumi.String("admin"),
					Password: pulumi.String("password"),
				},
			})
			assert.NoError(t, err)

			clientId, principalId, err := deployManagedIdentity(ctx, provider, "my-db-identity",
				pulumi.String("1").ToStringOutput(),
				&provisioningv1.ManagedIdentitySpec{
					ResourceGroupName: "my-rg",
					Location:          "westeurope",
					Roles:             []string{"db_owner"},
				},
				"my-db", []pulumi.Resource{})
			assert.NoError(t, err)
			assert.NotNil(t, clientId)
			assert.NotNil(t, principalId)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
