# Extend DB provisioning manifests with an optional user (+ Entra managed identity) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional app-facing database user (and, for the two Azure-native database kinds, an optional Entra managed identity) to the `AzureDatabase`, `AzureManagedDatabase`, and `MsSqlDatabase` provisioning CRDs, with credentials exported the same way `dbName` already is.

**Architecture:** Add two shared spec types (`DatabaseUserSpec`, `ManagedIdentitySpec`) plus per-resource fields. Centralize the actual Pulumi resource wiring (login/user creation, role grants, contained-user script, managed-identity registration) in one new shared file (`mssql_user.go`) so the three `deploy*` functions each call the same small set of helpers instead of duplicating T-SQL/Pulumi logic three times.

**Tech Stack:** Go, Pulumi Go SDK, `pulumiverse/pulumi-mssql`, `pulumi-azure-native-sdk/managedidentity/v2`, `pulumi-random`, `k8s.io/code-generator` + `controller-gen` for CRD/clientset codegen, `stretchr/testify` + Pulumi mocks for tests.

## Global Constraints

- Design source of truth: `docs/superpowers/specs/2026-08-07-db-provisioning-optional-user-design.md` (Approved).
- No new credential fields — the AAD-auth connection for azuresql/sqlmi reuses the ambient `AZURE_CLIENT_ID`/`AZURE_CLIENT_SECRET`/`AZURE_TENANT_ID` env vars already read in `internal/controllers/provisioning/provisioners/pulumi/pulumi.go`.
- Passwords are always auto-generated (`pulumi-random`), never accepted in the spec.
- `User`/`ManagedIdentity`/`ContainedUser` are all optional; omitting them must leave existing behavior byte-for-byte unchanged (this is the main regression risk — every existing test must keep passing unmodified in assertions).
- Follow existing repo conventions exactly: block-scoped-style Go (gofmt), `+optional`/`+kubebuilder` markers matching sibling fields, idempotent `mssql.Script` Read/Update/Delete triples matching the `ownerLoginName` pattern in `mssql_db.go`.
- Build/test commands (Git Bash on Windows, per repo `Makefile`):
  - Build: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./...`
  - Test: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/... -run <TestName> -v`
  - All tests: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./... 2>&1 | grep -v 'platform-controllers/pkg/generated'` (mirrors `make test`)

---

## File Structure

| File | Change |
|---|---|
| `pkg/apis/provisioning/v1alpha1/commonTypes.go` | Add `DatabaseUserSpec`, `ManagedIdentitySpec` |
| `pkg/apis/provisioning/v1alpha1/azureDatabaseTypes.go` | Add `User`, `ManagedIdentity` to `AzureDatabaseSpec`; add export fields |
| `pkg/apis/provisioning/v1alpha1/azureManagedDatabaseTypes.go` | Add `User`, `ContainedUser`, `ManagedIdentity` to `AzureManagedDatabaseSpec`; add export fields |
| `pkg/apis/provisioning/v1alpha1/mssqlDatabaseTypes.go` | Add `User` to `MsSqlDatabaseSpec`; add export fields |
| `pkg/apis/provisioning/v1alpha1/zz_generated.deepcopy.go` | Regenerated (Task 5) |
| `helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml`, `_azuremanageddatabases.yaml`, `_mssqldatabases.yaml` | Regenerated (Task 5) |
| `internal/controllers/provisioning/provisioners/pulumi/exporters.go` | Add shared `newRandomPassword` helper |
| `internal/controllers/provisioning/provisioners/pulumi/entra_user.go` | Use the extracted `newRandomPassword` helper (no behavior change) |
| `internal/controllers/provisioning/provisioners/pulumi/mssql_user.go` | **New.** Shared helpers: `deployDatabaseRoleGrants`, `deployLoginUser`, `deployContainedUser`, `deployManagedIdentity` |
| `internal/controllers/provisioning/provisioners/pulumi/mssql_user_test.go` | **New.** Tests for the four helpers above |
| `internal/controllers/provisioning/provisioners/pulumi/mssql_db.go` | Wire `User` handling via `deployLoginUser`/`deployContainedUser` |
| `internal/controllers/provisioning/provisioners/pulumi/mssql_db_test.go` | Extend with a `User`-set case |
| `internal/controllers/provisioning/provisioners/pulumi/azure_db.go` | Add `mssql.Provider` + `LookupDatabaseOutput`; wire `User`/`ManagedIdentity` |
| `internal/controllers/provisioning/provisioners/pulumi/azure_db_test.go` | **New.** |
| `internal/controllers/provisioning/provisioners/pulumi/azure_managed_db.go` | Add `mssql.Provider` + `LookupDatabaseOutput`; wire `User`(+`ContainedUser`)/`ManagedIdentity` |
| `internal/controllers/provisioning/provisioners/pulumi/azure_managed_db_test.go` | **New.** |
| `README.md` | Update the three resources' example YAML |

---

### Task 1: Shared user/identity types

**Files:**
- Modify: `pkg/apis/provisioning/v1alpha1/commonTypes.go`

**Interfaces:**
- Produces: `provisioningv1.DatabaseUserSpec{Name string, Roles []string}`, `provisioningv1.ManagedIdentitySpec{Name string, ResourceGroupName string, Location string, Roles []string}` — consumed by Tasks 2–4 (spec fields) and Task 7 (helpers).

- [ ] **Step 1: Add the two types**

Append to `commonTypes.go` (after the existing `ProvisioningResourceKind` type at the bottom of the file):

```go
// DatabaseUserSpec describes an optional app-facing database user. The password is never part of
// the spec — it is always auto-generated and exported alongside the username.
type DatabaseUserSpec struct {
	// Login/user name. Defaults to the provisioned database name if omitted.
	// +optional
	Name string `json:"name,omitempty"`
	// Database role(s) granted to this user (e.g. db_owner, db_datareader). No roles are granted if omitted.
	// +optional
	Roles []string `json:"roles,omitempty"`
}

