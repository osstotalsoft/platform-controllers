package pulumi

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/managedidentity/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// blobContainerRoleDefinitionIds maps the roles a managed identity can get on a container to the ids
// of these built-in Azure role definitions, the same in every subscription.
var blobContainerRoleDefinitionIds = map[provisioningv1.BlobContainerRole]string{
	provisioningv1.BlobContainerRoleContributor: "ba92f5b4-2d11-453d-a403-e96b0029c9fe",
	provisioningv1.BlobContainerRoleReader:      "2a2b9908-6ea1-4ae2-8e65-a410df84e7d1",
	provisioningv1.BlobContainerRoleOwner:       "b7e6dc6d-f1e8-4753-8033-0f276bb0955b",
}

// blobContainerNamePattern is Azure's naming rule for blob containers: 3-63 lowercase letters,
// digits and hyphens, starting and ending with a letter or digit, without consecutive hyphens.
var blobContainerNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// blobContainerSasPermissions are the permissions of the exported container SAS: read, add, create,
// write, delete and list, what an app storing its files in the container needs.
const blobContainerSasPermissions = "racwdl"

// blobContainerSasRotationDays is how often a new container SAS is issued. Within one rotation
// window the SAS, and so the exported value, stays the same from one update to the next.
const blobContainerSasRotationDays = 30

// defaultBlobContainerSasValidityDays is the validity of the container SAS when spec.sasValidityDays
// is not set.
const defaultBlobContainerSasValidityDays = 365

// blobContainerNow is the clock the container SAS expiry is computed from, replaced by tests.
var blobContainerNow = time.Now

