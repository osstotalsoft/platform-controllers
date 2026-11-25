# platform-controllers

Kubernetes first, multi-tenant infrastructure provisioning services

## platform.totalsoft.ro

Custom platform resources

### Platform

Definition can be found [here](./helm/crds/platform.totalsoft.ro_platforms.yaml)
Example:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: Platform
metadata:
  name: charismaonline.qa
spec:
  targetNamespace: qa
  secretStore: # optional; where toVault exports are written and secrets are aggregated from, see Secret store
    type: AzureKeyVault # HashicorpVault (the default, also when secretStore is absent) | AzureKeyVault
    azureKeyVault: # required exactly when type is AzureKeyVault
      name: kv-lsng-qa
      resourceGroupName: rg-lsng-qa # the subscription comes from AZURE_SUBSCRIPTION_ID
      workloadIdentityClientId: 00000000-0000-0000-0000-000000000000 # the identity pods read the vault with
```

A platform uses exactly one secret store.

### Domain

Definition can be found [here](./helm/crds/platform.totalsoft.ro_domains.yaml)
Example:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: Domain
metadata:
  name: d3
  namespace: qa
spec:
  platformRef: charismaonline.qa
  dependsOn:
    - domain: d1
```

#### Domain dependencies

`spec.dependsOn` (optional) orders the provisioning of a tenant's domains. In the example, a tenant's `d3` stack is provisioned only after the same tenant's `d1` stack has been provisioned successfully at least once. The rules:

- A dependency names a `Domain` on the same platform (`metadata.name` + `spec.platformRef`).
- Dependencies apply only within the same tenant. Platform-level provisioning (`platform-<domain>` stacks) ignores `dependsOn`.
- A dependency the tenant doesn't have in its `domainRefs` is ignored. Only direct dependencies count: if `d3` depends on `d2` and `d2` on `d1`, a tenant with `d3` and `d1` but not `d2` doesn't order `d3` after `d1`.
- A dependency is ready when the tenant's stack for it has had at least one successful update, or when it has no resources for that tenant. A later failure of the dependency doesn't block its dependents.
- While a dependency isn't ready, the dependent domain isn't provisioned and a `WaitingForDomain` warning event is written on the Tenant. This isn't an error and isn't retried: the dependency's successful provisioning starts the dependent domain.
- A dependency cycle among a tenant's domains is reported as a provisioning failure of the domains in the cycle, with a `DomainDependencyCycle` warning event on the Tenant. It isn't retried; changing the `Domain`'s `dependsOn` re-triggers it.

### Tenant

Definition can be found [here](./helm/crds/platform.totalsoft.ro_tenants.yaml)
Example:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: Tenant
metadata:
  name: tenant1
  namespace: qa
spec:
  adminEmail: admin@totalsoft.ro
  deletePolicy: DeleteAll
  description: tenant1
  domainRefs: []
  enabled: true
  id: cbb451f2-a7c0-430a-949c-54d576c77b8d
  platformRef: charismaonline.qa
```

### TenantCategory

Definition can be found [here](./helm/crds/platform.totalsoft.ro_tenantcategories.yaml)

A `TenantCategory` is an optional classification (country, business typology, tenant group, etc.) that a `Tenant` can reference by name via `spec.categoryRef`. A category is resolved by matching its `spec.platformRef` against the tenant's own `spec.platformRef` — a tenant can only reference a `TenantCategory` belonging to the same platform, regardless of which namespace either resource lives in. Like a `Tenant`, it can declare its own `provisioningOverrides` (see [Overrides](#overrides)), applied to every tenant referencing it.

> _Note_ If a tenant's `categoryRef` does not resolve to an existing `TenantCategory` on the same platform, provisioning for that tenant fails until the reference is corrected. Updating a `TenantCategory` automatically re-triggers provisioning for every tenant of that platform referencing it.

Example:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: TenantCategory
metadata:
  name: CEEAS
  namespace: qa
spec:
  platformRef: charismaonline.qa
  description: Central and Eastern Europe, Austria, Switzerland
```

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: Tenant
metadata:
  name: tenant1
  namespace: qa
spec:
  platformRef: charismaonline.qa
  categoryRef: CEEAS
  ...
```

## provisioning.totalsoft.ro

monitors infrastructure manifests and provisions the desired infrastructure for every platform / tenant.

### Env

| Variable               | example value         | details                                                                                |
| ---------------------- | --------------------- | -------------------------------------------------------------------------------------- |
| AZURE_LOCATION         | West Europe           | default location used to deploy resources in azure                                     |
| VAULT_ADDR             | http://localhost:8200 | address to vault server                                                                |
| VAULT_TOKEN            | {token}               | vault token                                                                            |
| MIGRATION_JOBS_ENABLED | false                 | whether to start migration jobs, see [Migration jobs](#migration-jobs); default `true` |

### Migration jobs

When a tenant's domain is provisioned with changes and the tenant is enabled, the provisioner starts the domain's migrations: it clones every Job labelled `provisioning.totalsoft.ro/migration-job-template: "true"` and `platform.totalsoft.ro/domain: <domain>` into a Job for the tenant.

`MIGRATION_JOBS_ENABLED=false` (Helm value `global.migrationJobsEnabled: false`) turns this off for the whole cluster

### Target

The infrastructure manifests can specify wheather the infrastructure can be provisioned for each tenant in a platform, or for the entire platform

#### Tenant target (default)

The provisioning target can be set to `Tenant`, allowing provioning of 'per tenant' infrastructure resources.

> _Note_ You can skip provisioning for a list of tenants you can specify a `Blacklist` filter. To allow provisioning for a subset of the tenants, you can specify a `Whitelist` filter.

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  ....
  target:
   category: Tenant
   filter:
     kind: Blacklist
     values:
       - mbfs
       - bnpro
```