// ManagedIdentitySpec describes an optional Entra (Azure AD) user-assigned managed identity wired
// in as a contained database user. Only applicable to Azure-native database kinds.
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

- [ ] **Step 2: Verify it compiles**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./pkg/apis/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add pkg/apis/provisioning/v1alpha1/commonTypes.go
git commit -m "feat: add DatabaseUserSpec and ManagedIdentitySpec shared types"
```

---

### Task 2: AzureDatabase spec + export fields

**Files:**
- Modify: `pkg/apis/provisioning/v1alpha1/azureDatabaseTypes.go`

**Interfaces:**
- Consumes: `provisioningv1.DatabaseUserSpec`, `provisioningv1.ManagedIdentitySpec` (Task 1)
- Produces: `AzureDatabaseSpec.User *DatabaseUserSpec`, `AzureDatabaseSpec.ManagedIdentity *ManagedIdentitySpec`, `AzureDatabaseExportsSpec.Username/Password/IdentityClientId/IdentityPrincipalId ValueExport` — consumed by Task 9.

- [ ] **Step 1: Add spec fields**

In `AzureDatabaseSpec`, right before `Exports`:

```go
	// Optional app-facing database user (contained, password-based — AzureDatabase has no server-login concept).
	// +optional
	User *DatabaseUserSpec `json:"user,omitempty"`
	// Optional Entra (Azure AD) user-assigned managed identity, wired in as a contained AAD database user.
	// +optional
	ManagedIdentity *ManagedIdentitySpec `json:"managedIdentity,omitempty"`
	// +optional
	Exports          []AzureDatabaseExportsSpec `json:"exports,omitempty"`
```

(replacing the existing `// +optional` / `Exports` pair — keep `ProvisioningMeta` as the last field, unchanged.)

- [ ] **Step 2: Add export fields**

Change `AzureDatabaseExportsSpec` to:

```go
type AzureDatabaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
	// +optional
	Username ValueExport `json:"username,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
	// +optional
	IdentityClientId ValueExport `json:"identityClientId,omitempty"`
	// +optional
	IdentityPrincipalId ValueExport `json:"identityPrincipalId,omitempty"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./pkg/apis/...`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add pkg/apis/provisioning/v1alpha1/azureDatabaseTypes.go
git commit -m "feat: add optional user/managedIdentity fields to AzureDatabase"
```

---

### Task 3: AzureManagedDatabase spec + export fields

**Files:**
- Modify: `pkg/apis/provisioning/v1alpha1/azureManagedDatabaseTypes.go`

**Interfaces:**
- Consumes: `provisioningv1.DatabaseUserSpec`, `provisioningv1.ManagedIdentitySpec` (Task 1)
- Produces: `AzureManagedDatabaseSpec.User *DatabaseUserSpec`, `.ContainedUser bool`, `.ManagedIdentity *ManagedIdentitySpec`, `AzureManagedDatabaseExportsSpec.Username/Password/IdentityClientId/IdentityPrincipalId ValueExport` — consumed by Task 10.

- [ ] **Step 1: Add spec fields**

In `AzureManagedDatabaseSpec`, right before `Exports`:

```go
	// Optional app-facing database user.
	// +optional
	User *DatabaseUserSpec `json:"user,omitempty"`
	// Deploy `user` as a contained database user (password-based, no server-level login) instead of
	// a server login + mapped database user. Ignored if `user` is not set. SQL Managed Instance has
	// contained database authentication enabled by default, so no extra prerequisite is needed.
	// +optional
	// +kubebuilder:default:=false
	ContainedUser bool `json:"containedUser,omitempty"`
	// Optional Entra (Azure AD) user-assigned managed identity, wired in as a contained AAD database user.
	// +optional
	ManagedIdentity *ManagedIdentitySpec `json:"managedIdentity,omitempty"`
	// Export provisioning values spec.
	// +optional
	Exports          []AzureManagedDatabaseExportsSpec `json:"exports,omitempty"`
