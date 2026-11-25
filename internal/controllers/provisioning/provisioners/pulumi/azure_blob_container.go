package pulumi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/managedidentity/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiTime "github.com/pulumiverse/pulumi-time/sdk/go/time"
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

// storageBlobDelegatorRoleDefinitionId is the id of the built-in Storage Blob Delegator role: it only
// allows requesting a user delegation key, to sign SAS (e.g. download links) as the identity.
const storageBlobDelegatorRoleDefinitionId = "db58b8e5-c6ad-4a2a-8342-4190687cbf4a"

// blobContainerNamePattern is Azure's naming rule for blob containers: 3-63 lowercase letters,
// digits and hyphens, starting and ending with a letter or digit, without consecutive hyphens.
var blobContainerNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// blobContainerSasPermissions are the permissions of the exported container SAS: read, add, create,
// write, delete and list, what an app storing its files in the container needs. They are also every
// permission a sasTokens[] entry can have, in the order Azure requires them in a SAS.
const blobContainerSasPermissions = "racwdl"

// blobContainerSasRotationDays is how often a new container SAS is issued. Between rotations the
// SAS, and so the exported value, stays the same from one update to the next.
const blobContainerSasRotationDays = 30

// defaultBlobContainerSasValidityDays is the validity of the container SAS when spec.sasValidityDays
// is not set.
const defaultBlobContainerSasValidityDays = 365