By default the filter `values` are matched against the tenant name. To filter tenants by their `Tenant.spec.categoryRef` instead (e.g. to provision a resource only for tenants belonging to a business/region group), set `filter.by` to `Category`. The values are matched against the name of the referenced `TenantCategory` resource (see below):

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  ....
  target:
   category: Tenant
   filter:
     kind: Whitelist
     by: Category
     values:
       - CEEAS
```

#### Platform target

The `Platform` target allows provisioning shared resources for the entire platform.

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
metadata:
  name: mercury-db
spec:
 ...
  target:
    category: Platform
```

### Overrides

The spec of a provisioning resource can be overridden per tenant category (two ways), per tenant name, or per tenant. When more than one override applies to the same tenant, they are merged in the following order (later ones win):

1. `categoryOverrides` (on the provisioning resource) — dictionary keyed by the exact `Tenant.spec.categoryRef` value, spec override as value.
2. `TenantCategory.spec.provisioningOverrides` — declared on the `TenantCategory` referenced by the tenant's `categoryRef`, targeting a specific resource by kind/apiVersion/name/namespace. Applies to every tenant referencing that category.
3. `tenantOverrides` (on the provisioning resource) — keyed by a glob pattern (`filepath.Match`) matched against the tenant name.
4. `Tenant.spec.provisioningOverrides` — declared on the tenant itself, targeting a specific resource by kind/apiVersion/name/namespace. Always takes precedence.

Example:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: TenantCategory
metadata:
  name: CEEAS
  namespace: qa
spec:
  platformRef: charismaonline.qa
  provisioningOverrides:
    - target:
        kind: AzureDatabase
        apiVersion: provisioning.totalsoft.ro/v1alpha1
        name: mercury-db
      spec:
        sqlServer:
          elasticPoolName: dbpool-ceeas
```

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
metadata:
  name: mercury-db
spec:
  ....
  categoryOverrides:
    CEEAS:
      sqlServer:
        elasticPoolName: dbpool-ceeas-fallback
  tenantOverrides:
    mbfs*:
      sqlServer:
        elasticPoolName: dbpool-mbfs
```

### Dependencies

An infrastructure manifests can specify wheather the provisioned resources depend on the resources provisioned by another manifest.
If a dependency is specified, the provisioning of the dependent resource is delayed until the dependency is provisioned.

> _Note_ Dependencies can be specified between resources provisioned for the same platform, tenant and domain.

The dependency list is specified in the "dependsOn" present on every provisoning manifest. A dependency is identified by kind and name.

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
metadata:
  name: my-db
spec:
  ...

---

apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: HelmRelease
metadata:
  name: my-server
spec:
  dependsOn:
    - kind: AzureManagedDatabase
      name: my-db
  ...
```

### Secret store

`toVault` exports a value to the secret store of the target's platform (`Platform.spec.secretStore`): HashiCorp Vault (the default) or Azure Key Vault. Provisioning resources don't depend on which one: switching a platform's store reprovisions its domains, which exports every value to the new store and deletes it from the old one, and the configuration domains' SecretProviderClasses are rewritten for the new store (see ConfigurationDomain).

```yaml
  exports:
    - domain: origination
      password:
        toVault:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password