```

- [ ] **Step 2: Add export fields**

Change `AzureManagedDatabaseExportsSpec` to:

```go
type AzureManagedDatabaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
	// +optional
	Username ValueExport `json:"username,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
	// +optional
	IdentityClientId ValueExport `json:"identityClientId,omitempty"`
	// +optional
	IdentityPrincipalId ValueExport `json:"identityPrincipalId,omitempty"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./pkg/apis/...`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add pkg/apis/provisioning/v1alpha1/azureManagedDatabaseTypes.go
git commit -m "feat: add optional user/containedUser/managedIdentity fields to AzureManagedDatabase"
```

---

### Task 4: MsSqlDatabase spec + export fields

**Files:**
- Modify: `pkg/apis/provisioning/v1alpha1/mssqlDatabaseTypes.go`

**Interfaces:**
- Consumes: `provisioningv1.DatabaseUserSpec` (Task 1)
- Produces: `MsSqlDatabaseSpec.User *DatabaseUserSpec`, `MsSqlDatabaseExportsSpec.Username/Password ValueExport` — consumed by Task 8.

- [ ] **Step 1: Add spec field**

In `MsSqlDatabaseSpec`, right before `Exports`:

```go
	// Optional app-facing database user, distinct from the admin sqlAuth credential used for provisioning.
	// +optional
	User *DatabaseUserSpec `json:"user,omitempty"`
	// +optional
	Exports          []MsSqlDatabaseExportsSpec `json:"exports,omitempty"`
```

- [ ] **Step 2: Add export fields**

Change `MsSqlDatabaseExportsSpec` to:

```go
type MsSqlDatabaseExportsSpec struct {
	// The domain or bounded-context in which this database will be used.
	Domain string `json:"domain"`
	// +optional
	DbName ValueExport `json:"dbName,omitempty"`
	// +optional
	Username ValueExport `json:"username,omitempty"`
	// +optional
	Password ValueExport `json:"password,omitempty"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./pkg/apis/...`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add pkg/apis/provisioning/v1alpha1/mssqlDatabaseTypes.go
git commit -m "feat: add optional user field to MsSqlDatabase"
```

---

### Task 5: Regenerate deepcopy + CRD manifests

**Files:**
- Modify (generated): `pkg/apis/provisioning/v1alpha1/zz_generated.deepcopy.go`
- Modify (generated): `helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml`, `helm/crds/provisioning.totalsoft.ro_azuremanageddatabases.yaml`, `helm/crds/provisioning.totalsoft.ro_mssqldatabases.yaml`

