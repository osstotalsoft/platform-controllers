package pulumi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// EnvPulumiImportAll, when true, makes every stack adopt its already existing stateful resources
// (the ones retained under DeletePolicyRetainStatefulResources: databases, buckets, logins, ...)
// instead of creating them — the disaster recovery path for a lost Pulumi state. A resource's own
// spec.import, when set, overrides it for that resource. Resources a stack already manages are
// never re-imported. A stateful resource that doesn't exist fails the reconcile: while recovering,
// every one of them is expected to be there.
var EnvPulumiImportAll = "PULUMI_IMPORT_ALL"

// Type tokens of the stateful resources the provisioner can import.
const (
	azureResourceGroupType        = "azure-native:resources:ResourceGroup"
	azureSqlDatabaseType          = "azure-native:sql:Database"
	azureSqlManagedDatabaseType   = "azure-native:sql:ManagedDatabase"
	azureUserAssignedIdentityType = "azure-native:managedidentity:UserAssignedIdentity"
	azureadGroupType              = "azuread:index/group:Group"
	azureadGroupMemberType        = "azuread:index/groupMember:GroupMember"
	minioBucketType               = "minio:index/s3Bucket:S3Bucket"
	mssqlDatabaseType             = "mssql:index/database:Database"
	mssqlSqlLoginType             = "mssql:index/sqlLogin:SqlLogin"
	mssqlSqlUserType              = "mssql:index/sqlUser:SqlUser"
	mssqlAzureadPrincipalType     = "mssql:index/azureadServicePrincipal:AzureadServicePrincipal"
)

// importOptions decides, per Pulumi resource, whether a deploy function should import an existing
// stateful resource rather than create a new one.
//
// Import is only ever attempted for resources missing from the stack's current state: an import ID
// that differs from an already-managed resource's ID (even just in casing) is treated by the Pulumi
// engine as an import-replacement, deleting and recreating the resource.
type importOptions struct {
	// enabled tells whether the resource at hand imports: the stack's global setting, or the
	// resource's own (see forResource).
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

// newImportOptions returns the global import setting for the stack provisioning infra.
func newImportOptions(infra *provisioning.InfrastructureManifests) *importOptions {
	enabled, err := strconv.ParseBool(os.Getenv(EnvPulumiImportAll))
	o := &importOptions{enabled: err == nil && enabled, stack: &stackImports{}}

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
	for _, r := range infra.AzureVirtualDesktops {
		metas = append(metas, r.GetProvisioningMeta())
	}
	return metas
}

// forResource returns the import options of the resource with the given provisioning meta: its own
// spec.import when set (with the tenant and tenant category overrides already applied to it),
// otherwise the global setting. Safe to call on a nil receiver.
func (o *importOptions) forResource(meta *provisioningv1.ProvisioningMeta) *importOptions {
	if o == nil || meta.Import == nil {
		return o
	}
	return &importOptions{enabled: *meta.Import, stack: o.stack}
}

// forAnyOf returns the import options of a resource shared by the resources with the given
// provisioning metas: imported when any of them is, or per the global setting when there are none.
func (o *importOptions) forAnyOf(metas []*provisioningv1.ProvisioningMeta) *importOptions {
	if o == nil || len(metas) == 0 {
		return o
	}
	shared := &importOptions{stack: o.stack}
	for _, meta := range metas {
		shared.enabled = shared.enabled || o.forResource(meta).enabled
	}
	return shared
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