func deployAzureBlobContainer(target provisioning.ProvisioningTarget,
	blobContainer *provisioningv1.AzureBlobContainer,
	dependencies []pulumi.Resource,
	imports *importOptions,
	ctx *pulumi.Context) (*storage.BlobContainer, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureBlobContainer")
	spec := blobContainer.Spec

	containerName := blobContainerName(target, spec.ContainerName)
	if spec.ImportContainerName != "" {
		containerName = spec.ImportContainerName
	} else if err := validateBlobContainerName(containerName); err != nil {
		return nil, err
	}

	sasValidityDays := spec.SasValidityDays
	if sasValidityDays == 0 {
		sasValidityDays = defaultBlobContainerSasValidityDays
	}
	if sasValidityDays <= blobContainerSasRotationDays {
		return nil, fmt.Errorf("spec.sasValidityDays must be more than %d, the SAS rotation period, got %d", blobContainerSasRotationDays, sasValidityDays)
	}

	if err := validateUniqueNames(spec.ManagedIdentities, "managedIdentities"); err != nil {
		return nil, err
	}
	identities, err := renderBlobContainerIdentities(spec.ManagedIdentities, target)
	if err != nil {
		return nil, err
	}
	if err := validateServiceAccounts(blobIdentitiesAsManagedIdentities(identities), blobContainer.Namespace); err != nil {
		return nil, err
	}
	for _, identity := range identities {
		if _, ok := blobContainerRoleDefinitionIds[blobContainerRole(identity)]; !ok {
			return nil, fmt.Errorf("spec.managedIdentities[%s].role %q is not one of StorageBlobDataContributor, StorageBlobDataReader, StorageBlobDataOwner", identity.Name, identity.Role)
		}
	}
	exportDomains, err := blobContainerExportDomains(blobContainer)
	if err != nil {
		return nil, err
	}

	account, err := storage.LookupStorageAccount(ctx, &storage.LookupStorageAccountArgs{
		ResourceGroupName: spec.StorageAccount.ResourceGroupName,
		AccountName:       spec.StorageAccount.AccountName,
	})
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("storageAccount %s not found", spec.StorageAccount.AccountName)
	}
	subscriptionId, err := subscriptionIdOf(account.Id)
	if err != nil {
		return nil, err
	}

	pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources

	importContainerId := ""
	if spec.ImportContainerName != "" || imports.shouldImport(azureBlobContainerType, blobContainer.Name) {
		importContainerId = fmt.Sprintf("%s/blobServices/default/containers/%s", account.Id, containerName)
	}

	container, err := storage.NewBlobContainer(ctx, blobContainer.Name, &storage.BlobContainerArgs{
		ResourceGroupName: pulumi.String(spec.StorageAccount.ResourceGroupName),
		AccountName:       pulumi.String(spec.StorageAccount.AccountName),
		ContainerName:     pulumi.String(containerName),
	},
		pulumi.RetainOnDelete(pulumiRetainOnDelete),
		pulumi.IgnoreChanges([]string{"resourceGroupName", "accountName", "containerName"}),
		pulumi.Import(pulumi.ID(importContainerId)),
		pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	identitiesByName := map[string]deployedIdentity{}
	for i := range identities {
		identity := identities[i]
		clientId, principalId, err := deployBlobContainerIdentity(ctx, fmt.Sprintf("%s-%s-blob", blobContainer.Name, identity.Name),
			&identity, containerName, subscriptionId, blobContainer.Namespace, container, dependencies, pulumiRetainOnDelete, imports)
		if err != nil {
			return nil, err
		}
		identitiesByName[identity.Name] = deployedIdentity{clientId: clientId, principalId: principalId}
	}

	blobEndpoint := account.PrimaryEndpoints.Blob
	containerUrl := strings.TrimSuffix(blobEndpoint, "/") + "/" + containerName

	var accountKey, connectionString, containerSasUrl pulumi.StringOutput
	if anyBlobContainerExport(spec.Exports, func(e provisioningv1.AzureBlobContainerExportsSpec) provisioningv1.ValueExport { return e.AccountKey }) ||
		anyBlobContainerExport(spec.Exports, func(e provisioningv1.AzureBlobContainerExportsSpec) provisioningv1.ValueExport { return e.ConnectionString }) {
		accountKey = storageAccountPrimaryKey(ctx, spec.StorageAccount)
		connectionString = pulumi.ToSecret(accountKey.ApplyT(func(key string) string {
			return storageConnectionString(spec.StorageAccount.AccountName, key, blobEndpoint)
		})).(pulumi.StringOutput)
	}
	if anyBlobContainerExport(spec.Exports, func(e provisioningv1.AzureBlobContainerExportsSpec) provisioningv1.ValueExport { return e.ContainerSasUrl }) {
		sas := storage.ListStorageAccountServiceSASOutput(ctx, storage.ListStorageAccountServiceSASOutputArgs{
			ResourceGroupName:      pulumi.String(spec.StorageAccount.ResourceGroupName),
			AccountName:            pulumi.String(spec.StorageAccount.AccountName),
			CanonicalizedResource:  pulumi.String(fmt.Sprintf("/blob/%s/%s", spec.StorageAccount.AccountName, containerName)),
			Resource:               pulumi.StringPtr("c"),
			Permissions:            pulumi.StringPtr(blobContainerSasPermissions),
			Protocols:              storage.HttpProtocolHttps.ToHttpProtocolPtrOutput(),
			SharedAccessExpiryTime: pulumi.StringPtr(blobContainerSasExpiry(blobContainerNow(), sasValidityDays)),
		})
		containerSasUrl = pulumi.ToSecret(sas.ServiceSasToken().ApplyT(func(token string) string {
			return containerUrl + "?" + token
		})).(pulumi.StringOutput)
	}

	for i, exp := range spec.Exports {
		domain := exportDomains[i]
		values := map[string]exportTemplateWithValue{
			"containerName":    {exp.ContainerName, pulumi.String(containerName)},
			"accountName":      {exp.AccountName, pulumi.String(spec.StorageAccount.AccountName)},
			"blobEndpoint":     {exp.BlobEndpoint, pulumi.String(blobEndpoint)},
			"containerUrl":     {exp.ContainerUrl, pulumi.String(containerUrl)},
			"containerSasUrl":  {exp.ContainerSasUrl, containerSasUrl},
			"accountKey":       {exp.AccountKey, accountKey},
			"connectionString": {exp.ConnectionString, connectionString},
		}
		if err := addIdentityExports(values, identitiesByName, identityExports{identities: exp.Identities}, domain); err != nil {
			return nil, err
		}
		if len(exp.Identities) > 0 {
			if err := validateExportKeys(values, provisioning.GetTemplateContext(target), domain); err != nil {
				return nil, err
			}
		}

		err = valueExporter(newExportContext(ctx, domain, blobContainer.Name, blobContainer.ObjectMeta, gvk), values)
		if err != nil {
			return nil, err
		}
	}
	return container, nil
}

