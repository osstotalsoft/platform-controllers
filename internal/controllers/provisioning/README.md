# Provisioning Controller — Configuration Guide

The provisioning controller watches `provisioning.totalsoft.ro` custom resources and drives infrastructure provisioning via Pulumi for every platform/tenant combination.

## Helm installation

```bash
helm upgrade --install platform-controllers ./helm \
  --namespace platform-system \
  --create-namespace \
  -f my-values.yaml
```

## Helm values reference

```yaml
global:
  registry: ghcr.io/osstotalsoft/platform-controllers
  tag: "0.0.1"
  imagePullPolicy: IfNotPresent
  imagePullSecrets: "registrykey"
  logLevel: 4                          # verbosity (klog)
  workersCount: 5                      # parallel reconcile workers
  migrationJobTTLSecondsAfterFinished: 86400

  azure:
    enabled: false
    useMsi: false
    workloadIdentityClientId: ""       # set to enable Workload Identity (see below)

  backend:
    type: cloud                        # "cloud" | "custom"
    customBackedUrl: ""                # required when type=custom

  vault:
    enabled: false
    address: http://vault.vault:8200

  minio:
    enabled: false
    endpoint: http://minio:9000

  keycloak:
    enabled: false
    url: http://keycloak:8080
    clientId: pulumi

  s3:
    enabled: false
    profile: default

  rusi:
    enabled: false
```

## Required Kubernetes resources

### Secret: `provisioner-secrets`

| Key | Required when | Description |
|-----|--------------|-------------|
| `pulumiAccessToken` | `backend.type=cloud` | Pulumi Cloud access token |
| `pulumiConfigPassphrase` | `backend.type=custom` | State encryption passphrase |
| `vaultAccessToken` | `vault.enabled=true` | Vault token |
| `githubAccessToken` | optional | GitHub token for private Helm chart repos |
| `minioAccessKey` | `minio.enabled=true` | Minio access key |
| `minioSecretKey` | `minio.enabled=true` | Minio secret key |
| `keycloakClientSecret` | `keycloak.enabled=true` | Keycloak client secret |

```bash
kubectl create secret generic provisioner-secrets \
  --from-literal=pulumiAccessToken=pul-xxx \
  --from-literal=vaultAccessToken=hvs.xxx \
  -n platform-system
```

### ConfigMap: `azure-config` (when `azure.enabled=true`)

| Key | Description |
|-----|-------------|
| `clientId` | Service principal or user-assigned managed identity client ID |
| `location` | Default Azure region (e.g. `West Europe`) |
| `subscriptionId` | Azure subscription ID |
| `tenantId` | Azure AD tenant ID |
| `managedIdentityRG` | Resource group of the managed identity |
| `managedIdentityName` | Name of the managed identity |

```bash
kubectl create configmap azure-config \
  --from-literal=clientId=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=location="West Europe" \
  --from-literal=subscriptionId=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=tenantId=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=managedIdentityRG=my-rg \
  --from-literal=managedIdentityName=my-identity \
  -n platform-system
```

### Secret: `azure-secret` (service principal mode only)

```bash
kubectl create secret generic azure-secret \
  --from-literal=clientSecret=<client-secret> \
  -n platform-system
```

---

## Azure authentication modes

### 1. Service Principal

Set `azure.useMsi=false` (default). Requires `azure-config` ConfigMap and `azure-secret` Secret.

```yaml
global:
  azure:
    enabled: true
    useMsi: false
```

### 2. Workload Identity

Scopes Azure credentials to the `provisioning-controller` ServiceAccount rather than the entire node. Requires three one-time Azure setup steps, then a single Helm value.

**Step 1 — Enable OIDC issuer and Workload Identity on the cluster:**

```bash
az aks update -g <rg> -n <aks> \
  --enable-oidc-issuer \
  --enable-workload-identity
```

**Step 2 — Create a federated credential on the managed identity:**

```bash
OIDC_ISSUER=$(az aks show -g <rg> -n <aks> \
  --query oidcIssuerProfile.issuerUrl -o tsv)

az identity federated-credential create \
  --name provisioning-controller-fedcred \
  --identity-name <managed-identity-name> \
  --resource-group <identity-rg> \
  --issuer $OIDC_ISSUER \
  --subject "system:serviceaccount:<k8s-namespace>:provisioning-controller" \
  --audiences api://AzureADTokenExchange
```

**Step 3 — Set `workloadIdentityClientId` in Helm values:**

```yaml
global:
  azure:
    enabled: true
    useMsi: true
    workloadIdentityClientId: "<managed-identity-client-id>"
```

