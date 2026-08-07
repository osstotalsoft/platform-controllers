package pulumi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/managedidentity/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// resolveName returns spec's name if set, otherwise defaultName.
func resolveName(name, defaultName string) string {
	if name != "" {
		return name
	}
	return defaultName
}

// deployDatabaseRoleGrants grants each named role, inside the database identified by databaseId,
// to the principal whose provider-assigned resource ID is memberId (already in the
// "<databaseId>/<principalId>" composite form the mssql provider expects — e.g. a SqlUser's or
// AzureadServicePrincipal's own .ID()). No-op if roles is empty.
func deployDatabaseRoleGrants(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string,
	databaseId pulumi.StringInput, memberId pulumi.StringInput, roles []string,
	dependencies []pulumi.Resource) error {

	for _, role := range roles {
		roleLookup := mssql.LookupDatabaseRoleOutput(ctx, mssql.LookupDatabaseRoleOutputArgs{
			DatabaseId: databaseId,
			Name:       pulumi.String(role),
		}, pulumi.Provider(provider))
		roleId := roleLookup.ApplyT(func(r mssql.LookupDatabaseRoleResult) string { return r.Id }).(pulumi.StringOutput)

		_, err := mssql.NewDatabaseRoleMember(ctx, fmt.Sprintf("%s-role-%s", resourceNamePrefix, role), &mssql.DatabaseRoleMemberArgs{
			RoleId:   roleId,
			MemberId: memberId,
		}, pulumi.Provider(provider), pulumi.DependsOn(dependencies))
		if err != nil {
			return err
		}
	}
	return nil
}

// deployLoginUser creates a server-level SqlLogin (generated password) plus a database-scoped
// SqlUser mapped to it, and grants userSpec.Roles. Used when a server login is available (sqlmi,
// on-prem MsSqlDatabase).
func deployLoginUser(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string,
	databaseId pulumi.StringInput, userSpec *provisioningv1.DatabaseUserSpec, defaultName string,
	dependencies []pulumi.Resource) (string, pulumi.StringOutput, error) {

	username := resolveName(userSpec.Name, defaultName)

	password, err := newRandomPassword(ctx, fmt.Sprintf("%s-login", resourceNamePrefix))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	login, err := mssql.NewSqlLogin(ctx, fmt.Sprintf("%s-login", resourceNamePrefix), &mssql.SqlLoginArgs{
		Name:     pulumi.String(username),
		Password: password,
	}, pulumi.Provider(provider), pulumi.DependsOn(dependencies))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	user, err := mssql.NewSqlUser(ctx, fmt.Sprintf("%s-user", resourceNamePrefix), &mssql.SqlUserArgs{
		DatabaseId: databaseId,
		LoginId:    login.ID().ToStringOutput(),
		Name:       pulumi.String(username),
	}, pulumi.Provider(provider), pulumi.DependsOn(append(dependencies, login)))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	err = deployDatabaseRoleGrants(ctx, provider, resourceNamePrefix, databaseId, user.ID().ToStringOutput(), userSpec.Roles,
		append(dependencies, user))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	return username, password, nil
}

