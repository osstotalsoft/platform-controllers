# platform-controllers
Kubernetes first, multi-tenant infrastructure provisioning services

## provisioning-controller 
monitors infrastructure manifests and provisions the desired infrastructure for every platform tenant.

## Configuration
### Env
- AZURE_LOCATION=West Europe
- VAULT_ADDR=http://localhost:8200
- VAULT_TOKEN={token}

### Kubernetes CRD
#### AzureDatabase
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuredatabases.yaml)

Example:
```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureDatabase
spec:
  dbName: origination_db
  platformRef: charismaonline.qa
```


#### AzureManagedDatabase
Definition can be found [here](./helm/crds/provisioning.totalsoft.ro_azuremanageddatabases.yaml)

Example:
```yaml
apiVersion: provisioning.totalsoft.ro/v1alpha1
kind: AzureManagedDatabase
spec:
  dbName: origination_db
  managedInstance:
    name: incubsqlmi
    resourceGroup: SQLMI_RG
  platformRef: charismaonline.qa
```