```

- **HashiCorp Vault** needs `VAULT_ENABLED=true`.
- **Azure Key Vault** needs `AZURE_ENABLED=true`, and ignores `VAULT_ENABLED`. A cluster without HashiCorp Vault sets `VAULT_ENABLED=false` and `AZURE_ENABLED=true`, plus the usual Azure identity settings.
- A tenant whose platform doesn't exist fails its `toVault` exports, rather than exporting them to HashiCorp Vault.

> _Switching a platform from HashiCorp Vault to Azure Key Vault_: keep `VAULT_ENABLED=true` (and the Vault settings) on the provisioner until every domain of the platform, for the platform and each of its tenants, has been reprovisioned: deleting the old Vault secrets needs Vault. Disabling it in the same rollout fails every stack of the platform, on every retry. During the switch, the configuration-domain-controller may write a domain's `provider: azure` SecretProviderClass before the provisioner has exported all of its values to the vault, so pods restarted then can mount fewer secrets than expected; restart them once the platform's stacks have succeeded.

#### Azure Key Vault

Unlike Vault, where all the values of an object go into one JSON secret at `<platform>/<namespace>/<domain>/[<tenant>/]<object>`, every value gets its own Key Vault secret, because Key Vault has no paths and Azure SecretProviderClasses can't pick keys out of a JSON secret:

- **Name**: `<domain>-<key>-<hash>`, so that a domain's secrets are listed together in the vault. In the domain and the rendered key, `__` is replaced by `--` and any other character Key Vault doesn't allow by `-`, and the end of the key is cut to fit 127 characters. The hash is the first 8 hex characters of the SHA-256 of `<platform>/<namespace>/<domain>/[<tenant>/]<object>/<key>`, so the same key exported from another platform, namespace, domain, tenant or object never collides. E.g. `MultiTenancy__Tenants__acme__ConnectionStrings__Password` exported for the `origination` domain is named `origination-MultiTenancy--Tenants--acme--ConnectionStrings--Password-<hash>`.
- **Tags**: `platform`, `namespace`, `domain`, `tenant` (tenant targets only), `object`, and `configKey`, the rendered key. The rendered key can be at most 256 characters long.

#### Identities and roles

Three identities use each platform's vault:

| Identity | Role on the vault | For |
|---|---|---|
| Provisioner | **Contributor** + **Key Vault Secrets Officer** | Writing and deleting the secrets (below) |
| Configuration-domain-controller | **Key Vault Reader** | Listing the secrets' names and tags, never their values |
| Pods (`secretStore.azureKeyVault.workloadIdentityClientId`) | **Key Vault Secrets User** | Reading the values through the Secrets Store CSI driver |

The configuration-domain-controller signs in with the Azure SDK's default credential: with `global.azure.useWorkloadIdentity`, its service account gets `global.azure.configurationDomainController.workloadIdentityClientId` (or, if empty, the provisioner's `workloadIdentityClientId`), which needs a federated credential for `system:serviceaccount:<release namespace>:configuration-domain-controller`; otherwise it uses the `azure-config` / `azure-secret` client ID and secret.

The platform's identity is a user-assigned managed identity, created once per platform, outside this chart and the provisioner. Neither creates it; the identity binding below only adds a federated credential to it. It must be in the cluster's tenant, but can be in any subscription and resource group:

```sh
az identity create -g <identity rg> -n <identity name> --query "{id:id, clientId:clientId, principalId:principalId}"
```

- `clientId` goes into the Platform's `secretStore.azureKeyVault.workloadIdentityClientId` and the ClusterRole below.
- `principalId` gets **Key Vault Secrets User** on the platform's vault, granted on the vault itself, so that the pods can't read other vaults:

  ```sh
  az role assignment create --role "Key Vault Secrets User" \
    --assignee-object-id <principalId> --assignee-principal-type ServicePrincipal \
    --scope /subscriptions/<subscription>/resourceGroups/<vault rg>/providers/Microsoft.KeyVault/vaults/<vault>
  ```

  For a vault using access policies instead of Azure RBAC, give the identity an access policy with the secret **Get** permission instead.
- `id`, the identity's resource ID (`/subscriptions/<subscription>/resourceGroups/<identity rg>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<identity name>`), is passed to the identity binding. For an existing identity, `az identity show -g <identity rg> -n <identity name>` returns all three.

The pods sign in as their own service account, with the platform's identity. A namespace can have more service accounts than the 20 federated credentials an identity holds, so each cluster running the platform needs, outside this chart:

1. The Key Vault provider of the Secrets Store CSI driver: `az aks enable-addons --addons azure-keyvault-secrets-provider -g <cluster rg> -n <cluster>`.
2. An AKS identity binding of the platform's identity: `az aks identity-binding create -g <cluster rg> --cluster-name <cluster> --name <binding> --managed-identity-resource-id <identity id>`, with the identity's `id` from above. The command needs the `aks-preview` Azure CLI extension (`az extension add --name aks-preview`). Creating it adds one federated credential to the identity (`aks-identity-binding`), the only one the pods need, however many service accounts they use.
3. A ClusterRole allowing the platform's namespaces to use it, bound to their service accounts:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: use-mi-<client id>
rules:
  - apiGroups: ["cid.wi.aks.azure.com"]
    resources: ["<client id>"]
    verbs: ["use-managed-identity"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: use-mi-<client id>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: use-mi-<client id>
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:serviceaccounts:qa-lsng # one per namespace of the platform
```

Pods need no annotation, label or federated credential of their own: the SecretProviderClass sets `useAzureTokenProxy: "true"`, so the Key Vault provider (v1.8.1 or later) gets the token through the identity binding's proxy, which checks the ClusterRole. A pod whose service account it doesn't cover doesn't start, with `403 Forbidden: service account is not an allowed subject for this identity binding`. When the binding is created, AKS adds the `api://AKSIdentityBinding` audience to the CSI driver. The cluster needs the OIDC issuer and workload identity (`az aks update --enable-oidc-issuer --enable-workload-identity`); identity bindings aren't supported with API server VNet integration.

#### Required permissions