func deployAzureBlobContainer(target provisioning.ProvisioningTarget,
	blobContainer *provisioningv1.AzureBlobContainer,
	dependencies []pulumi.Resource,
	imports *importOptions,
	ctx *pulumi.Context) (*storage.BlobContainer, error) {

	valueExporter := handleValueExport(target, imports)
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
	sasTokens, err := blobContainerSasTokens(spec.SasTokens, sasValidityDays)
	if err != nil {
		return nil, err
	}
	exportedSasTokens, err := blobContainerSasTokenExports(spec.Exports, exportDomains, sasTokens)
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
			&identity, containerName, subscriptionId, account.Id, blobContainer.Namespace, container, dependencies, pulumiRetainOnDelete, imports)
		if err != nil {
			return nil, err
		}
		identitiesByName[identity.Name] = deployedIdentity{clientId: clientId, principalId: principalId}
	}

	blobEndpoint := account.PrimaryEndpoints.Blob
	containerUrl := strings.TrimSuffix(blobEndpoint, "/") + "/" + containerName

	// Only container scoped credentials are exported, never the account keys, which grant access to
	// every container of the account. Only the SAS some exports[] entry exports are signed, all from
	// the same rotation.
	var containerSasUrl pulumi.StringOutput
	sasTokenUrls := map[string]pulumi.StringOutput{}
	if sasTokenNames := exportedSasTokenNames(exportedSasTokens); anyContainerSasUrlExport(spec.Exports) || len(sasTokenNames) > 0 {
		sasStart, err := deployBlobContainerSasRotation(ctx, blobContainer.Name)
		if err != nil {
			return nil, err
		}
		sign := func(sas blobContainerSas) pulumi.StringOutput {
			return blobContainerSasUrl(ctx, spec.StorageAccount, containerName, containerUrl, sas, sasStart)
		}
		if anyContainerSasUrlExport(spec.Exports) {
			containerSasUrl = sign(blobContainerSas{permissions: blobContainerSasPermissions, validityDays: sasValidityDays})
		}
		for _, name := range sasTokenNames {
			sasTokenUrls[name] = sign(sasTokens[name])
		}
	}

	for i, exp := range spec.Exports {
		domain := exportDomains[i]
		values := map[string]exportTemplateWithValue{
			"containerName":   {exp.ContainerName, pulumi.String(containerName)},
			"accountName":     {exp.AccountName, pulumi.String(spec.StorageAccount.AccountName)},
			"blobEndpoint":    {exp.BlobEndpoint, pulumi.String(blobEndpoint)},
			"containerUrl":    {exp.ContainerUrl, pulumi.String(containerUrl)},
			"containerSasUrl": {exp.ContainerSasUrl, containerSasUrl},
		}
		if err := addIdentityExports(values, identitiesByName, identityExports{identities: exp.Identities}, domain); err != nil {
			return nil, err
		}
		for j, name := range exportedSasTokens[i] {
			values[fmt.Sprintf("sasTokens[%d].containerSasUrl", j)] = exportTemplateWithValue{exp.SasTokens[j].ContainerSasUrl, sasTokenUrls[name]}
		}
		if len(exp.Identities) > 0 || len(exp.SasTokens) > 0 {
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
// credential for each of its ServiceAccounts, grants it its role on container, and Storage Blob
// Delegator on the storage account accountId.
//
// The delegator role lets the identity sign SAS, such as short-lived download links, with a user
// delegation key. Azure only issues that key to a principal with the role on the storage account (or
// above), not on a container, but the role grants no data access: a SAS the identity signs can't
// exceed its own role on the container.
//
// Like the database identities (see deployManagedIdentity), the identity lives in a fixed resource
// group shared by every tenant, so its Azure name gets the tenant-scoped container name appended.
// The identity is retained with the container; the role assignments, like the federated credentials,
// hold no state and are not: a retained identity must not keep access once its owner is gone.
func deployBlobContainerIdentity(ctx *pulumi.Context, resourceNamePrefix string,
	identitySpec *provisioningv1.BlobContainerIdentitySpec, containerName, subscriptionId, accountId, ownerNamespace string,
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

	roleAssignments := []struct {
		name             string
		roleDefinitionId string
		scope            pulumi.StringInput
	}{
		{"role-assignment", blobContainerRoleDefinitionIds[blobContainerRole(*identitySpec)], container.ID().ToStringOutput()},
		{"delegator-role-assignment", storageBlobDelegatorRoleDefinitionId, pulumi.String(accountId)},
	}
	for _, ra := range roleAssignments {
		_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-%s", resourceNamePrefix, ra.name), &authorization.RoleAssignmentArgs{
			PrincipalId: identity.PrincipalId,
			// A new identity takes a while to replicate in Entra ID: with the principal type given, Azure
			// doesn't need to look it up and accepts the assignment right away.
			PrincipalType: pulumi.String(authorization.PrincipalTypeServicePrincipal),
			RoleDefinitionId: pulumi.String(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s",
				subscriptionId, ra.roleDefinitionId)),
			Scope: ra.scope,
		}, pulumi.DependsOn(dependencies))
		if err != nil {
			return pulumi.StringOutput{}, pulumi.StringOutput{}, err
		}
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

// blobContainerSas is a container SAS to sign: its permissions, in the order Azure requires, and how
// many days each SAS issued is valid.
type blobContainerSas struct {
	permissions  string
	validityDays int
}

// deployBlobContainerSasRotation returns the start of the container SAS: the time of the last
// rotation, kept in the stack state by a pulumi-time Rotating resource. It stays the same from one
// update to the next, so the exported SAS only change when the Rotating resource is replaced,
// blobContainerSasRotationDays after the previous rotation. A previous SAS then stays valid for its
// validityDays - blobContainerSasRotationDays more days.
//
// The rotation is detected by the stack refresh, like the AVD registration token's: with
// PULUMI_SKIP_REFRESH the SAS are never renewed.
func deployBlobContainerSasRotation(ctx *pulumi.Context, name string) (pulumi.StringOutput, error) {
	rotation, err := pulumiTime.NewRotating(ctx, fmt.Sprintf("%s-sas-rotation", name), &pulumiTime.RotatingArgs{
		RotationDays: pulumi.Int(blobContainerSasRotationDays),
	})
	if err != nil {
		return pulumi.StringOutput{}, err
	}
	return rotation.Rfc3339, nil
}

// blobContainerSasUrl signs sas for the container with the storage account key, valid from start for
// sas.validityDays, and returns the container URL with it, as a secret.
func blobContainerSasUrl(ctx *pulumi.Context, account provisioningv1.StorageAccountSpec, containerName, containerUrl string,
	sas blobContainerSas, start pulumi.StringOutput) pulumi.StringOutput {

	expiry := start.ApplyT(func(start string) (string, error) {
		t, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return "", fmt.Errorf("invalid SAS rotation time %q: %w", start, err)
		}
		return t.AddDate(0, 0, sas.validityDays).UTC().Format(time.RFC3339), nil
	}).(pulumi.StringOutput)

	token := storage.ListStorageAccountServiceSASOutput(ctx, storage.ListStorageAccountServiceSASOutputArgs{
		ResourceGroupName:     pulumi.String(account.ResourceGroupName),
		AccountName:           pulumi.String(account.AccountName),
		CanonicalizedResource: pulumi.String(fmt.Sprintf("/blob/%s/%s", account.AccountName, containerName)),
		Resource:              pulumi.StringPtr("c"),
		Permissions:           pulumi.StringPtr(sas.permissions),
		Protocols:             storage.HttpProtocolHttps.ToHttpProtocolPtrOutput(),
		// Both times stay the same until the next rotation: without a start time Azure signs the SAS
		// from the current time, and every update would export a new SAS.
		SharedAccessStartTime:  start,
		SharedAccessExpiryTime: expiry,
	}).ServiceSasToken()

	return pulumi.ToSecret(token.ApplyT(func(token string) string {
		return containerUrl + "?" + token
	})).(pulumi.StringOutput)
}