**Interfaces:**
- Consumes: the type changes from Tasks 1–4.
- Produces: `DeepCopy`/`DeepCopyInto` methods for `DatabaseUserSpec`/`ManagedIdentitySpec` and updated `AzureDatabaseSpec`/`AzureManagedDatabaseSpec`/`MsSqlDatabaseSpec` copy logic — required for the CRD types to satisfy `runtime.Object` (build will fail without this once any pointer field is added, since existing `DeepCopyInto` for those specs won't copy the new pointer fields).

- [ ] **Step 1: Confirm codegen tools are installed**

Run: `which controller-gen`
If missing: `go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest`

Run: `ls ~/go/pkg/mod/k8s.io/code-generator@v0.28.2 2>/dev/null || echo missing`
If missing: `go install k8s.io/code-generator@v0.28.2` (must match the version pinned in `hack/update-codegen.sh`)

- [ ] **Step 2: Regenerate deepcopy/clientset**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && make generate-apis`
Expected: exits 0; `git status --short` shows changes under `pkg/apis/provisioning/v1alpha1/zz_generated.deepcopy.go` and (if applicable) `pkg/generated/`.

- [ ] **Step 3: Regenerate CRD YAMLs**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && make generate-crd`
Expected: exits 0; `git status --short` shows changes under `helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml`, `_azuremanageddatabases.yaml`, `_mssqldatabases.yaml` reflecting the new `user`/`containedUser`/`managedIdentity` properties.

- [ ] **Step 4: Verify the whole repo still builds**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./...`
Expected: no output (success).

- [ ] **Step 5: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add pkg/apis/provisioning/v1alpha1/zz_generated.deepcopy.go pkg/generated helm/crds
git commit -m "chore: regenerate deepcopy/clientset/CRDs for optional user fields"
```

---

### Task 6: Extract shared `newRandomPassword` helper

**Files:**
- Modify: `internal/controllers/provisioning/provisioners/pulumi/exporters.go`
- Modify: `internal/controllers/provisioning/provisioners/pulumi/entra_user.go`
- Test: `internal/controllers/provisioning/provisioners/pulumi/entra_user_test.go` (existing — must keep passing unmodified)

**Interfaces:**
- Produces: `newRandomPassword(ctx *pulumi.Context, name string) (pulumi.StringOutput, error)` — consumed by Task 7's `deployLoginUser`/`deployContainedUser`.

This is a pure refactor (no behavior change) — `deployEntraUser`'s existing test must still pass with no edits.

- [ ] **Step 1: Add the helper to `exporters.go`**

Add near the top of the file (after the `const` block), using the exact same shape `deployEntraUser` already builds inline:

```go
// newRandomPassword generates a random password suitable for a SQL login/user or Entra user.
func newRandomPassword(ctx *pulumi.Context, name string) (pulumi.StringOutput, error) {
	randomPassword, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-password", name), &random.RandomPasswordArgs{
		Length:     pulumi.Int(10),
		Upper:      pulumi.Bool(true),
		MinUpper:   pulumi.Int(1),
		Lower:      pulumi.Bool(true),
		MinLower:   pulumi.Int(1),
		Numeric:    pulumi.Bool(true),
		MinNumeric: pulumi.Int(1),
		Special:    pulumi.Bool(true),
		MinSpecial: pulumi.Int(1),
	})
	if err != nil {
		return pulumi.StringOutput{}, err
	}
	return randomPassword.Result, nil
}
```

Add the new import to `exporters.go`'s import block: `"github.com/pulumi/pulumi-random/sdk/v4/go/random"`.

- [ ] **Step 2: Use it from `entra_user.go`**

In `deployEntraUser`, replace:

```go
	initialPassword := pulumi.String(entraUser.Spec.InitialPassword).ToStringOutput()
	if entraUser.Spec.InitialPassword == "" {
		randomPassword, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-initial-password", entraUser.Spec.UserPrincipalName), &random.RandomPasswordArgs{
			Length:     pulumi.Int(10),
			Upper:      pulumi.Bool(true),
			MinUpper:   pulumi.Int(1),
			Lower:      pulumi.Bool(true),
			MinLower:   pulumi.Int(1),
			Numeric:    pulumi.Bool(true),
			MinNumeric: pulumi.Int(1),
			Special:    pulumi.Bool(true),
			MinSpecial: pulumi.Int(1),
		})

		if err != nil {
			return nil, err
		}

		initialPassword = randomPassword.Result
	}
```

with:

```go
	initialPassword := pulumi.String(entraUser.Spec.InitialPassword).ToStringOutput()
	if entraUser.Spec.InitialPassword == "" {
		var err error
		initialPassword, err = newRandomPassword(ctx, fmt.Sprintf("%s-initial", entraUser.Spec.UserPrincipalName))
		if err != nil {
			return nil, err
		}
	}
```

Remove the now-unused `"github.com/pulumi/pulumi-random/sdk/v4/go/random"` import from `entra_user.go` (it moved to `exporters.go`).

- [ ] **Step 3: Run the existing entra_user test to confirm no regression**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployEntraUser -v`
Expected: `PASS`.

- [ ] **Step 4: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add internal/controllers/provisioning/provisioners/pulumi/exporters.go internal/controllers/provisioning/provisioners/pulumi/entra_user.go
git commit -m "refactor: extract shared newRandomPassword helper from deployEntraUser"
```

---

### Task 7: Shared mssql user/identity helpers

**Files:**
- Create: `internal/controllers/provisioning/provisioners/pulumi/mssql_user.go`
- Create: `internal/controllers/provisioning/provisioners/pulumi/mssql_user_test.go`

**Interfaces:**
- Consumes: `newRandomPassword` (Task 6), `provisioningv1.DatabaseUserSpec`/`ManagedIdentitySpec` (Task 1).
- Produces (consumed by Tasks 8–10):
  - `deployLoginUser(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string, databaseId pulumi.StringInput, userSpec *provisioningv1.DatabaseUserSpec, defaultName string, dependencies []pulumi.Resource) (username string, password pulumi.StringOutput, err error)`
  - `deployContainedUser(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string, databaseId pulumi.StringInput, userSpec *provisioningv1.DatabaseUserSpec, defaultName string, dependencies []pulumi.Resource) (username string, password pulumi.StringOutput, err error)`
  - `deployManagedIdentity(ctx *pulumi.Context, provider *mssql.Provider, resourceNamePrefix string, databaseId pulumi.StringInput, identitySpec *provisioningv1.ManagedIdentitySpec, defaultName string, dependencies []pulumi.Resource) (clientId pulumi.StringOutput, principalId pulumi.StringOutput, err error)`

Both `username` fields resolve synchronously (`DatabaseUserSpec.Name` is a plain string, defaulted in Go before any Pulumi resource is created), so they're returned as plain `string`, not `pulumi.StringOutput` — simpler for callers to plug straight into `pulumi.String(username)` when exporting.

- [ ] **Step 1: Write the failing test for `deployLoginUser`**

```go
package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

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
```

- [ ] **Step 2: Run it to confirm it fails to compile (functions don't exist yet)**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployLoginUser -v`
Expected: build error — `undefined: deployLoginUser`.

- [ ] **Step 3: Implement `mssql_user.go`**

```go
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
```

- [ ] **Step 4: Run the test again to verify it passes**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployLoginUser -v`
Expected: `PASS`. If it fails on an Output/Input type mismatch (e.g. `.ToStringOutput()` not defined on `IDOutput`, or a `Ptr` vs non-`Ptr` input mismatch), fix the specific line the compiler flags — these are mechanical conversions (check the sibling type in the same file for the exact conversion method, e.g. `pulumi.ID(...).ToStringOutput()` vs an `ApplyT`) and re-run.

- [ ] **Step 5: Write the failing test for `deployContainedUser`**

```go
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
```

- [ ] **Step 6: Run it, confirm PASS (implementation already exists from Step 3)**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployContainedUser -v`
Expected: `PASS`.

- [ ] **Step 7: Write the failing test for `deployManagedIdentity`**

```go
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
```

- [ ] **Step 8: Run it to verify it passes**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployManagedIdentity -v`
Expected: `PASS`. Same note as Step 4 re: mechanical Output-conversion fixes.

- [ ] **Step 9: Run the full package test suite to confirm no regressions**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -v`
Expected: all `PASS`, including pre-existing `TestDeployEntraUser`, `TestDeployMsSqlDatabase`, etc.

- [ ] **Step 10: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add internal/controllers/provisioning/provisioners/pulumi/mssql_user.go internal/controllers/provisioning/provisioners/pulumi/mssql_user_test.go
git commit -m "feat: add shared mssql user/managed-identity deployment helpers"
```

---

### Task 8: Wire `User` into MsSqlDatabase

**Files:**
- Modify: `internal/controllers/provisioning/provisioners/pulumi/mssql_db.go`
- Modify: `internal/controllers/provisioning/provisioners/pulumi/mssql_db_test.go`

**Interfaces:**
- Consumes: `deployLoginUser` (Task 7), `MsSqlDatabaseSpec.User`, `MsSqlDatabaseExportsSpec.Username`/`Password` (Task 4).

- [ ] **Step 1: Write the failing test**

Add to `mssql_db_test.go` (new `t.Run` inside `TestDeployMsSqlDatabase`, reusing the existing `mssqlDb` var pattern but with `User` set):

```go
	t.Run("msSqlDatabase spec with user", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant2", platform)
		mssqlDb := &provisioningv1.MsSqlDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-db-with-user",
			},
			Spec: provisioningv1.MsSqlDatabaseSpec{
				DbName: "my-db-with-user",
				SqlServer: provisioningv1.MsSqlServerSpec{
					HostName: "localhost",
					Port:     1433,
					SqlAuth: provisioningv1.MsSqlServerAuth{
						Username: "admin",
						Password: "password",
					},
				},
				User: &provisioningv1.DatabaseUserSpec{
					Roles: []string{"db_owner"},
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployMsSqlDb(tenant, mssqlDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployMsSqlDatabase -v`
Expected: it currently compiles and passes trivially (the `User` field exists from Task 4 but is unused) — confirm this, then proceed; the real assertion of "does it wire the user" comes from Step 3's implementation plus the passing rerun in Step 4. (There is deliberately no separate red/green gap here — `User` is optional so a not-yet-wired field is not a compile error. Treat Step 4 as the true confirmation.)

- [ ] **Step 3: Wire `User` handling in `deployMsSqlDb`**

In `mssql_db.go`, after the existing `OwnerLoginName` block (right before `ctx.Export("mssqlDbName", db.Name)`), add:

```go
	var username string
	var password pulumi.StringOutput
	if mssqlDb.Spec.User != nil {
		userDeps := []pulumi.Resource{db}
		if restoreScript != nil {
			userDeps = append(userDeps, restoreScript)
		}
		username, password, err = deployLoginUser(ctx, provider, mssqlDb.Name, db.ID().ToStringOutput(),
			mssqlDb.Spec.User, dbName, userDeps)
		if err != nil {
			return nil, err
		}
	}
```

Then extend the exports loop to also export `username`/`password` when `User` is set:

```go
	for _, exp := range mssqlDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}}
		if mssqlDb.Spec.User != nil {
			values["username"] = exportTemplateWithValue{exp.Username, pulumi.String(username)}
			values["password"] = exportTemplateWithValue{exp.Password, password}
		}
		err = valueExporter(newExportContext(ctx, exp.Domain, mssqlDb.Name, mssqlDb.ObjectMeta, gvk), values)
		if err != nil {
			return nil, err
		}
	}
```

(This replaces the existing single-line `map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}}` call inside the loop.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployMsSqlDatabase -v`
Expected: both `t.Run` subtests `PASS`.

