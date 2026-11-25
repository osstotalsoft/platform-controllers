package pulumi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	keyvault "github.com/pulumi/pulumi-azure-native-sdk/keyvault/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// EnvPulumiImportAll, when true, makes every stack adopt its already existing stateful resources
// (the ones retained under DeletePolicyRetainStatefulResources: databases, buckets, logins, ...)
// instead of creating them — the disaster recovery path for a lost Pulumi state. A tenant's
// spec.import, when set, overrides it for that tenant's stacks, and a resource's own spec.import
// overrides both for that resource. Resources a stack already manages are
// never re-imported. A stateful resource that doesn't exist fails the reconcile: while recovering,
// every one of them is expected to be there.
var EnvPulumiImportAll = "PULUMI_IMPORT_ALL"

// Type tokens of the stateful resources the provisioner can import.
const (
	azureResourceGroupType        = "azure-native:resources:ResourceGroup"
	azureSqlDatabaseType          = "azure-native:sql:Database"
	azureSqlManagedDatabaseType   = "azure-native:sql:ManagedDatabase"
	azureUserAssignedIdentityType = "azure-native:managedidentity:UserAssignedIdentity"
	azureBlobContainerType        = "azure-native:storage:BlobContainer"
	minioBucketType               = "minio:index/s3Bucket:S3Bucket"
	mssqlDatabaseType             = "mssql:index/database:Database"
	mssqlSqlLoginType             = "mssql:index/sqlLogin:SqlLogin"
	mssqlSqlUserType              = "mssql:index/sqlUser:SqlUser"
	mssqlAzureadPrincipalType     = "mssql:index/azureadServicePrincipal:AzureadServicePrincipal"

	// azureKeyVaultSecretType is not a stateful resource, but azure-native refuses to create a secret that already
	// exists, so the secrets toAzureKeyVault exports to are adopted too when recovering a lost state.
	azureKeyVaultSecretType = "azure-native:keyvault:Secret"
)

// importExistingAzureKeyVaultSecret returns the options adopting the vault's secret named secretName when it exists,
// and none when it doesn't: unlike a stateful resource, a secret may well be missing while recovering, e.g. when its
// export was added since.
//
// ARM never returns a secret's value, so the import has to ignore it, and the post-import update sets it. Ignoring
// just properties.value is not enough: the imported inputs also hold what ARM does return (properties.attributes, and
// the tags as they were), and any difference with the program's inputs makes the engine update the secret right after
// importing it, in the same update and with the value still ignored — a PUT without a value, which ARM rejects with
// 400 ParameterNotSpecified. So all of the properties and the tags are ignored for the import, leaving it nothing to
// update, and the post-import update applies them.
func importExistingAzureKeyVaultSecret(ctx *pulumi.Context, imports *importOptions, vaultResourceGroupName, vaultName, secretName string) ([]pulumi.ResourceOption, error) {
	existing, err := keyvault.LookupSecret(ctx, &keyvault.LookupSecretArgs{
		ResourceGroupName: vaultResourceGroupName,
		VaultName:         vaultName,
		SecretName:        secretName,
	})
	if isAzureNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot look up the Azure Key Vault secret %s to import: %w", secretName, err)
	}
	return imports.importResource(pulumi.ID(existing.Id), "properties", "tags"), nil
}

// isAzureNotFound reports whether err is an azure-native invoke failing because the resource doesn't exist. The
// provider reports ARM errors as text only, formatted by its PulumiAzcoreResponseError: `Status=404 Code="..." ...`.
func isAzureNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Status=404")
}

// importOptions decides, per Pulumi resource, whether a deploy function should import an existing
// stateful resource rather than create a new one.
//
// Import is only ever attempted for resources missing from the stack's current state: an import ID
// that differs from an already-managed resource's ID (even just in casing) is treated by the Pulumi
// engine as an import-replacement, deleting and recreating the resource.
type importOptions struct {
	// enabled tells whether the resource at hand imports: the stack's setting (see
	// newImportOptions), or the resource's own (see forResource).
	enabled bool
	stack   *stackImports
}

// stackImports is the import state shared by all of a stack's resources.
type stackImports struct {
	// anyEnabled tells whether any of the stack's resources may import, i.e. whether its state has
	// to be loaded.
	anyEnabled bool
	// managed holds managedResourceKey(type, name) of every resource in the stack's state.
	managed map[string]bool
	// driftIgnored is set when the stack's program imported a resource with some of its inputs
	// ignored, which only an update after the import can apply.
	driftIgnored atomic.Bool
}

// newImportOptions returns the import setting of the stack provisioning infra for target: the
// tenant's own when set, otherwise the global one.
func newImportOptions(target provisioning.ProvisioningTarget, infra *provisioning.InfrastructureManifests) *importOptions {
	global, err := strconv.ParseBool(os.Getenv(EnvPulumiImportAll))
	global = err == nil && global
	enabled := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) bool {
			if tenant.Spec.Import != nil {
				return *tenant.Spec.Import
			}
			return global
		},
		func(*platformv1.Platform) bool { return global },
	)
	o := &importOptions{enabled: enabled, stack: &stackImports{}}

	o.stack.anyEnabled = o.enabled
	for _, meta := range statefulResourceMetas(infra) {
		o.stack.anyEnabled = o.stack.anyEnabled || o.forResource(meta).enabled
	}
	return o
}

