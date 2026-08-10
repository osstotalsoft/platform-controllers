package pulumi

import (
	"fmt"
	"os"
	"sort"
	"strconv"
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

// newMssqlAzureAuthProvider creates an mssql.Provider connected to hostname via Azure AD
// authentication, for the two Azure-native database kinds (AzureDatabase, AzureManagedDatabase).
//
// The provider resource is named "<resourceNamePrefix>-mssql-provider" — resourceNamePrefix must be
// the owning CR's name — so two such providers deployed in the same domain/stack (e.g. two
// AzureDatabases, or one AzureDatabase plus one AzureManagedDatabase, both with a User or
// ManagedIdentity configured) never collide on Pulumi resource name/URN.
//
// Auth mode mirrors how pulumi.go's createOrSelectStack already switches the azure-native/azuread
// providers between Workload Identity and client-secret modes (see EnvAzureUseWorkloadIdentity):
//   - Workload Identity enabled: pulumi-mssql's Provider selects AAD authentication mode based on
//     whether the AzureAuth block is present at all — not on what's inside it (all three of
//     ProviderAzureAuthArgs' fields are optional). So AzureAuth is set to an empty, non-nil
//     &mssql.ProviderAzureAuthArgs{}: its mere presence is what makes the provider fall back to its
//     default Azure credential chain (see github.com/microsoft/go-mssqldb's azuread driver), which
//     does support Workload Identity federation. Leaving AzureAuth nil/omitted would mean neither
//     azureAuth nor sqlAuth is set, and the provider would have no auth mode selected at all.
//   - Workload Identity disabled: the ambient AZURE_CLIENT_ID/AZURE_CLIENT_SECRET/AZURE_TENANT_ID
//     service-principal env vars (already relied upon elsewhere for the AAD Admin service principal)
//     are required and wired into AzureAuth. Fails fast with a clear error — instead of silently
//     connecting with empty credentials — if any of the three is missing, matching pulumi.go's
//     existing fail-fast pattern for incomplete Workload Identity configuration.
func newMssqlAzureAuthProvider(ctx *pulumi.Context, resourceNamePrefix string, hostname string) (*mssql.Provider, error) {
	providerArgs := &mssql.ProviderArgs{
		Hostname: pulumi.String(hostname),
	}

	useWorkloadIdentity := false
	if v := os.Getenv(EnvAzureUseWorkloadIdentity); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", EnvAzureUseWorkloadIdentity, err)
		}
		useWorkloadIdentity = parsed
	}

	if !useWorkloadIdentity {
		clientId := os.Getenv("AZURE_CLIENT_ID")
		clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
		tenantId := os.Getenv("AZURE_TENANT_ID")
		if clientId == "" || clientSecret == "" || tenantId == "" {
			return nil, fmt.Errorf(
				"AZURE_CLIENT_ID, AZURE_CLIENT_SECRET and AZURE_TENANT_ID must all be set to authenticate the mssql provider when %s is not enabled",
				EnvAzureUseWorkloadIdentity)
		}
		providerArgs.AzureAuth = &mssql.ProviderAzureAuthArgs{
			ClientId:     pulumi.String(clientId),
			ClientSecret: pulumi.String(clientSecret),
			TenantId:     pulumi.String(tenantId),
		}
	} else {
		// Workload Identity: set AzureAuth to an empty, non-nil struct. The provider selects AAD
		// auth mode based on the block's presence, not its contents (see the doc comment above), so
		// this — not a nil/omitted AzureAuth — is what triggers its default Azure credential chain
		// fallback.
		providerArgs.AzureAuth = &mssql.ProviderAzureAuthArgs{}
	}

	return mssql.NewProvider(ctx, fmt.Sprintf("%s-mssql-provider", resourceNamePrefix), providerArgs)
}