- [ ] **Step 5: Run the full package suite**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -v`
Expected: all `PASS`.

- [ ] **Step 6: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add internal/controllers/provisioning/provisioners/pulumi/mssql_db.go internal/controllers/provisioning/provisioners/pulumi/mssql_db_test.go
git commit -m "feat: wire optional user into MsSqlDatabase provisioning"
```

---

### Task 9: Wire `User`/`ManagedIdentity` into AzureDatabase

**Files:**
- Modify: `internal/controllers/provisioning/provisioners/pulumi/azure_db.go`
- Create: `internal/controllers/provisioning/provisioners/pulumi/azure_db_test.go`

**Interfaces:**
- Consumes: `deployContainedUser`, `deployManagedIdentity` (Task 7), `AzureDatabaseSpec.User`/`.ManagedIdentity`, `AzureDatabaseExportsSpec.*` (Task 2). Ambient env vars `AZURE_CLIENT_ID`/`AZURE_CLIENT_SECRET`/`AZURE_TENANT_ID` (already read elsewhere in `pulumi.go` — read directly here via `os.Getenv`, no new plumbing).

This is the first test file for `azure_db.go` — none existed before this task.

- [ ] **Step 1: Write the failing test**

Create `azure_db_test.go`:

```go
package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func newAzureDb(name string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: provisioningv1.AzureDatabaseSpec{
			DbName: name,
			SqlServer: provisioningv1.SqlServerSpec{
				ResourceGroupName: "SQL_RG",
				ServerName:        "testsvr",
			},
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				DomainRef: "example-domain",
			},
		},
	}
}

func TestDeployAzureDb(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)

	t.Run("no user, no managed identity — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db")
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with contained user", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with managed identity", func(t *testing.T) {
		azureDb := newAzureDb("my-azure-db-mi")
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQL_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
```