The Helm chart will:
- Annotate the `provisioning-controller` ServiceAccount with `azure.workload.identity/client-id`
- Label the pod with `azure.workload.identity/use: "true"` so the webhook injects credentials
- Skip the `AZURE_CLIENT_ID` and `ARM_CLIENT_ID` env vars (the webhook and the controller's fallback logic handle them)

With Workload Identity active, the `azure-config` ConfigMap is still required for `location`, `subscriptionId`, `tenantId`, `managedIdentityRG`, and `managedIdentityName`. The `clientId` key in that ConfigMap is not used — the client ID comes from `workloadIdentityClientId` in Helm values and is injected by the webhook.

---

## Pulumi backend

### Pulumi Cloud (default)

```yaml
global:
  backend:
    type: cloud
```

Requires `pulumiAccessToken` in `provisioner-secrets`.

### Self-hosted / custom backend (e.g. S3-compatible)

```yaml
global:
  backend:
    type: custom
    customBackedUrl: "s3://my-bucket?region=ro&endpoint=http://minio:9000&disableSSL=true&s3ForcePathStyle=true"
```

Requires `pulumiConfigPassphrase` in `provisioner-secrets` for state encryption.

---

## Environment variable reference

All values below are injected by the Helm chart. This table is useful when running the controller outside of Helm.

| Variable | Source | Description |
|----------|--------|-------------|
| `NAMESPACE` | pod field | Kubernetes namespace the controller runs in |
| `AZURE_ENABLED` | `azure.enabled` | Enable Azure Pulumi providers |
| `AZURE_USE_MSI` | `azure.useMsi` | Use managed identity instead of service principal |
| `AZURE_CLIENT_ID` | `azure-config` / webhook | Client ID of the service principal or user-assigned MI |
| `AZURE_CLIENT_SECRET` | `azure-secret` | Service principal secret (non-MSI only) |
| `AZURE_LOCATION` | `azure-config` | Default Azure region |
| `AZURE_SUBSCRIPTION_ID` | `azure-config` | Azure subscription |
| `AZURE_TENANT_ID` | `azure-config` | Azure AD tenant |
| `AZURE_MANAGED_IDENTITY_RG` | `azure-config` | Resource group of the managed identity |
| `AZURE_MANAGED_IDENTITY_NAME` | `azure-config` | Name of the managed identity |
| `ARM_CLIENT_ID` | `azure-config` or fallback | Client ID for the `azuread` Pulumi provider; falls back to `AZURE_CLIENT_ID` when Workload Identity is active |
| `ARM_CLIENT_SECRET` | `azure-secret` | Secret for the `azuread` provider (non-MSI only) |
| `ARM_TENANT_ID` | `azure-config` | Tenant for the `azuread` provider |
| `VAULT_ENABLED` | `vault.enabled` | Enable Vault integration |
| `VAULT_ADDR` | `vault.address` | Vault server URL |
| `VAULT_TOKEN` | `provisioner-secrets` | Vault access token |
| `MINIO_ENDPOINT` | `minio.endpoint` | Minio server URL |
| `MINIO_ACCESS_KEY` | `provisioner-secrets` | Minio access key |
| `MINIO_SECRET_KEY` | `provisioner-secrets` | Minio secret key |
| `KEYCLOAK_URL` | `keycloak.url` | Keycloak server URL |
| `KEYCLOAK_CLIENT_ID` | `keycloak.clientId` | Keycloak client ID |
| `KEYCLOAK_CLIENT_SECRET` | `provisioner-secrets` | Keycloak client secret |
| `PULUMI_ACCESS_TOKEN` | `provisioner-secrets` | Pulumi Cloud token (cloud backend) |
| `PULUMI_BACKEND_URL` | `backend.customBackedUrl` | Custom backend URL |
| `PULUMI_CONFIG_PASSPHRASE` | `provisioner-secrets` | State encryption passphrase (custom backend) |
| `GITHUB_TOKEN` | `provisioner-secrets` | GitHub token for private Helm chart sources |
| `AWS_PROFILE` | `s3.profile` | AWS/S3 credentials profile |
| `PULUMI_SKIP_REFRESH` | — | Set to `true` to skip the Pulumi refresh step on each reconcile (faster, less safe) |
| `RUSI_ENABLED` | `rusi.enabled` | Enable RUSI sidecar for event publishing |
| `RUSI_GRPC_PORT` | — | RUSI sidecar gRPC port (when RUSI enabled) |
| `WORKERS_COUNT` | `workersCount` | Number of parallel reconcile workers |
| `MIGRATION_JOB_TTL_SECONDS_AFTER_FINISHED` | `migrationJobTTLSecondsAfterFinished` | TTL for completed migration jobs |

---

## Pulumi providers

The controller installs the following Pulumi plugins at startup:

| Provider | Version | Used for |
|----------|---------|---------|
| `azure-native` | v2.92.2 | Azure Resource Manager provisioning |
| `azuread` | v5.53.8 | Azure Entra ID (users, service principals) |
| `kubernetes` | v4.30.0 | Helm releases, Kubernetes resources |
| `vault` | v5.20.0 | HashiCorp Vault secret storage |
| `minio` | v0.16.9 | Minio / S3-compatible buckets |
| `keycloak` | v6.10.1 | Keycloak OpenID Connect clients |
| `mssql` | v0.0.8 | Microsoft SQL Server databases |
| `random` | v4.19.2 | Random value generation |
| `command` | v1.2.1 | Local script execution |

---

## RUSI events

When `RUSI_ENABLED=true` the controller publishes events to the following topics after each reconcile:

| Topic | Fired when |
|-------|-----------|
| `PlatformControllers.ProvisioningController.TenantProvisionedSuccessfully` | Tenant provisioning succeeded |
| `PlatformControllers.ProvisioningController.TenantProvisioningFailed` | Tenant provisioning failed |
| `PlatformControllers.ProvisioningController.PlatformProvisionedSuccessfully` | Platform provisioning succeeded |
| `PlatformControllers.ProvisioningController.PlatformProvisioningFailed` | Platform provisioning failed |
