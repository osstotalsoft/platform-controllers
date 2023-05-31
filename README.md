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
  code: qa
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
  code: tenant1
  id: cbb451f2-a7c0-430a-949c-54d576c77b8d
  platformRef: charismaonline.qa
```

## provisioning.totalsoft.ro
monitors infrastructure manifests and provisions the desired infrastructure for every platform tenant.

> *Note* You can skip provisioning for one tenant by adding the label `provisioning.totalsoft.ro/skip-provisioning`="true"

### Env
| Variable       | example value         | details                                            |
|----------------|-----------------------|----------------------------------------------------|
| AZURE_LOCATION | West Europe           | default location used to deploy resources in azure |
| VAULT_ADDR     | http://localhost:8200 | address to vault server                            |
| VAULT_TOKEN    | {token}               | vault token                                        |


### AzureDatabase
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml)

Example:
```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  dbName: origination_db
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

> *Note* You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"


### AzureManagedDatabase
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuremanageddatabases.yaml)

Example:
```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
spec:
  dbName: origination_db
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
> *Note* You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"


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
> *Note* You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"


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
> *Note* You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"


### AzureVirtualDesktop
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azurevirtualdesktops.yaml)

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

> *Note 1* The monitored config maps can be either in the same namespace as the ConfigurationDomain or in the platform's target namespace, they are identified by `platform.totalsoft.ro/domain`and `platform.totalsoft.ro/platform` labels.

> *Note 2* There is support for global platform config maps, in this case the `platform.totalsoft.ro/domain` label has the value "global". These global config maps are always monitored and aggregated with the current domain config maps.

> *Note 3* The monitored vault secrets are organized by platform (secret engine), namespace (subfolder) and domain (subfolder). The keys of the secrets should be the environment variable names, and if they are not unique for the domain they will be overwritten.

> *Note 4* There is support for global platform secrets (subfolder) and global namespace secrets (subfolder), in this case the domain folder should be named "global". These global secrets are always monitored and aggregated with the current domain secrets. 


> For example, for the above manifest, the controller will aggregate secrets from the following paths:
-  /charismaonline.qa/qa/global
-  /charismaonline.qa/qa-lsng/global
-  /charismaonline.qa/qa-lsng/origination/