Note: `azureSql.LookupServer`/`LookupElasticPool` invokes (already used by the unmodified part of `deployAzureDb`) go through the same generic `mocks.Call` echo — this file's first test (`"no user, no managed identity"`) exercising unmodified code confirms the mock harness works for this function before adding new-code assertions.

- [ ] **Step 2: Run it to confirm the first subtest passes and the other two fail to compile**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployAzureDb -v`
Expected: compiles and the first subtest passes; `User`/`ManagedIdentity` fields already exist (Task 2) so this actually compiles today — the real gap is behavioral (nothing happens with those fields yet). Proceed to Step 3 regardless.

- [ ] **Step 3: Add the mssql connection + user/identity wiring to `deployAzureDb`**

In `azure_db.go`, add to the imports: `"os"`, `mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"`.

Replace the tail of the function, from `ctx.Export("azureDbName", db.Name)` through the end, with:

```go
	ctx.Export("azureDbName", db.Name)

	var username string
	var password, identityClientId, identityPrincipalId pulumi.StringOutput
	if azureDb.Spec.User != nil || azureDb.Spec.ManagedIdentity != nil {
		provider, err := mssql.NewProvider(ctx, "mssql-provider", &mssql.ProviderArgs{
			Hostname: pulumi.String(fmt.Sprintf("%s.database.windows.net", azureDb.Spec.SqlServer.ServerName)),
			AzureAuth: &mssql.ProviderAzureAuthArgs{
				ClientId:     pulumi.String(os.Getenv("AZURE_CLIENT_ID")),
				ClientSecret: pulumi.String(os.Getenv("AZURE_CLIENT_SECRET")),
				TenantId:     pulumi.String(os.Getenv("AZURE_TENANT_ID")),
			},
		})
		if err != nil {
			return nil, err
		}

		dbLookup := mssql.LookupDatabaseOutput(ctx, mssql.LookupDatabaseOutputArgs{
			Name: db.Name,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{db}))
		databaseId := dbLookup.ApplyT(func(r mssql.LookupDatabaseResult) string { return r.Id }).(pulumi.StringOutput)

		if azureDb.Spec.User != nil {
			username, password, err = deployContainedUser(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.User, dbName, []pulumi.Resource{db})
			if err != nil {
				return nil, err
			}
		}

		if azureDb.Spec.ManagedIdentity != nil {
			identityClientId, identityPrincipalId, err = deployManagedIdentity(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.ManagedIdentity, dbName, []pulumi.Resource{db})
			if err != nil {
				return nil, err
			}
		}
	}

	for _, exp := range azureDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}}
		if azureDb.Spec.User != nil {
			values["username"] = exportTemplateWithValue{exp.Username, pulumi.String(username)}
			values["password"] = exportTemplateWithValue{exp.Password, password}
		}
		if azureDb.Spec.ManagedIdentity != nil {
			values["identityClientId"] = exportTemplateWithValue{exp.IdentityClientId, identityClientId}
			values["identityPrincipalId"] = exportTemplateWithValue{exp.IdentityPrincipalId, identityPrincipalId}
		}
		err = valueExporter(newExportContext(ctx, exp.Domain, azureDb.Name, azureDb.ObjectMeta, gvk),
			values)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}
```

