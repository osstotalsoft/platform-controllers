# Extend DB provisioning manifests with an optional user (+ Entra managed identity)

**Status:** Draft — awaiting review
**Branch:** `RDINC-5722-Platform-provisioner---extend-db-resources-with-optional-user`
**Scope:** `pkg/apis/provisioning/v1alpha1/*`, `internal/controllers/provisioning/provisioners/pulumi/*`, generated CRDs/clientset, `README.md`

## Context

Three CRDs provision databases: `AzureDatabase` (Azure SQL Database), `AzureManagedDatabase` (SQL Managed Instance), `MsSqlDatabase` (on-prem SQL Server). Today none of them create an app-facing database user — they only create the database itself and export its `dbName`. `MsSqlDatabase` has a `sqlServer.sqlAuth{username,password}` field, but that is the **admin** credential used by Pulumi to connect and provision, not an app user.

Goal: add an optional app-facing database user to all three, with credentials exportable (alongside the existing `dbName` export) so a consumer can assemble a connection string downstream. Additionally, for `AzureDatabase` and `AzureManagedDatabase` (the two Azure-native kinds), add an optional Entra ID (Azure AD) path: create a user-assigned managed identity and wire it into the database as a contained AAD user, exporting the identity's `clientId`/`principalId` for apps that authenticate via Workload Identity / `DefaultAzureCredential` instead of a SQL password.

