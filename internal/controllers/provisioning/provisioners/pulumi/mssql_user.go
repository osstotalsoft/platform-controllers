package pulumi

import (
	"fmt"

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

	roleGrants := ""
	for _, role := range userSpec.Roles {
		roleGrants += fmt.Sprintf("ALTER ROLE [%s] ADD MEMBER [%s];\n", role, username)
	}

	script, err := mssql.NewScript(ctx, fmt.Sprintf("%s-contained-user", resourceNamePrefix), &mssql.ScriptArgs{
		DatabaseId: databaseId,
		ReadScript: pulumi.String(fmt.Sprintf(
			"SELECT CASE WHEN EXISTS (SELECT 1 FROM sys.database_principals WHERE name = '%s') THEN 'Present' ELSE 'Absent' END AS [UserStatus]",
			username)),
		UpdateScript: password.ApplyT(func(p string) string {
			return fmt.Sprintf("CREATE USER [%s] WITH PASSWORD = '%s';\n%s", username, p, roleGrants)
		}).(pulumi.StringOutput),
		DeleteScript: pulumi.String(fmt.Sprintf("DROP USER IF EXISTS [%s];", username)),
		State: pulumi.StringMap{
			"UserStatus": pulumi.String("Present"),
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