(`dbName` here is the already-computed, dot-replaced local variable the existing function builds a few lines above — reuse it, do not recompute.)

- [ ] **Step 4: Run the test to verify all three subtests pass**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployAzureDb -v`
Expected: all three `PASS`. Fix any mechanical Output-conversion errors the compiler flags (same note as Task 7 Step 4).

- [ ] **Step 5: Run the full package suite**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -v`
Expected: all `PASS`.

- [ ] **Step 6: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add internal/controllers/provisioning/provisioners/pulumi/azure_db.go internal/controllers/provisioning/provisioners/pulumi/azure_db_test.go
git commit -m "feat: wire optional user and managed identity into AzureDatabase provisioning"
```

---

### Task 10: Wire `User`/`ContainedUser`/`ManagedIdentity` into AzureManagedDatabase

**Files:**
- Modify: `internal/controllers/provisioning/provisioners/pulumi/azure_managed_db.go`
- Create: `internal/controllers/provisioning/provisioners/pulumi/azure_managed_db_test.go`

**Interfaces:**
- Consumes: `deployLoginUser`, `deployContainedUser`, `deployManagedIdentity` (Task 7), `AzureManagedDatabaseSpec.User`/`.ContainedUser`/`.ManagedIdentity`, `AzureManagedDatabaseExportsSpec.*` (Task 3).

Same pattern as Task 9, with the addition of the `ContainedUser` branch.

- [ ] **Step 1: Write the failing test**

Create `azure_managed_db_test.go`:

```go
package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func newAzureManagedDb(name string) *provisioningv1.AzureManagedDatabase {
	return &provisioningv1.AzureManagedDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: provisioningv1.AzureManagedDatabaseSpec{
			DbName: name,
			ManagedInstance: provisioningv1.AzureManagedInstanceSpec{
				Name:          "incubsqlmi",
				ResourceGroup: "SQLMI_RG",
			},
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				DomainRef: "example-domain",
			},
		},
	}
}