// deployDatabaseRoleGrants grants each named role, inside the database identified by databaseId,
// to the principal whose provider-assigned resource ID is memberId (already in the
// "<databaseId>/<principalId>" composite form the mssql provider expects — e.g. a SqlUser's or
// AzureadServicePrincipal's own .ID()). No-op if roles is empty.
func deployDatabaseRoleGrants(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string,
	databaseId pulumi.StringInput, memberId pulumi.StringInput, roles []string,
	dependencies []pulumi.Resource, retainOnDelete bool) error {

	for _, role := range roles {
		roleLookup := mssql.LookupDatabaseRoleOutput(ctx, mssql.LookupDatabaseRoleOutputArgs{
			DatabaseId: databaseId,
			Name:       pulumi.String(role),
		}, pulumi.Provider(provider))
		roleId := roleLookup.ApplyT(func(r mssql.LookupDatabaseRoleResult) string { return r.Id }).(pulumi.StringOutput)

		_, err := mssql.NewDatabaseRoleMember(ctx, fmt.Sprintf("%s-role-%s", resourceNamePrefix, role), &mssql.DatabaseRoleMemberArgs{
			RoleId:   roleId,
			MemberId: memberId,
		}, pulumi.Provider(provider), pulumi.DependsOn(dependencies), pulumi.RetainOnDelete(retainOnDelete))
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
	dependencies []pulumi.Resource, retainOnDelete bool) (string, pulumi.StringOutput, error) {

	username := resolveName(userSpec.Name, defaultName)

	password, err := newRandomPassword(ctx, fmt.Sprintf("%s-login", resourceNamePrefix))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	login, err := mssql.NewSqlLogin(ctx, fmt.Sprintf("%s-login", resourceNamePrefix), &mssql.SqlLoginArgs{
		Name:     pulumi.String(username),
		Password: password,
	}, pulumi.Provider(provider), pulumi.DependsOn(dependencies), pulumi.RetainOnDelete(retainOnDelete))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	user, err := mssql.NewSqlUser(ctx, fmt.Sprintf("%s-user", resourceNamePrefix), &mssql.SqlUserArgs{
		DatabaseId: databaseId,
		LoginId:    login.ID().ToStringOutput(),
		Name:       pulumi.String(username),
	}, pulumi.Provider(provider), pulumi.DependsOn(append(dependencies, login)), pulumi.RetainOnDelete(retainOnDelete))
	if err != nil {
		return "", pulumi.StringOutput{}, err
	}

	err = deployDatabaseRoleGrants(ctx, provider, resourceNamePrefix, databaseId, user.ID().ToStringOutput(), userSpec.Roles,
		append(dependencies, user), retainOnDelete)
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
	dependencies []pulumi.Resource, retainOnDelete bool) (string, pulumi.StringOutput, error) {

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
	// exist, otherwise "Present" optionally followed by a sorted, comma-joined list of the
	// *managed* roles (see roleFilter below) the user currently belongs to. Unlike a plain
	// "Present"/"Absent" flag, this ties the tracked state to the actual role membership, so
	// adding a role to userSpec.Roles changes the desired value and is detected as a mismatch
	// against readScript's output on the next apply, which triggers updateScript to reconcile it
	// (mirroring the ownerLoginName/set-db-owner pattern in mssql_db.go, where the tracked state IS
	// the configuration that matters).
	desiredState := "Present"
	if len(sortedRoles) > 0 {
		desiredState = "Present:" + strings.Join(sortedRoles, ",")
	}

	// roleFilter restricts readScript's role-membership aggregation to exactly the roles this
	// deployment manages (sortedRoles). This is a deliberate design choice, not an oversight:
	//
	// Role *removal* from the spec is intentionally NOT enforced by this Script — updateScript only
	// ever ADDs role memberships, it never DROPs one. If readScript reported ALL of the user's actual
	// role memberships (as it used to), then a role granted out-of-band (or previously granted by an
	// older spec and since removed from userSpec.Roles) would make readScript's output a permanent
	// superset of desiredState. That mismatch would never resolve — since updateScript still
	// wouldn't drop it — so the Script would show drift and re-run UpdateScript on every single
	// `pulumi up`, forever, and could surface as a hard "provider produced inconsistent result after
	// apply" error from the Terraform-bridged provider.
	//
	// By scoping the aggregation to only the roles currently in userSpec.Roles, an out-of-band or
	// previously-removed role is silently excluded from readScript's output, so it can never cause
	// read/desired drift: the Script converges once the roles in sortedRoles are granted, and stays
	// converged even if the actual principal still holds additional roles this deployment doesn't
	// (and, for a removed role, no longer does) manage. If a role is removed from userSpec.Roles,
	// the membership itself is left untouched in the database — it is simply no longer tracked.
	roleFilter := "1 = 0" // no roles managed: never match any actual role-membership row
	if len(sortedRoles) > 0 {
		quotedRoles := make([]string, len(sortedRoles))
		for i, role := range sortedRoles {
			quotedRoles[i] = fmt.Sprintf("'%s'", role)
		}
		roleFilter = fmt.Sprintf("r.name IN (%s)", strings.Join(quotedRoles, ","))
	}

	// readScript reports "Absent" if the contained user doesn't exist, otherwise "Present" plus the
	// sorted, comma-joined list of *managed* roles (roleFilter) currently granted to it (STRING_AGG
	// ... WITHIN GROUP is available on both Azure SQL Database and Azure SQL Managed Instance). The
	// LEFT JOINs plus GROUP BY on the user's principal_id ensure a row is still produced (with an
	// empty role list) when the user exists but has no managed role memberships. The role filter is
	// applied inside the second LEFT JOIN's ON clause (not a WHERE clause) so that a role membership
	// row for an unmanaged role still keeps the underlying drm/user row alive (producing a NULL role
	// name, excluded by STRING_AGG) instead of removing the row entirely and breaking the
	// "user exists but has zero managed roles" case.
	readScript := fmt.Sprintf(`
SELECT ISNULL(
	(SELECT 'Present' + ISNULL(':' + STRING_AGG(r.name, ',') WITHIN GROUP (ORDER BY r.name), '')
	 FROM sys.database_principals u
	 LEFT JOIN sys.database_role_members drm ON drm.member_principal_id = u.principal_id
	 LEFT JOIN sys.database_principals r ON r.principal_id = drm.role_principal_id AND %s
	 WHERE u.name = '%s'
	 GROUP BY u.principal_id),
	'Absent') AS [UserStatus]`, roleFilter, username)

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
	}, pulumi.Provider(provider), pulumi.DependsOn(dependencies), pulumi.RetainOnDelete(retainOnDelete))
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
	dependencies []pulumi.Resource, retainOnDelete bool) (pulumi.StringOutput, pulumi.StringOutput, error) {

	name := resolveName(identitySpec.Name, defaultName)

	identity, err := managedidentity.NewUserAssignedIdentity(ctx, fmt.Sprintf("%s-identity", resourceNamePrefix), &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String(name),
		ResourceGroupName: pulumi.String(identitySpec.ResourceGroupName),
		Location:          pulumi.String(identitySpec.Location),
	}, pulumi.DependsOn(dependencies), pulumi.RetainOnDelete(retainOnDelete))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	principal, err := mssql.NewAzureadServicePrincipal(ctx, fmt.Sprintf("%s-identity-user", resourceNamePrefix), &mssql.AzureadServicePrincipalArgs{
		DatabaseId: databaseId,
		ClientId:   identity.ClientId,
		Name:       pulumi.String(name),
	}, pulumi.Provider(provider), pulumi.DependsOn(append(dependencies, identity)), pulumi.RetainOnDelete(retainOnDelete))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	err = deployDatabaseRoleGrants(ctx, provider, resourceNamePrefix+"-identity", databaseId, principal.ID().ToStringOutput(),
		identitySpec.Roles, append(dependencies, principal), retainOnDelete)
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	return identity.ClientId, identity.PrincipalId, nil
}