The secrets are written with `azure-native`, as the same identity as the other Azure resources (the provisioner's service principal or workload identity). It needs two permissions on each platform's vault:

| Operation | Goes through | Needs |
|---|---|---|
| Create and update a secret | Azure Resource Manager (`Microsoft.KeyVault/vaults/secrets/write`) | **Contributor** (or **Key Vault Contributor**) on the vault, its resource group or the subscription |
| Delete a secret | The vault's data plane (`Microsoft.KeyVault/vaults/secrets/delete`) | **Key Vault Secrets Officer** on the vault |

Grant the second role on the vault itself, not above it, so that the identity can't reach the secrets of other vaults:

```sh
az role assignment create --role "Key Vault Secrets Officer" \
  --assignee-object-id <identity object id> --assignee-principal-type ServicePrincipal \
  --scope /subscriptions/<subscription>/resourceGroups/<resource group>/providers/Microsoft.KeyVault/vaults/<vault>
```

Without it, exporting works, but as soon as a value stops being exported, its stack fails with `403 Forbidden` (`ForbiddenByRbac`, action `Microsoft.KeyVault/vaults/secrets/delete`) and keeps failing on every retry, with the secret left in place, until the role is granted. For a vault using access policies instead of Azure RBAC, give the identity an access policy with the secret **Delete** permission instead.

#### Deleting, re-exporting and recovering

When a value stops being exported (or its tenant or domain is removed), its secret is deleted, which in Key Vault is a soft delete: it stays recoverable for the vault's retention period, and `azure-native` v2 doesn't recover it when the same value is exported again. Until it is purged (`az keyvault secret purge --vault-name <vault> --name <secret>`) or its retention ends, exporting the same key from the same object again fails with a conflict.

Changing a platform's store type, vault name or resource group reprovisions the platform's domains, for the platform and each of its tenants; changing only `workloadIdentityClientId` doesn't, since no export depends on it. In import mode (see below), secrets that already exist in the vault are adopted instead of created, since `azure-native` refuses to create a secret that exists (`cannot create already existing resource`); unlike stateful resources, a secret that doesn't exist yet is simply created. Azure never returns a secret's value, so the import leaves the secret as it is, and the post-import update then writes the exported value, attributes and tags.

### Importing existing resources (disaster recovery)

If the Pulumi state is lost, the stateful resources it managed (the ones retained under the `RetainStatefulResources` delete policy) still exist, and recreating them would fail. Import mode adopts them into the stacks instead:

- **Globally:** set `PULUMI_IMPORT_ALL=true` on the provisioner (Helm: `global.importAll`).
- **Per tenant:** set `import: true` or `import: false` on a `Tenant`. It overrides the global setting for all of that tenant's resources.
- **Per resource:** set `import: true` or `import: false` on a provisioning resource. It overrides both settings above for that resource. Like any spec field, it can be set per tenant or tenant category through the [overrides](#overrides).

Only `AzureDatabase`, `AzureManagedDatabase`, `MsSqlDatabase` and `MinioBucket` honor `import`, together with their SQL logins and users and managed identities. Other kinds ignore the field. A stack's resource group, which isn't a provisioning resource itself, follows the global setting.

While import is on:

- Resources a stack already manages are never re-imported, so it is safe to leave on until the recovery is done.
- A stateful resource that doesn't exist fails the reconcile.
- SQL login passwords are regenerated, and a second update in the same reconcile sets them on the logins.

Example: recover all of one tenant's resources, with the global setting off:

```yaml
apiVersion: platform.totalsoft.ro/v1alpha1
kind: Tenant
metadata:
  name: tenant1
spec:
  ...
  import: true
```

Example: recover only one database of a tenant:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
metadata:
  name: my-db
spec:
  ...
  tenantOverrides:
    tenant1:
      import: true
```

### AzureDatabase

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  dbName: origination_db
  domainRef: origination
  users:
    - name: origination_app_user
      roles:
        - db_owner
      permissions:
        - EXECUTE
      schemaPermissions:
        dbo:
          - EXECUTE
  managedIdentities:
    - name: origination_app_identity
      resourceGroupName: SQL_RG
      location: westeurope
      roles:
        - db_owner
  exports:
    - domain: origination
      dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database
      users:
        - userRef: origination_app_user # optional here since users has exactly one entry
          username:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Username
          password:
            toVault:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password
      identities:
        - identityRef: origination_app_identity # optional here since managedIdentities has exactly one entry
          identityClientId:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityClientId
  platformRef: charismaonline.qa
  sourceDatabaseId: /subscriptions/XXXXXX-b8a5-4967-a05a-2fa3c4295710/resourceGroups/SQLMI_RG/providers/Microsoft.Sql/servers/r7ddbsrv/databases/insurance_db
  sqlServer:
    elasticPoolName: dbpool
    resourceGroupName: SQL_RG
    serverName: r7ddbsrv
```

`exports[].users` and `exports[].identities` work as described in [Exporting users and identities](#exporting-users-and-identities).

> _Note_ You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"

### AzureManagedDatabase

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuremanageddatabases.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
spec:
  dbName: origination_db
  domainRef: origination
  containedUser: false # set to true to deploy every users[] entry as a contained user instead of a server login
  users:
    - name: origination_app_user
      roles:
        - db_owner
      permissions:
        - EXECUTE
      schemaPermissions:
        dbo:
          - EXECUTE
  managedIdentities:
    - name: origination_app_identity
      resourceGroupName: SQL_RG
      location: westeurope
      roles:
        - db_owner
      serviceAccounts: # optional, AKS Workload Identity
        - name: origination-api # namespace defaults to this resource's namespace
    - name: origination_migrations_identity
      resourceGroupName: SQL_RG
      location: westeurope
      roles:
        - db_owner
  exports:
    - domain: origination
      dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database
      users:
        - userRef: origination_app_user # optional here since users has exactly one entry
          username:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Username
          password:
            toVault:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password
      identities: # required here since managedIdentities has more than one entry
        - identityRef: origination_app_identity
          identityClientId:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__IdentityClientId
        - identityRef: origination_migrations_identity
          identityClientId:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__MigrationClientId
  managedInstance:
    name: incubsqlmi
    resourceGroup: SQLMI_RG
  restoreFrom:
    backupFileName: origination_test_backup_2022_04_13_154723.bak
    storageContainer:
      uri: https://my-blobstorage.blob.core.windows.net/backup-repository
      sasToken: my-saas-token
  platformRef: charismaonline.qa
```

> _Note_ You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"

#### Exporting users and identities

An `exports[]` entry publishes values into its `domain`'s ConfigMap, Secret and vault secret — one of each per domain. `users` and `identities` (`identities` only on `AzureManagedDatabase` and `AzureDatabase`; `users` also on `MsSqlDatabase`) let one entry publish several of them there:

- Each `users` item exports one `users` entry's `username` and/or `password`; each `identities` item exports one `managedIdentities` entry's `identityClientId` and/or `identityPrincipalId`. An item must export at least one value.
- `userRef` / `identityRef` select the entry. They are optional only when the list they point to has exactly one entry.
- An entry can export the same user or identity only once.
- All the entry's values end up side by side in the same objects, so their `keyTemplate`s must render distinct, non-empty keys in each destination. An entry that sets `users` or `identities` is rejected otherwise; the check covers every value of that entry, `dbName` included.

> _Deprecated_ The single-value fields `userRef`/`username`/`password` and `identityRef`/`identityClientId`/`identityPrincipalId` on an `exports[]` entry still work exactly as before, and can be combined with the lists, but new specs should use `users` and `identities`. Moving a spec to the lists with the same `keyTemplate`s publishes the same keys and values.

#### Server logins

Unless `containedUser` is set, each `users` entry (also on `MsSqlDatabase`) is a server login named `<name>_<database name>`, mapped to a database user. SQL Server refuses to drop a login that still has open sessions, and a tenant's applications are usually still connected when its stack is destroyed. So when a `users` entry, its database or the tenant is deleted, the login is first disabled, so applications can't reconnect, and its open sessions are killed; then it is dropped. Under the `RetainStatefulResources` delete policy the login is kept and stays enabled.

#### Managed identities

Each `managedIdentities` entry (also available on `AzureDatabase`) creates, per tenant, an Azure user-assigned managed identity named `<name>_<database name>` and adds it to the database as an Entra user with the given `roles`, `permissions` and `schemaPermissions`. Its client and principal ids are exported with an `exports[].identities` item; see [Exporting users and identities](#exporting-users-and-identities).

On a Managed Instance (`AzureManagedDatabase`) the Entra user is created with `CREATE USER [<name>_<database name>] FROM EXTERNAL PROVIDER`, which looks the identity up in Entra ID by its display name; an existing user with that name is kept as it is. (`AzureDatabase` creates it from the identity's client id, without a directory lookup.) So every Managed Instance used with `managedIdentities` needs:

- an Entra admin that includes the controller's identity, directly or through a group;
- the **Directory Readers** role (or the equivalent Microsoft Graph permissions) for the instance's own identity, because the statement is issued by a service principal;
- identity display names unique in the Entra tenant, which the tenant-scoped `<name>_<database name>` guarantees.

`serviceAccounts` lets Kubernetes workloads authenticate as the identity through [AKS Workload Identity](https://learn.microsoft.com/azure/aks/workload-identity-overview): a federated identity credential is created on the identity for each ServiceAccount, with subject `system:serviceaccount:<namespace>:<name>`, the cluster's OIDC issuer (`AZURE_AKS_OIDC_ISSUER_URL`, required when any `serviceAccounts` is set) and audience `api://AzureADTokenExchange`. The ServiceAccount is not created and needs no `azure.workload.identity/client-id` annotation; its pods need the label `azure.workload.identity/use: "true"` and select the identity by its exported client id, e.g. with Microsoft.Data.SqlClient `Authentication=Active Directory Workload Identity;User Id=<client id>`. Federated credentials are named `<namespace>_<name>` and are never retained on delete nor imported: they are recreated from the spec. So after a lost Pulumi state, `PULUMI_IMPORT_ALL` can't adopt them: delete the existing ones (`az identity federated-credential delete`) and the next reconcile recreates them. A `serviceAccounts` entry on a cluster without a valid `AZURE_AKS_OIDC_ISSUER_URL` fails the reconcile of the whole tenant/domain stack, like any other spec validation error.

### HelmRelease

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_helmreleases.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: HelmRelease
metadata:
  name: my-helm-release
  namespace: my-namespace
spec:
  domainRef: origination
  platformRef: radu.demo
  release:
    interval: 10m
    releaseName: my-release
    chart:
      spec:
        version: ">=0.1.0-0"
        chart: my-chart
        sourceRef:
          kind: HelmRepository
          name: my-helm-repo
          namespace: my-namespace
    upgrade:
      remediation:
        remediateLastFailure: false
    values:
      global:
        vaultEnvironment: "false"

        runtimeConfiguration:
          enabled: false
          configMap: origination-aggregate
          csi:
            secretProviderClass: origination-aggregate
```

### HelmReleaseV2

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_helmreleasev2s.yaml)

`HelmReleaseV2` supports the same provisioning behavior as `HelmRelease`, but provisions Flux `HelmRelease` resources with `apiVersion: helm.toolkit.fluxcd.io/v2`.

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: HelmReleaseV2
metadata:
  name: my-helm-release-v2
  namespace: my-namespace
spec:
  domainRef: origination
  platformRef: radu.demo
  release:
    interval: 10m
    releaseName: my-release
    chart:
      spec:
        version: ">=0.1.0-0"
        chart: my-chart
        sourceRef:
          kind: HelmRepository
          name: my-helm-repo
          namespace: my-namespace
    upgrade:
      remediation:
        remediateLastFailure: false
    values:
      global:
        vaultEnvironment: "false"

        runtimeConfiguration:
          enabled: false
          configMap: origination-aggregate
          csi:
            secretProviderClass: origination-aggregate
```

> _Note_ You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"

### AzureVirtualMachine

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azurevirtualmachines.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureVirtualMachine
metadata:
  name: charisma-client-vm
  namespace: qa-lsng
spec:
  domainRef: origination
  enableTrustedLaunch: false
  exports:
    - domain: origination
      adminPassword:
        toVault:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_Admin_Password
      adminUserName:
        toVault:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_Admin_UserName
      computerName:
        toConfigMap:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_ComputerName
      publicAddress:
        toConfigMap:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_PublicAddress
      vmName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_VM_Name
  osDiskType: Standard_LRS
  platformRef: charismaonline.qa
  rdpSourceAddressPrefix: 128.0.57.0/25
  sourceImageId: >-
    /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Compute/galleries/MyGallery/images/ch-client-base/versions/2.0.0
  subnetId: >-
    /subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/charismaonline.qa/providers/Microsoft.Network/virtualNetworks/charismaonline-vnet/subnets/default
  vmName: charisma-client
  vmSize: Standard_B1s
```

> _Note_ You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"

### AzureVirtualDesktop

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azurevirtualdesktops.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureVirtualDesktop
metadata:
  name: charisma-client-avd
  namespace: qa-lsng
spec:
  applications:
    - friendlyName: Charisma Enterprise
      name: charisma-enterprise
      path: >-
        C:\Program Files (x86)\TotalSoft\Charisma Enterprise\Windows Client\Charisma.WinUI.exe
  domainRef: origination
  enableTrustedLaunch: false
  exports:
    - adminPassword:
        toVault:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code
            }}__CharismaClient_Admin_Password
      adminUserName:
        toVault:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code
            }}__CharismaClient_Admin_UserName
      computerName:
        toConfigMap:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code
            }}__CharismaClient_ComputerName
      domain: origination
      hostPoolName:
        toConfigMap:
          keyTemplate: >-
            MultiTenancy__Tenants__{{ .Tenant.Code
            }}__CharismaClient_HostPool_Name
  hostPoolName: ch-client-fs
  initScript: |
    param
    (    
      [Parameter(Mandatory = $true)]
      [String]$ChServerIP
    )

    Write-Host $ChServerIP
  initScriptArgs:
    - name: ChServerIP
      value: 11.12.13.17
  osDiskType: Premium_LRS
  platformRef: charismaonline.qa
  sourceImageId: >-
    /subscriptions/15b38e46-ef41-4f5b-bdba-7d9354568c2d/resourceGroups/test-vm/providers/Microsoft.Compute/galleries/LFGalery/images/base-avd/versions/1.0.0
  subnetId: >-
    /subscriptions/15b38e46-ef41-4f5b-bdba-7d9354568c2d/resourceGroups/test-vm/providers/Microsoft.Network/virtualNetworks/ch-client-base-vnet/subnets/default
  users:
    admins:
      - admin-mbfs@test.onmicrosoft.com
    applicationUsers:
      - user1-mbfs@test.onmicrosoft.com
  vmNamePrefix: ch-client-fs
  vmNumberOfInstances: 2
  vmSize: Standard_B2s
  workspaceFriendlyName: Charisma
```

### AzurePowershellScript

`AzurePowershellScript` is a Custom Resource Definition (CRD) that represents an Azure PowerShell deployment script.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azurepowershellscripts.yaml)

#### Spec

The `AzurePowershellScript` spec has the following fields:

- `scriptContent`: The content of the PowerShell script to be executed.
- `scriptArguments`: The arguments to be passed to the PowerShell script. These should match the parameters defined in the scriptContent.
- `domainRef`: The reference to the domain that the user belongs to.
- `platformRef`: The reference to the platform that the user belongs to.
- `forceUpdateTag`: Update this value to trigger the script even if the content or args are unchanged

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzurePowerShellScript
metadata:
  name: createresourcegroup
  namespace: provisioning-test
spec:
  domainRef: domain2
  exports:
    - scriptOutputs:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ScriptOutputs
  platformRef: provisioning.test
  scriptContent: |-
    param([string] $name)

    $output = "RG name: {0}" -f $name 
    Write-Output $output  

    $DeploymentScriptOutputs = @{} 
    $DeploymentScriptOutputs['text'] = $output

    New-AzResourceGroup $name "West Europe"
  scriptArguments: "-name testrg-{{ .Platform }}-{{ .Tenant.Code }}"
  target:
    category: Tenant
```

### EntraUser

`EntraUser` is a Custom Resource Definition (CRD) that represents a user for Entra Id.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_entrausers.yaml)

## Spec

The `EntraUser` spec has the following fields:

- `userPrincipalName`: The user principal name of the user. This is typically the user's email address or username.
- `displayName`: The display name of the user.
- `initialPassword`: The initial password for the user. If this is not provided, a random password will be generated.
- `domainRef`: The reference to the domain that the user belongs to.
- `platformRef`: The reference to the platform that the user belongs to.

## Example

Here's an example of an `EntraUser` resource:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: EntraUser
metadata:
  name: example-user
  namespace: qa-lsng
spec:
  userPrincipalName: "user@tenant1-qa.example.com"
  displayName: "Example User"
  initialPassword: "password123"
  domainRef: "entra-users"
  platformRef: "qa"
  exports:
    - domain: entra-users
      initialPassword:
        toVault:
          keyTemplate: InitialPassword
      userPrincipalName:
        toVault:
          keyTemplate: UserPrincipalName
```

### MinioBucket

`MinioBucket` is a Custom Resource Definition (CRD) that represents a bucket in Minio.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_miniobuckets.yaml)

## Spec

The `MinioBucket` spec has the following fields:

- `bucketName`: The name of the bucket.
- `minioServer`: Optional Minio server configuration. If omitted, the controller uses the default Pulumi Minio provider configured from environment variables.
  - `server`: The Minio server host and port.
  - `user`: The Minio user.
  - `password`: The Minio password.
- `domainRef`: The reference to the domain that the user belongs to.
- `platformRef`: The reference to the platform that the user belongs to.

## Example

Here's an example of an `MinioBucket` resource:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: MinioBucket
metadata:
  name: core-storage
spec:
  bucketName: "core-storage"
  minioServer:
    server: "minio.example.com:9000"
    user: "minio-user"
    password: "minio-password"
  domainRef: "core"
  platformRef: "qa"
  exports:
    - domain: core
      accessKey:
        toVault:
          keyTemplate: AccessKey
      secretKey:
        toVault:
          keyTemplate: SecretKey
```

### MsSqlDatabase

`MsSqlDatabase` is a Custom Resource Definition (CRD) that represents a SQL database.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_mssqldatabases.yaml)

## Spec

The `MsSqlDatabase` spec has the following fields:

- `dbName`: Database name prefix. The actual database name will have platform and tenant suffix.
- `sqlServer`: Specification of the SQL Server where the new database will be created.
  - `hostName`: The host name of the SQL Server.
  - `port`: The port of the SQL Server.
  - `sqlAuth`: The SQL authentication credentials.
    - `username`: The username.
    - `password`: The password.
- `restoreFrom`: Specification for restoring the database from a backup. Leave empty for a new empty database.
  - `backupFilePath`: The path to the backup file.
- `domainRef`: The reference to the domain that the user belongs to.
- `platformRef`: The reference to the platform that the user belongs to.

## Example

Here's an example of an `MsSqlDatabase` resource:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: MsSqlDatabase
metadata:
  name: test-db
  namespace: provisioning-test
spec:
  dbName: test
  domainRef: domain1
  users:
    - name: test_app_user
      roles:
        - db_owner
      permissions:
        - EXECUTE
      schemaPermissions:
        dbo:
          - EXECUTE
  exports:
    - dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Test_Database__Database
      domain: domain1
      users:
        - userRef: test_app_user # optional here since users has exactly one entry
          username:
            toConfigMap:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Username
          password:
            toVault:
              keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Password
  platformRef: provisioning.test
  restoreFrom:
    backupFilePath: C:\tmp\test-db\mydb.bak
  sqlServer:
    hostName: myhost
    port: 1433
    sqlAuth:
      password: mypassword
      username: sa
  target:
    category: Tenant
```

`exports[].users` works as described in [Exporting users and identities](#exporting-users-and-identities).

### LocalScript

`LocalScript` is a Custom Resource Definition (CRD) that represents a script that executes locally.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_localscripts.yaml)

## Spec

The `LocalScript` spec has the following fields:

- `createScriptContent`: Script that runs on resource creation and update.
- `deleteScriptContent`: Script that runs on resource deletion.
- `shell`: The shell to use to run the script. Can be `bash` or `pwsh`.
- `environment`: The environment variables to be passed to the script. It can contain placeholders like `{{ .Tenant.Code }}` or `{{ .Platform }}`.
- `workingDir`: The working directory where the script will be executed.
- `forceUpdateTag`: Update this value to trigger the script even if the content or environment are unchanged. Caution: it performs delete-replace and triggers the Delete script.
- `domainRef`: The reference to the domain that the user belongs to.
- `platformRef`: The reference to the platform that the user belongs to.
- `target`: The target of the script. Can be `Tenant` or `Platform`.
- `exports`: The exports of the script.
- `dependsOn`: The dependencies of the script.

## Example

Here's an example of a `LocalScript` resource:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: LocalScript
metadata:
  name: prepare-data
  namespace: provisioning-test
spec:
  createScriptContent: |
    Get-Date
    Get-Location
    Write-Host "Env1: " $env:env1
    Write-Host "Tenant:" $env:tenant
  deleteScriptContent: Write-Host "Deleted"
  shell: pwsh
  forceUpdateTag: "3"
  domainRef: domain1
  workingDir: c:/temp
  environment:
    env1: env1Val2
    tenant: "{{ .Tenant.Code }}"
  exports:
    - domain: domain1
      scriptOutput:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ScriptOutput
  platformRef: provisioning.test
  target:
    category: Tenant
```

### KeycloakClient

`KeycloakClient` is a Custom Resource Definition (CRD) that represents a Keycloak OpenID client.

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_keycloakclients.yaml)

## Spec

The `KeycloakClient` spec has the following fields:

- `clientName`: The display name of the client in Keycloak.
- `clientId`: The client identifier used by applications.
- `realm`: The Keycloak realm where the client will be created.
- `organization`: Optional organization identifier/template used to associate the service account with a Keycloak organization.
- `enabled`: Indicates if the client is enabled.
- `consentRequired`: Indicates if user consent is required.
- `publicClient`: Indicates whether the client is public or confidential.
- `serviceAccountsEnabled`: Enables service accounts for the client.
- `standardFlowEnabled`: Enables the standard authorization code flow.
- `implicitFlowEnabled`: Enables the implicit flow.
- `directAccessGrantsEnabled`: Enables direct access grants (resource owner password flow).
- `fullScopeAllowed`: Allows full scope for the client.
- `protocolMappers`: Optional protocol mapper definitions.
- `defaultClientScopes`: Optional list of default client scopes.
- `optionalClientScopes`: Optional list of optional client scopes.
- `domainRef`: The reference to the business domain.
- `platformRef`: The reference to the target platform.
- `target`: The provisioning target. Can be `Tenant` or `Platform`.
- `exports`: Exported values such as `clientId` and `clientSecret`.
- `dependsOn`: Optional dependencies on other provisioning resources.

## Example

Here's an example of a `KeycloakClient` resource:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: KeycloakClient
metadata:
  name: agent-gateway
  namespace: provisioning-test
spec:
  clientName: agent-gateway
  clientId: agent-gateway
  realm: charisma
  enabled: true
  consentRequired: false
  publicClient: false
  serviceAccountsEnabled: true
  standardFlowEnabled: true
  implicitFlowEnabled: false
  directAccessGrantsEnabled: false
  fullScopeAllowed: false
  domainRef: security
  platformRef: provisioning.test
  organization: "{{ .Tenant.Id }}"
  defaultClientScopes:
    - profile
    - email
    - roles
  optionalClientScopes:
    - offline_access
  protocolMappers:
    - name: audience-mapper
      protocol: openid-connect
      protocolMapper: oidc-audience-mapper
      config:
        included.custom.audience: agent-gateway
        access.token.claim: "true"
        id.token.claim: "false"
  exports:
    - domain: security
      clientId:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__Security__Keycloak__ClientId
      clientSecret:
        toVault:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__Security__Keycloak__ClientSecret
  target:
    category: Tenant
```

> _Note 1_ When `organization` is specified, the controller also attempts to add the generated service account user to the resolved Keycloak organization.

> _Note 2_ Exported values can be written to ConfigMaps, Kubernetes Secrets, or Vault, similar to other provisioning resources.

## configuration.totalsoft.ro

manages external configuration for the services in the platform, read more about from the [Twelve-Factor App ](https://12factor.net/config) methodology.

### ConfigurationDomain

Definition can be found [here](./helm/crds/configuration.totalsoft.ro_configurationdomains.yaml)

If `aggregateConfigMaps` is set, it will aggregate all config maps from it's namespace and platform's target namespace, for the specified domain and generates an output config map in the same namespace.

If `aggregateSecrets` is set, it will aggregate all secrets stored in vault for the specified platform namespace and domain and generates an output CSI SecretProviderClass namespace.

Example:

```yaml
apiVersion: configuration.totalsoft.ro/v1alpha1
kind: ConfigurationDomain
metadata:
  name: origination
  namespace: qa-lsng
spec:
  aggregateConfigMaps: true
  aggregateSecrets: true
  platformRef: charismaonline.qa
```

> _Note 1_ The monitored config maps can be either in the same namespace as the ConfigurationDomain or in the platform's target namespace, they are identified by `platform.totalsoft.ro/domain`and `platform.totalsoft.ro/platform` labels.

> _Note 2_ There is support for global platform config maps, in this case the `platform.totalsoft.ro/domain` label has the value "global". These global config maps are always monitored and aggregated with the current domain config maps.

> _Note 3_ The monitored vault secrets are organized by platform (secret engine), namespace (subfolder) and domain (subfolder). The keys of the secrets should be the environment variable names, and if they are not unique for the domain they will be overwritten.

> _Note 4_ There is support for global platform secrets (subfolder) and global namespace secrets (subfolder), in this case the domain folder should be named "global". These global secrets are always monitored and aggregated with the current domain secrets.

> For example, for the above manifest, the controller will aggregate secrets from the following paths:

- /charismaonline.qa/qa/global
- /charismaonline.qa/qa-lsng/global
- /charismaonline.qa/qa-lsng/origination/

> _Note 5_ For a platform whose secret store is Azure Key Vault (see Secret store), the controller lists the vault's secrets (names and tags only) and picks the domain's by their tags, in the same three groups: `namespace=qa, domain=global`, `namespace=qa-lsng, domain=global` and `namespace=qa-lsng, domain=origination`, all with `platform=charismaonline.qa`. Tenants' secrets are included; secrets without a `configKey` tag, and disabled ones, are skipped. A later group wins on the same `configKey`; within a group, two secrets with the same `configKey` resolve to the alphabetically first name, and the domain gets a `DuplicateConfigKey` warning event. The output is the same `<domain>-aggregate` SecretProviderClass, with `provider: azure`, the vault, `AZURE_TENANT_ID`, the platform's `workloadIdentityClientId` with `useAzureTokenProxy: "true"` (see Secret store), and one object per key, mounted under its `configKey` (`objectAlias`). A domain without any secret gets a SecretProviderClass without objects, which the CSI driver refuses to mount (`missing object versions`): pods mounting it don't start until the domain has a secret. The vault is polled every 2 minutes. A Key Vault platform needs `AZURE_ENABLED=true` on the controller, otherwise its domains fail to sync.

> _Note 6_ Without the Secrets Store CSI driver (no SecretProviderClass API), the controller still aggregates config maps and kube secrets; domains whose secrets would go to a SecretProviderClass get a warning event.