// deployBlobContainerIdentity creates an Azure user-assigned managed identity with a federated
// credential for each of its ServiceAccounts, and grants it its role on container.
//
// Like the database identities (see deployManagedIdentity), the identity lives in a fixed resource
// group shared by every tenant, so its Azure name gets the tenant-scoped container name appended.
// The identity is retained with the container; the role assignment, like the federated credentials,
// holds no state and is not: a retained identity must not keep access once its owner is gone.
func deployBlobContainerIdentity(ctx *pulumi.Context, resourceNamePrefix string,
	identitySpec *provisioningv1.BlobContainerIdentitySpec, containerName, subscriptionId, ownerNamespace string,
	container *storage.BlobContainer, dependencies []pulumi.Resource, retainOnDelete bool, imports *importOptions) (pulumi.StringOutput, pulumi.StringOutput, error) {

	name := fmt.Sprintf("%s_%s", identitySpec.Name, containerName)

	identityResourceName := fmt.Sprintf("%s-identity", resourceNamePrefix)
	identityOpts := []pulumi.ResourceOption{pulumi.DependsOn(dependencies), pulumi.RetainOnDelete(retainOnDelete)}
	if imports.shouldImport(azureUserAssignedIdentityType, identityResourceName) {
		identityOpts = append(identityOpts, imports.importResource(pulumi.ID(fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s",
			subscriptionId, identitySpec.ResourceGroupName, name)))...)
	}

	identity, err := managedidentity.NewUserAssignedIdentity(ctx, identityResourceName, &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String(name),
		ResourceGroupName: pulumi.String(identitySpec.ResourceGroupName),
		Location:          pulumi.String(identitySpec.Location),
	}, identityOpts...)
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	managedIdentitySpec := blobIdentityAsManagedIdentity(*identitySpec)
	if err := deployFederatedCredentials(ctx, identity, &managedIdentitySpec, resourceNamePrefix, ownerNamespace); err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-role-assignment", resourceNamePrefix), &authorization.RoleAssignmentArgs{
		PrincipalId: identity.PrincipalId,
		// A new identity takes a while to replicate in Entra ID: with the principal type given, Azure
		// doesn't need to look it up and accepts the assignment right away.
		PrincipalType: pulumi.String(authorization.PrincipalTypeServicePrincipal),
		RoleDefinitionId: pulumi.String(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s",
			subscriptionId, blobContainerRoleDefinitionIds[blobContainerRole(*identitySpec)])),
		Scope: container.ID(),
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	return identity.ClientId, identity.PrincipalId, nil
}

// blobContainerName is the tenant (or platform) scoped name of the container: the prefix, then the
// platform and tenant names, with dots and underscores, which container names can't contain, turned
// into hyphens.
func blobContainerName(target provisioning.ProvisioningTarget, prefix string) string {
	name := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s-%s", prefix, tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s-%s", prefix, platform.GetName())
		},
	)
	return strings.NewReplacer(".", "-", "_", "-").Replace(strings.ToLower(name))
}

func validateBlobContainerName(name string) error {
	if len(name) < 3 || len(name) > 63 || !blobContainerNamePattern.MatchString(name) {
		return fmt.Errorf("container name %q must be 3-63 lowercase letters, digits and single hyphens, starting and ending with a letter or digit; shorten spec.containerName", name)
	}
	return nil
}

// blobContainerSasExpiry is the expiry of the container SAS issued at now: validityDays after the
// start of now's rotation window. Every update within one window asks for the same SAS, so the
// exported value only changes once per window, and the previous SAS stays valid for
// validityDays - blobContainerSasRotationDays days after it's replaced.
func blobContainerSasExpiry(now time.Time, validityDays int) string {
	day := now.UTC().Unix() / 86400
	windowStart := day - day%blobContainerSasRotationDays
	return time.Unix((windowStart+int64(validityDays))*86400, 0).UTC().Format(time.RFC3339)
}

// storageAccountPrimaryKey returns the storage account's first access key, as a secret.
func storageAccountPrimaryKey(ctx *pulumi.Context, account provisioningv1.StorageAccountSpec) pulumi.StringOutput {
	keys := storage.ListStorageAccountKeysOutput(ctx, storage.ListStorageAccountKeysOutputArgs{
		ResourceGroupName: pulumi.String(account.ResourceGroupName),
		AccountName:       pulumi.String(account.AccountName),
	})
	return pulumi.ToSecret(keys.ApplyT(func(r storage.ListStorageAccountKeysResult) (string, error) {
		if len(r.Keys) == 0 || r.Keys[0].Value == "" {
			return "", fmt.Errorf("storage account %s has no access key; is shared key access disabled?", account.AccountName)
		}
		return r.Keys[0].Value, nil
	})).(pulumi.StringOutput)
}

