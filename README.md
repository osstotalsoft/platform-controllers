# platform-controllers
Kubernetes first, multi-tenant infrastructure provisioning services

## Provisioning-controller 
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


## Configuration-controller