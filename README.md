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
```

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
```

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

## provisioning.totalsoft.ro

monitors infrastructure manifests and provisions the desired infrastructure for every platform / tenant.

### Env

| Variable       | example value         | details                                            |
| -------------- | --------------------- | -------------------------------------------------- |
| AZURE_LOCATION | West Europe           | default location used to deploy resources in azure |
| VAULT_ADDR     | http://localhost:8200 | address to vault server                            |
| VAULT_TOKEN    | {token}               | vault token                                        |

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

### AzureDatabase

Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml)

Example:

```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  dbName: origination_db
  domainRef: origination
  exports:
    - domain: origination
      dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database
  platformRef: charismaonline.qa
  sourceDatabaseId: /subscriptions/XXXXXX-b8a5-4967-a05a-2fa3c4295710/resourceGroups/SQLMI_RG/providers/Microsoft.Sql/servers/r7ddbsrv/databases/insurance_db
  sqlServer:
    elasticPoolName: dbpool
    resourceGroupName: SQL_RG
    serverName: r7ddbsrv
```

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
  exports:
    - domain: origination
      dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Database
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
  name: test-db
  namespace: provisioning-test
spec:
  dbName: test
  domainRef: domain1
  exports:
    - dbName:
        toConfigMap:
          keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Test_Database__Database
      domain: domain1
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