// blobContainerSasTokens validates spec.sasTokens and returns its entries by name, their permissions
// in the order Azure requires and their validity defaulted to defaultValidityDays.
func blobContainerSasTokens(specs []provisioningv1.BlobContainerSasTokenSpec, defaultValidityDays int) (map[string]blobContainerSas, error) {
	if err := validateUniqueNames(specs, "sasTokens"); err != nil {
		return nil, err
	}
	sasTokens := make(map[string]blobContainerSas, len(specs))
	for _, s := range specs {
		permissions, err := canonicalBlobContainerSasPermissions(s.Permissions)
		if err != nil {
			return nil, fmt.Errorf("spec.sasTokens[%s].permissions: %w", s.Name, err)
		}
		validityDays := s.ValidityDays
		if validityDays == 0 {
			validityDays = defaultValidityDays
		}
		if validityDays <= blobContainerSasRotationDays {
			return nil, fmt.Errorf("spec.sasTokens[%s].validityDays must be more than %d, the SAS rotation period, got %d", s.Name, blobContainerSasRotationDays, validityDays)
		}
		sasTokens[s.Name] = blobContainerSas{permissions: permissions, validityDays: validityDays}
	}
	return sasTokens, nil
}

// canonicalBlobContainerSasPermissions returns permissions in the order Azure requires them in a SAS,
// failing when they are empty, repeat a letter or have one other than blobContainerSasPermissions'.
func canonicalBlobContainerSasPermissions(permissions string) (string, error) {
	if permissions == "" {
		return "", fmt.Errorf("at least one of %q is required", blobContainerSasPermissions)
	}
	given := map[rune]bool{}
	for _, p := range permissions {
		if !strings.ContainsRune(blobContainerSasPermissions, p) {
			return "", fmt.Errorf("%q is not one of %q", p, blobContainerSasPermissions)
		}
		if given[p] {
			return "", fmt.Errorf("%q is repeated", p)
		}
		given[p] = true
	}
	var canonical strings.Builder
	for _, p := range blobContainerSasPermissions {
		if given[p] {
			canonical.WriteRune(p)
		}
	}
	return canonical.String(), nil
}

// blobContainerSasTokenExports returns, for each exports[] entry, the sasTokens[] entry each of its
// sasTokens items exports. An item must export containerSasUrl, and an exports[] entry can export each
// sasTokens[] entry at most once.
func blobContainerSasTokenExports(exports []provisioningv1.AzureBlobContainerExportsSpec, domains []string, sasTokens map[string]blobContainerSas) ([][]string, error) {
	names := make([][]string, len(exports))
	for i, exp := range exports {
		exported := map[string]bool{}
		for j, t := range exp.SasTokens {
			if t.ContainerSasUrl == (provisioningv1.ValueExport{}) {
				return nil, fmt.Errorf("exports[domain=%s]: sasTokens[%d] must export containerSasUrl", domains[i], j)
			}
			name, err := resolveRefName(sasTokens, t.SasTokenRef, domains[i], "sasTokens[].sasTokenRef", "sasTokens")
			if err != nil {
				return nil, err
			}
			if exported[name] {
				return nil, fmt.Errorf("exports[domain=%s]: sasTokens[%d] exports spec.sasTokens[].name %q, which this exports[] entry already exports", domains[i], j, name)
			}
			exported[name] = true
			names[i] = append(names[i], name)
		}
	}
	return names, nil
}

// exportedSasTokenNames returns the sasTokens[] entries some exports[] entry exports, sorted.
func exportedSasTokenNames(exportedSasTokens [][]string) []string {
	seen := map[string]bool{}
	var names []string
	for _, entryNames := range exportedSasTokens {
		for _, name := range entryNames {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// subscriptionIdOf returns the subscription in an ARM resource id.
func subscriptionIdOf(resourceId string) (string, error) {
	parts := strings.Split(resourceId, "/")
	if len(parts) < 3 || !strings.EqualFold(parts[1], "subscriptions") || parts[2] == "" {
		return "", fmt.Errorf("no subscription in resource id %q", resourceId)
	}
	return parts[2], nil
}

func anyContainerSasUrlExport(exports []provisioningv1.AzureBlobContainerExportsSpec) bool {
	for _, e := range exports {
		if e.ContainerSasUrl != (provisioningv1.ValueExport{}) {
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