func TestDeployAzureManagedDb(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)

	t.Run("no user, no managed identity — unchanged behavior", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db")
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with login+user (ContainedUser false)", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-login-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with contained user (ContainedUser true)", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-contained-user")
		azureDb.Spec.User = &provisioningv1.DatabaseUserSpec{Roles: []string{"db_owner"}}
		azureDb.Spec.ContainedUser = true
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("with managed identity", func(t *testing.T) {
		azureDb := newAzureManagedDb("my-mi-db-identity")
		azureDb.Spec.ManagedIdentity = &provisioningv1.ManagedIdentitySpec{
			ResourceGroupName: "SQLMI_RG",
			Location:          "westeurope",
			Roles:             []string{"db_owner"},
		}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			db, err := deployAzureManagedDb(tenant, azureDb, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, db)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
```

- [ ] **Step 2: Run it to see the baseline**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployAzureManagedDb -v`
Expected: compiles (fields already exist from Task 3); first subtest passes, others are behaviorally inert until Step 3. Proceed.

- [ ] **Step 3: Add the mssql connection + user/identity wiring to `deployAzureManagedDb`**

In `azure_managed_db.go`, add to imports: `"os"`, `mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"`.

Replace the tail, from the `for _, exp := range azureDb.Spec.Exports` loop through the final `ctx.Export(...)`/`return db, nil`, with:

```go
	var username string
	var password, identityClientId, identityPrincipalId pulumi.StringOutput
	if azureDb.Spec.User != nil || azureDb.Spec.ManagedIdentity != nil {
		provider, err := mssql.NewProvider(ctx, "mssql-provider", &mssql.ProviderArgs{
			Hostname: pulumi.String(fmt.Sprintf("%s.%s.database.windows.net", azureDb.Spec.ManagedInstance.Name, azureDb.Spec.ManagedInstance.ResourceGroup)),
			AzureAuth: &mssql.ProviderAzureAuthArgs{
				ClientId:     pulumi.String(os.Getenv("AZURE_CLIENT_ID")),
				ClientSecret: pulumi.String(os.Getenv("AZURE_CLIENT_SECRET")),
				TenantId:     pulumi.String(os.Getenv("AZURE_TENANT_ID")),
			},
		})
		if err != nil {
			return nil, err
		}

		dbLookup := mssql.LookupDatabaseOutput(ctx, mssql.LookupDatabaseOutputArgs{
			Name: db.Name,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{db}))
		databaseId := dbLookup.ApplyT(func(r mssql.LookupDatabaseResult) string { return r.Id }).(pulumi.StringOutput)

		if azureDb.Spec.User != nil {
			if azureDb.Spec.ContainedUser {
				username, password, err = deployContainedUser(ctx, provider, azureDb.Name, databaseId,
					azureDb.Spec.User, dbName, []pulumi.Resource{db})
			} else {
				username, password, err = deployLoginUser(ctx, provider, azureDb.Name, databaseId,
					azureDb.Spec.User, dbName, []pulumi.Resource{db})
			}
			if err != nil {
				return nil, err
			}
		}

		if azureDb.Spec.ManagedIdentity != nil {
			identityClientId, identityPrincipalId, err = deployManagedIdentity(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.ManagedIdentity, dbName, []pulumi.Resource{db})
			if err != nil {
				return nil, err
			}
		}
	}

	for _, exp := range azureDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbname": {exp.DbName, db.Name}}
		if azureDb.Spec.User != nil {
			values["username"] = exportTemplateWithValue{exp.Username, pulumi.String(username)}
			values["password"] = exportTemplateWithValue{exp.Password, password}
		}
		if azureDb.Spec.ManagedIdentity != nil {
			values["identityClientId"] = exportTemplateWithValue{exp.IdentityClientId, identityClientId}
			values["identityPrincipalId"] = exportTemplateWithValue{exp.IdentityPrincipalId, identityPrincipalId}
		}
		err = valueExporter(newExportContext(ctx, exp.Domain, azureDb.Name, azureDb.ObjectMeta, gvk),
			values)
		if err != nil {
			return nil, err
		}
	}
	ctx.Export(fmt.Sprintf("azureManagedDb:%s", azureDb.Spec.DbName), db.Name)

	return db, nil
}
```

(Keep the existing `"dbname"` lower-case export key exactly as today — this file already uses `"dbname"` while `azure_db.go`/`mssql_db.go` use `"dbName"`; do not "fix" that inconsistency as part of this task, it's out of scope.)

- [ ] **Step 4: Run the test to verify all four subtests pass**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go test ./internal/controllers/provisioning/provisioners/pulumi/... -run TestDeployAzureManagedDb -v`
Expected: all four `PASS`. Fix mechanical Output-conversion issues the same way as prior tasks.

- [ ] **Step 5: Run the full package suite and the whole repo build**

Run: `cd /d/Projects/GitHub/totalsoft.ro/platform-controllers && go build ./... && go test ./internal/controllers/provisioning/provisioners/pulumi/... -v`
Expected: build succeeds, all tests `PASS`.

- [ ] **Step 6: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add internal/controllers/provisioning/provisioners/pulumi/azure_managed_db.go internal/controllers/provisioning/provisioners/pulumi/azure_managed_db_test.go
git commit -m "feat: wire optional user, containedUser and managed identity into AzureManagedDatabase provisioning"
```

---

### Task 11: Update README examples

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: nothing new — purely documents Tasks 2–4's fields.

- [ ] **Step 1: Update the `AzureDatabase` example**

In the `### AzureDatabase` section, extend the example YAML with:

```yaml
  user:
    roles:
      - db_owner
    # name: origination_app_user   # optional, defaults to the provisioned database name
  managedIdentity:
    resourceGroupName: SQL_RG
    location: westeurope
    roles:
      - db_owner
  exports:
    - domain: origination
      dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database
      username:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Username
      password:
        toVault:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password
```

(Merge the new `exports` entries into the existing example's single `exports` item rather than duplicating the `- domain: origination` block.)

- [ ] **Step 2: Update the `AzureManagedDatabase` example**

Same additions as Step 1, plus `containedUser: false` (with a one-line comment: `# set to true to deploy a contained user instead of a server login`).

- [ ] **Step 3: Update the `MsSqlDatabase` example**

In the `### MsSqlDatabase` section, add the same `user`/`exports` additions as Step 1 minus `managedIdentity` (not applicable there).

- [ ] **Step 4: Proofread rendered markdown**

Open `README.md` and visually confirm the three new/edited code blocks are valid YAML (correct indentation, no tab characters) and read naturally in context.

- [ ] **Step 5: Commit**

```bash
cd /d/Projects/GitHub/totalsoft.ro/platform-controllers
git add README.md
git commit -m "docs: document optional user/containedUser/managedIdentity fields in README"
```
