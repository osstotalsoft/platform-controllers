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
  platformRef: charismaonline.qa
```

> *Note* You can skip provisioning for some tenant by adding the label `provisioning.totalsoft.ro/skip-tenant-SOME_TENANT_CODE`="true"


#### AzureManagedDatabase
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuremanageddatabases.yaml)

Example:
```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
spec:
  dbName: origination_db
  domain: origination
  exports:
    dbName:
      toConfigMap:
        keyTemplate: MultiTenancy__Tenants__{{ .Tenant.Code }}__ConnectionStrings__Leasing_Database__Database
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


## configuration.totalsoft.ro
manages external configuration for the services in the platform

### ConfigurationAggregate
Definition can be found [here](./helm/crds/configuration.totalsoft.ro_configurationaggregates.yaml)

It aggregates all config maps for the specified platform and domain and generates an output config map in the same namespace.

Example:
```yaml
apiVersion: configuration.totalsoft.ro/v1alpha1
kind: ConfigurationAggregate
metadata:
  name: test
  namespace: qa-lsng
spec:
  domain: origination
  platformRef: charismaonline.qa
```

> *Note 1* The monitored config maps can be in any namespace, they are identified by `platform.totalsoft.ro/domain`and `platform.totalsoft.ro/platform` labels

> *Note 2* There is support for global platform config maps, in this case the `platform.totalsoft.ro/domain` label has the value "all". These global config maps are always monitored and aggregated with the current domain config maps.