// deployContainedUser creates a password-based contained database user (no server-level login) via
// an idempotent Script, and grants userSpec.Roles. Used when there is no server-login concept
// (AzureDatabase) or the caller opted into contained mode (AzureManagedDatabase.ContainedUser).
func deployContainedUser(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string,
	databaseId pulumi.StringInput, userSpec *provisioningv1.DatabaseUserSpec, defaultName string,
	dependencies []pulumi.Resource) (string, pulumi.StringOutput, error) {

	username := resolveName(userSpec.Name, defaultName)

	password, err := newRandomPassword(ctx, fmt.Sprintf("%s-contained", resourceNamePrefix))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	// Sort roles so the desired/actual state strings are order-independent (a spec that lists the
	// same roles in a different order must not be seen as a change).
	sortedRoles := append([]string{}, userSpec.Roles...)
	sort.Strings(sortedRoles)

	// desiredState mirrors what readScript computes from the DB: "Absent" if the user doesn't
	// exist, otherwise "Present" optionally followed by a sorted, comma-joined list of the roles
	// the user currently belongs to. Unlike a plain "Present"/"Absent" flag, this ties the tracked
	// state to the actual role membership, so adding/removing a role in userSpec.Roles changes the
	// desired value and is detected as a mismatch against readScript's output on the next apply,
	// which triggers updateScript to reconcile it (mirroring the ownerLoginName/set-db-owner
	// pattern in mssql_db.go, where the tracked state IS the configuration that matters).
	desiredState := "Present"
	if len(sortedRoles) > 0 {
		desiredState = "Present:" + strings.Join(sortedRoles, ",")
	}

	// readScript reports "Absent" if the contained user doesn't exist, otherwise "Present" plus
	// the sorted, comma-joined list of roles currently granted to it (STRING_AGG ... WITHIN GROUP
	// is available on both Azure SQL Database and Azure SQL Managed Instance). The LEFT JOINs plus
	// GROUP BY on the user's principal_id ensure a row is still produced (with an empty role list)
	// when the user exists but has no role memberships.
	readScript := fmt.Sprintf(`
SELECT ISNULL(
	(SELECT 'Present' + ISNULL(':' + STRING_AGG(r.name, ',') WITHIN GROUP (ORDER BY r.name), '')
	 FROM sys.database_principals u
	 LEFT JOIN sys.database_role_members drm ON drm.member_principal_id = u.principal_id
	 LEFT JOIN sys.database_principals r ON r.principal_id = drm.role_principal_id
	 WHERE u.name = '%s'
	 GROUP BY u.principal_id),
	'Absent') AS [UserStatus]`, username)

	// Each ALTER ROLE statement is itself guarded by a membership check, so updateScript is safe to
	// re-run both when creating the user for the first time and when reconciling roles onto an
	// already-existing user (the only case that changes UserStatus without CREATE USER running).
	roleGrants := ""
	for _, role := range sortedRoles {
		roleGrants += fmt.Sprintf(`IF NOT EXISTS (
	SELECT 1 FROM sys.database_role_members drm
	JOIN sys.database_principals r ON r.principal_id = drm.role_principal_id
	JOIN sys.database_principals m ON m.principal_id = drm.member_principal_id
	WHERE r.name = '%s' AND m.name = '%s')
	ALTER ROLE [%s] ADD MEMBER [%s];
`, role, username, role, username)
	}

	script, err := mssql.NewScript(ctx, fmt.Sprintf("%s-contained-user", resourceNamePrefix), &mssql.ScriptArgs{
		DatabaseId: databaseId,
		ReadScript: pulumi.String(readScript),
		UpdateScript: password.ApplyT(func(p string) string {
			return fmt.Sprintf(
				"IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = '%s')\n\tCREATE USER [%s] WITH PASSWORD = '%s';\n%s",
				username, username, p, roleGrants)
		}).(pulumi.StringOutput),
		DeleteScript: pulumi.String(fmt.Sprintf("DROP USER IF EXISTS [%s];", username)),
		State: pulumi.StringMap{
			"UserStatus": pulumi.String(desiredState),
		},
	}, pulumi.Provider(provider), pulumi.DependsOn(dependencies))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}
	_ = script

	return username, password, nil
}

// deployManagedIdentity creates an Azure user-assigned managed identity and wires it into the
// database as a contained AAD service-principal user, granting identitySpec.Roles.
func deployManagedIdentity(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string,
	databaseId pulumi.StringInput, identitySpec *provisioningv1.ManagedIdentitySpec, defaultName string,
	dependencies []pulumi.Resource) (pulumi.StringOutput, pulumi.StringOutput, error) {

	name := resolveName(identitySpec.Name, defaultName)

	identity, err := managedidentity.NewUserAssignedIdentity(ctx, fmt.Sprintf("%s-identity", resourceNamePrefix), &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String(name),
		ResourceGroupName: pulumi.String(identitySpec.ResourceGroupName),
		Location:          pulumi.String(identitySpec.Location),
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	principal, err := mssql.NewAzureadServicePrincipal(ctx, fmt.Sprintf("%s-identity-user", resourceNamePrefix), &mssql.AzureadServicePrincipalArgs{
		DatabaseId: databaseId,
		ClientId:   identity.ClientId,
		Name:       pulumi.String(name),
	}, pulumi.Provider(provider), pulumi.DependsOn(append(dependencies, identity)))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	err = deployDatabaseRoleGrants(ctx, provider, resourceNamePrefix+"-identity", databaseId, principal.ID().ToStringOutput(),
		identitySpec.Roles, append(dependencies, principal))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	return identity.ClientId, identity.PrincipalId, nil
}