**Non-goals:** rotating passwords, provisioning the AAD Admin on the SQL Server/Managed Instance (already configured, out of scope), Entra/managed-identity support for on-prem `MsSqlDatabase` (Azure AD auth doesn't apply there), a fully assembled connection-string value (individual fields are exported instead — matches how `dbName` already works).

## Mechanism

`AzureDatabase`/`AzureManagedDatabase` currently only touch the `azure-native` (ARM) provider — no T-SQL connection exists for them. Adding a user requires opening one. We do this via `pulumi-mssql` (`pulumiverse/pulumi-mssql`, already a dependency, already used by `MsSqlDatabase`), connecting with `AzureAuth` (clientId/clientSecret/tenantId) sourced from the **same ambient service-principal env vars** (`AZURE_CLIENT_ID`/`AZURE_CLIENT_SECRET`/`AZURE_TENANT_ID`) that `pulumi.go` already wires up for the `azure-native`/`azuread` providers — this SP is the one already configured as AAD Admin on target servers/MIs, so no new credential field is introduced. `MsSqlDatabase` keeps using its existing `sqlAuth`-based provider — no change there.

Once the ARM-created database exists, `mssql.LookupDatabase{Name: dbName}` resolves it to the `pulumi-mssql` provider's own database ID (`DB_ID(...)`), which every subsequent user/login/role resource targets via `DatabaseId`.

`pulumi-mssql` ships typed resources we use instead of hand-rolled scripts wherever possible: `SqlLogin` (server-level login), `SqlUser` (login-mapped db user), `AzureadServicePrincipal` (contained AAD db user keyed by client ID — what a managed identity registers as), `DatabaseRoleMember` (role grant). The one gap: there is no typed resource for a **password-based contained user with no login** (what Azure SQL Database needs, since it has no server-level login concept). For that one case we fall back to an idempotent `mssql.Script` (`CREATE USER ... WITH PASSWORD=...` + `ALTER ROLE ... ADD MEMBER ...`), the same Create/Read/Update/Delete-script idiom `mssql_db.go` already uses for `ownerLoginName` and restore.

## Shared types (`commonTypes.go`)

```go
// DatabaseUserSpec describes an optional app-facing database user.
type DatabaseUserSpec struct {
    // Login/user name. Defaults to the provisioned database name if omitted.
    // +optional
    Name string `json:"name,omitempty"`
    // Database role(s) granted to this user (e.g. db_owner, db_datareader). No roles are granted if omitted.
    // +optional
    Roles []string `json:"roles,omitempty"`
}

// ManagedIdentitySpec describes an optional Entra (Azure AD) user-assigned managed identity
// wired in as a contained database user. Azure-native database kinds only.
type ManagedIdentitySpec struct {
    // Identity name. Defaults to the provisioned database name if omitted.
    // +optional
    Name string `json:"name,omitempty"`
    // Resource group the managed identity is created in.
    ResourceGroupName string `json:"resourceGroupName"`
    // Azure region.
    Location string `json:"location"`
    // Database role(s) granted to this identity. No roles are granted if omitted.
    // +optional
    Roles []string `json:"roles,omitempty"`
}
```

The password for `DatabaseUserSpec` is never part of the spec — it is always auto-generated with `random.RandomPasswordArgs` (same shape `entra_user.go` already uses; extracted into a shared `newRandomPassword(ctx, name)` helper in `exporters.go` since it will now be called from four places).

`User` and `ManagedIdentity` are independent, optional, and may both be set on the same database (e.g. during a SQL-auth → Entra migration).

## Per-resource spec changes

| Resource | New field(s) | Notes |
|---|---|---|
| `AzureDatabaseSpec` | `User *DatabaseUserSpec`, `ManagedIdentity *ManagedIdentitySpec` | Both optional |
| `AzureManagedDatabaseSpec` | `User *DatabaseUserSpec`, `ManagedIdentity *ManagedIdentitySpec` | Both optional |
| `MsSqlDatabaseSpec` | `User *DatabaseUserSpec` | No managed-identity option (on-prem, no AAD) |

### AzureDatabase (azuresql) — contained user

- SQL-auth path: single idempotent `mssql.Script` per database — `CREATE USER [name] WITH PASSWORD='<generated>'` + one `ALTER ROLE [role] ADD MEMBER [name]` per configured role.
- Managed-identity path (see below): typed `AzureadServicePrincipal` + `DatabaseRoleMember`, no script.

### AzureManagedDatabase (sqlmi) — login + db user

- SQL-auth path: `mssql.NewSqlLogin` (server-level, generated password) → `mssql.NewSqlUser{DatabaseId, LoginId, Name}` → `mssql.NewDatabaseRoleMember` per role. Fully typed — SQL MI is a real instance so server logins behave the same as on-prem.
- Managed-identity path: same as azuresql.

### MsSqlDatabase (on-prem) — login + db user

- New `User` field is **distinct** from the existing admin `sqlServer.sqlAuth` (which remains the provisioning-time admin credential). Same typed `SqlLogin` → `SqlUser` → `DatabaseRoleMember` chain as sqlmi, reusing the `mssql.Provider` already built from the existing admin `sqlAuth` — no new connection.

## Entra managed identity (azuresql & sqlmi only)

1. `managedidentity.NewUserAssignedIdentity` (azure-native SDK, already a dependency via `pulumi-azure-native-sdk/managedidentity/v2`) creates the UAMI in `ManagedIdentity.ResourceGroupName`/`Location`.
2. `mssql.NewAzureadServicePrincipal{DatabaseId, ClientId: identity.ClientId, Name}` registers it as a contained AAD database user (`CREATE USER ... FROM EXTERNAL PROVIDER`, resolved by client ID — this is exactly what a managed identity registers as in Azure AD).
3. `mssql.NewDatabaseRoleMember{RoleId, MemberId: servicePrincipal.Id}` per configured role (built-in roles resolved via `mssql.LookupDatabaseRole{Name: role}`).

This assumes the ambient SP connecting via `AzureAuth` is already the AAD Admin (or delegate) on the target server/MI — confirmed already configured; no AAD Admin provisioning is added by this change.

## Exports

Each resource's `*ExportsSpec` gains new optional `ValueExport` fields alongside the existing `dbName`, following the exact existing routing (`toVault`/`toConfigMap`/`toKubeSecret` + `keyTemplate`) — no new export mechanism:

- `username ValueExport`, `password ValueExport` — populated when `User` is set.
- `identityClientId ValueExport`, `identityPrincipalId ValueExport` — populated when `ManagedIdentity` is set (azuresql/sqlmi only). `clientId` is what an app targets when a pod has multiple federated identities; `principalId` is needed for any additional RBAC/role assignments outside the database.

A consumer combines `dbName` + `username`/`password` (or `identityClientId`) into a connection string downstream (e.g. in a Helm chart), the same way `dbName` is already combined today. No resource in this change assembles a full connection string itself.

## Error handling

- Role names are free-form strings (not validated against a fixed enum, since custom app roles are allowed) — an invalid/nonexistent role surfaces as a Pulumi apply-time error, propagated through the existing reconcile-error path.
- SQL-auth `Script` resources are idempotent (Read/Update/Delete scripts) so re-applying an unchanged `User` spec is a no-op, matching the existing `ownerLoginName` pattern.
- No import/adoption story for pre-existing logins, users, or managed identities is added (mirrors the fact that `ownerLoginName` also has none) — YAGNI unless requested.

## Testing & docs impact

- New/updated Pulumi-mocks-based unit tests (`pulumi.WithMocks`, following `mssql_db_test.go`/`entra_user_test.go`) for each of the three `deploy*` functions covering: no `User`/`ManagedIdentity` set (unchanged behavior), `User` set, and (azuresql/sqlmi) `ManagedIdentity` set.
- Regenerate CRD YAMLs (`helm/crds/provisioning.totalsoft.ro_*.yaml`) and generated clientset/deepcopy/applyconfiguration via `controller-gen` (existing codegen tooling — not run by hand).
- Update `README.md` examples for `AzureDatabase`, `AzureManagedDatabase`, `MsSqlDatabase` with the new `user`/`managedIdentity` fields.

## Open questions

- None outstanding — all prior open points (password source, export shape, MsSql user scope, managed-identity DB wiring, AAD Admin prerequisite, role configurability) were resolved during design review; see decisions above.