// storageConnectionString builds the account's connection string. A blob endpoint in the public
// cloud's (or a sovereign cloud's) standard <account>.blob.<suffix> form becomes an EndpointSuffix,
// which gives the clients the account's other services too; any other endpoint is given as is.
func storageConnectionString(accountName, accountKey, blobEndpoint string) string {
	prefix := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s", accountName, accountKey)
	if u, err := url.Parse(blobEndpoint); err == nil {
		if suffix, ok := strings.CutPrefix(u.Host, accountName+".blob."); ok && suffix != "" {
			return fmt.Sprintf("%s;EndpointSuffix=%s", prefix, suffix)
		}
	}
	return fmt.Sprintf("%s;BlobEndpoint=%s", prefix, blobEndpoint)
}

// subscriptionIdOf returns the subscription in an ARM resource id.
func subscriptionIdOf(resourceId string) (string, error) {
	parts := strings.Split(resourceId, "/")
	if len(parts) < 3 || !strings.EqualFold(parts[1], "subscriptions") || parts[2] == "" {
		return "", fmt.Errorf("no subscription in resource id %q", resourceId)
	}
	return parts[2], nil
}

func anyBlobContainerExport(exports []provisioningv1.AzureBlobContainerExportsSpec,
	field func(provisioningv1.AzureBlobContainerExportsSpec) provisioningv1.ValueExport) bool {
	for _, e := range exports {
		if field(e) != (provisioningv1.ValueExport{}) {
			return true
		}
	}
	return false
}

// blobContainerExportDomains returns the domain of each exports[] entry, domainRef when not set,
// and fails when two entries export to the same domain, where they would overwrite each other.
func blobContainerExportDomains(blobContainer *provisioningv1.AzureBlobContainer) ([]string, error) {
	domains := make([]string, len(blobContainer.Spec.Exports))
	seen := map[string]bool{}
	for i, exp := range blobContainer.Spec.Exports {
		domain := exp.Domain
		if domain == "" {
			domain = blobContainer.Spec.DomainRef
		}
		if seen[domain] {
			return nil, fmt.Errorf("spec.exports[].domain %q is duplicated — each domain must appear at most once, since export secret/configmap names and vault paths are keyed by domain alone", domain)
		}
		seen[domain] = true
		domains[i] = domain
	}
	return domains, nil
}

func blobContainerRole(identity provisioningv1.BlobContainerIdentitySpec) provisioningv1.BlobContainerRole {
	if identity.Role == "" {
		return provisioningv1.BlobContainerRoleContributor
	}
	return identity.Role
}

// renderBlobContainerIdentities is renderServiceAccounts for the container's identities.
func renderBlobContainerIdentities(identities []provisioningv1.BlobContainerIdentitySpec, target provisioning.ProvisioningTarget) ([]provisioningv1.BlobContainerIdentitySpec, error) {
	rendered, err := renderServiceAccounts(blobIdentitiesAsManagedIdentities(identities), target)
	if err != nil {
		return nil, err
	}
	result := make([]provisioningv1.BlobContainerIdentitySpec, len(identities))
	for i, identity := range identities {
		identity.ServiceAccounts = rendered[i].ServiceAccounts
		result[i] = identity
	}
	return result, nil
}

// blobIdentityAsManagedIdentity returns the parts of identity the shared managed identity helpers
// (renderServiceAccounts, validateServiceAccounts, deployFederatedCredentials) use.
func blobIdentityAsManagedIdentity(identity provisioningv1.BlobContainerIdentitySpec) provisioningv1.ManagedIdentitySpec {
	return provisioningv1.ManagedIdentitySpec{
		Name:              identity.Name,
		ResourceGroupName: identity.ResourceGroupName,
		Location:          identity.Location,
		ServiceAccounts:   identity.ServiceAccounts,
	}
}

func blobIdentitiesAsManagedIdentities(identities []provisioningv1.BlobContainerIdentitySpec) []provisioningv1.ManagedIdentitySpec {
	result := make([]provisioningv1.ManagedIdentitySpec, len(identities))
	for i, identity := range identities {
		result[i] = blobIdentityAsManagedIdentity(identity)
	}
	return result
}