// statefulResourceMetas returns the provisioning meta of the stack's resources that honor
// spec.import.
func statefulResourceMetas(infra *provisioning.InfrastructureManifests) []*provisioningv1.ProvisioningMeta {
	var metas []*provisioningv1.ProvisioningMeta
	for _, r := range infra.AzureDbs {
		metas = append(metas, r.GetProvisioningMeta())
	}
	for _, r := range infra.AzureManagedDbs {
		metas = append(metas, r.GetProvisioningMeta())
	}
	for _, r := range infra.MsSqlDbs {
		metas = append(metas, r.GetProvisioningMeta())
	}
	for _, r := range infra.MinioBuckets {
		metas = append(metas, r.GetProvisioningMeta())
	}
	for _, r := range infra.AzureBlobContainers {
		metas = append(metas, r.GetProvisioningMeta())
	}
	return metas
}

// forResource returns the import options of the resource with the given provisioning meta: its own
// spec.import when set (with the tenant and tenant category overrides already applied to it),
// otherwise the stack's setting. Safe to call on a nil receiver.
func (o *importOptions) forResource(meta *provisioningv1.ProvisioningMeta) *importOptions {
	if o == nil || meta.Import == nil {
		return o
	}
	return &importOptions{enabled: *meta.Import, stack: o.stack}
}

// shouldImport reports whether the resource registered with typeToken and name — or formerly
// registered under one of aliases — should be imported. Safe to call on a nil receiver.
func (o *importOptions) shouldImport(typeToken, name string, aliases ...string) bool {
	if o == nil || !o.enabled {
		return false
	}
	for _, n := range append([]string{name}, aliases...) {
		if o.stack.managed[managedResourceKey(typeToken, n)] {
			return false
		}
	}
	return true
}

// importResource returns the options importing the existing resource with the given id.
// driftingInputs lists inputs that may legitimately differ from the existing resource (e.g. a
// regenerated password): Pulumi refuses to import a resource whose inputs don't match, so they are
// ignored for the import only, and the post-import update in updateStack applies them.
func (o *importOptions) importResource(id pulumi.IDInput, driftingInputs ...string) []pulumi.ResourceOption {
	opts := []pulumi.ResourceOption{pulumi.Import(id)}
	if len(driftingInputs) > 0 {
		opts = append(opts, pulumi.IgnoreChanges(driftingInputs))
		o.stack.driftIgnored.Store(true)
	}
	return opts
}

// loadManagedResources records the resources currently in the stack's state. Must run before each
// of the stack's updates, after any refresh.
func (o *importOptions) loadManagedResources(ctx context.Context, s auto.Stack) error {
	if o == nil || !o.stack.anyEnabled {
		return nil
	}

	exported, err := s.Export(ctx)
	if err != nil {
		return fmt.Errorf("failed to export stack state: %w", err)
	}

	o.stack.managed, err = managedResources(exported)
	return err
}

// managedResources returns managedResourceKey(type, name) of every resource in an exported stack
// state.
func managedResources(exported apitype.UntypedDeployment) (map[string]bool, error) {
	managed := map[string]bool{}
	if len(exported.Deployment) == 0 {
		return managed, nil
	}

	// V4 deployments only add optional features on top of V3; resources keep the V3 shape.
	var deployment apitype.DeploymentV3
	if err := json.Unmarshal(exported.Deployment, &deployment); err != nil {
		return nil, fmt.Errorf("failed to read stack state: %w", err)
	}
	for _, res := range deployment.Resources {
		managed[managedResourceKey(string(res.Type), res.URN.Name())] = true
	}
	return managed, nil
}

// managedResourceKey identifies a resource by its own type token and name, ignoring its parent
// chain, which is unique enough within one stack.
func managedResourceKey(typeToken, name string) string {
	return typeToken + "::" + name
}

// lookedUpImportId turns the id a lookup found for the resource described by description into an
// import ID, failing when the lookup found nothing.
func lookedUpImportId(id pulumi.StringOutput, description string) pulumi.IDOutput {
	return id.ApplyT(func(id string) (pulumi.ID, error) {
		if id == "" {
			return "", fmt.Errorf("%s to import not found", description)
		}
		return pulumi.ID(id), nil
	}).(pulumi.IDOutput)
}

// azureSubscriptionId returns the subscription the azure-native provider deploys into, for building
// the ARM IDs of resources to import.
func azureSubscriptionId(ctx *pulumi.Context) (string, error) {
	config, err := authorization.GetClientConfig(ctx)
	if err != nil {
		return "", err
	}
	return config.SubscriptionId, nil
}

// needsPostImportUpdate reports, after an update, whether it imported resources with inputs it had
// to ignore, and resets that for the next update. Safe to call on a nil receiver.
func (o *importOptions) needsPostImportUpdate() bool {
	return o != nil && o.stack.driftIgnored.Swap(false)
}
