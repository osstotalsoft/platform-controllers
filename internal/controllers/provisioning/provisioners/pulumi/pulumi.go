// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	"os"

	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

var (
	EnvPulumiSkipRefresh = "PULUMI_SKIP_REFRESH"
	EnvAzureEnabled      = "AZURE_ENABLED"
)

type provisionedResourceMap = map[provisioningv1.ProvisioningResourceIdendtifier]pulumi.Resource

func Create(target provisioning.ProvisioningTarget, domain string, infra *provisioning.InfrastructureManifests) provisioning.ProvisioningResult {
	result := provisioning.ProvisioningResult{}
	upRes := auto.UpResult{}
	destroyRes := auto.DestroyResult{}
	emptyDeployFunc := func(ctx *pulumi.Context) error { return nil }

	anyAzureDb := len(infra.AzureDbs) > 0
	anyManagedAzureDb := len(infra.AzureManagedDbs) > 0
	anyAzurePowerShellScript := len(infra.AzurePowerShellScripts) > 0
	anyHelmRelease := len(infra.HelmReleases) > 0
	anyVirtualMachine := len(infra.AzureVirtualMachines) > 0
	anyVirtualDesktop := len(infra.AzureVirtualDesktops) > 0
	anyEntraUser := len(infra.EntraUsers) > 0
	anyMssqlDb := len(infra.MsSqlDbs) > 0
	anyLocalScript := len(infra.LocalScripts) > 0

	anyResource := anyAzureDb || anyManagedAzureDb || anyHelmRelease || anyVirtualMachine || anyVirtualDesktop || anyEntraUser || anyAzurePowerShellScript || anyMssqlDb || anyLocalScript
	needsResourceGroup := anyVirtualMachine || anyVirtualDesktop || anyAzurePowerShellScript

	stackName := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s", tenant.GetName(), domain)
		},
		func(*platformv1.Platform) string {
			return fmt.Sprintf("platform-%s", domain)
		},
	)

	if anyResource {
		upRes, result.Error = updateStack(stackName, target.GetPlatformName(), deployFunc(target, domain, infra, needsResourceGroup))
		if result.Error != nil {
			return result
		}
		result.HasChanges = hasChanges(upRes.Summary)
	} else {
		destroyRes, result.Error = tryDestroyAndDeleteStack(stackName, target.GetPlatformName(), emptyDeployFunc)
		if result.Error != nil {
			return result
		}
		result.HasChanges = hasChanges(destroyRes.Summary)
	}

	return result
}

func hasChanges(summary auto.UpdateSummary) bool {
	if summary.ResourceChanges == nil {
		return false
	}
	for key := range *summary.ResourceChanges {
		if apitype.OpType(key) != apitype.OpSame {
			return true
		}
	}
	return false
}

func updateStack(stackName, projectName string, deployFunc pulumi.RunFunc) (auto.UpResult, error) {
	ctx := context.Background()

	s, err := createOrSelectStack(ctx, stackName, projectName, deployFunc)
	if err != nil {
		klog.ErrorS(err, "Failed to create or select stack", "name", stackName)
		return auto.UpResult{}, err
	}
	klog.V(4).InfoS("Starting stack update", "name", stackName)

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		klog.ErrorS(err, "Failed to update stack", "name", stackName)
		return auto.UpResult{}, err
	}
	klog.V(4).InfoS("Stack update succeeded!", "name", stackName)
	klog.V(4).InfoS("Stack results", "name", stackName, "Outputs", res.Outputs)

	return res, err
}

func tryDestroyAndDeleteStack(stackName, projectName string, deployFunc pulumi.RunFunc) (auto.DestroyResult, error) {
	ctx := context.Background()
	s, err := auto.SelectStackInlineSource(ctx, stackName, projectName, deployFunc)
	if err != nil {
		// ignore if stack is not found
		if auto.IsSelectStack404Error(err) {
			klog.V(4).Info("Skipping destroy because stack was not found")
			return auto.DestroyResult{}, nil
		}
		klog.Errorf("Failed to select stack: %v", err)
		return auto.DestroyResult{}, err
	}
	klog.V(4).Info("Starting destroy")
	// wire up our update to stream progress to stdout
	stdoutStreamer := optdestroy.ProgressStreams(os.Stdout)
	res, err := s.Destroy(ctx, stdoutStreamer)
	if err != nil {
		klog.Errorf("Failed to destroy stack: %v", err)
		return auto.DestroyResult{}, err
	}

	klog.V(4).Info("Destroy succeeded!")

	klog.V(4).Info("Starting remove")
	// wire up our update to stream progress to stdout
	err = s.Workspace().RemoveStack(ctx, stackName)
	if err != nil {
		klog.Errorf("Failed to remove stack: %v", err)
		return res, err
	}
	klog.V(4).Info("Remove stack succeeded!")

	return res, nil
}

func createOrSelectStack(ctx context.Context, stackName, projectName string, deployFunc pulumi.RunFunc) (auto.Stack, error) {
	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, deployFunc)
	//auto.EnvVars(map[string]string{})
	if err != nil {
		klog.Errorf("Failed to create or select stack: %v", err)
		return auto.Stack{}, err
	}

	klog.V(4).Info("Installing plugins")
	w := s.Workspace()

	azureEnabled, err := strconv.ParseBool(os.Getenv(EnvAzureEnabled))
	if err != nil {
		klog.Errorf("Failed to parse %s: %v", EnvAzureEnabled, err)
		return auto.Stack{}, err
	}

	// for inline source programs, we must manage plugins ourselves
	if azureEnabled {
		err = w.InstallPlugin(ctx, "azure-native", "v2.4.0")
		if err != nil {
			klog.Errorf("Failed to install azure-native plugin: %v", err)
			return auto.Stack{}, err
		}
		err = w.InstallPlugin(ctx, "azuread", "v5.38.0")
		if err != nil {
			klog.Errorf("Failed to install azure-ad plugin: %v", err)
			return auto.Stack{}, err
		}
	}
	err = w.InstallPlugin(ctx, "random", "v4.13.2")
	if err != nil {
		klog.Errorf("Failed to install random plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "vault", "v5.11.0")
	if err != nil {
		klog.Errorf("Failed to install vault plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "kubernetes", "v4.18.3")
	if err != nil {
		klog.Errorf("Failed to install kubernetes plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "mssql", "v0.1.0")
	if err != nil {
		klog.Errorf("Failed to install mssql plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "command", "v1.0.1")
	if err != nil {
		klog.Errorf("Failed to install command plugin: %v", err)
		return auto.Stack{}, err
	}
	klog.V(4).Info("Successfully installed plugins")

	// set stack configuration
	configValues := map[string]auto.ConfigValue{}
	if azureEnabled {
		azureConfigValues := map[string]auto.ConfigValue{
			"azure-native:location":       {Value: os.Getenv("AZURE_LOCATION")},
			"azure-native:clientId":       {Value: os.Getenv("AZURE_CLIENT_ID")},
			"azure-native:subscriptionId": {Value: os.Getenv("AZURE_SUBSCRIPTION_ID")},
			"azure-native:tenantId":       {Value: os.Getenv("AZURE_TENANT_ID")},
			"azure-native:clientSecret":   {Value: os.Getenv("AZURE_CLIENT_SECRET"), Secret: true},
			"azuread:clientId":            {Value: os.Getenv("ARM_CLIENT_ID")},
			"azuread:tenantId":            {Value: os.Getenv("ARM_TENANT_ID")},
			"azuread:clientSecret":        {Value: os.Getenv("ARM_CLIENT_SECRET"), Secret: true},
		}

		for key, value := range azureConfigValues {
			configValues[key] = value
		}
	}

	if len(configValues) > 0 {
		err := s.SetAllConfig(ctx, configValues)
		if err != nil {
			klog.Errorf("Failed to set config: %v", err)
			return auto.Stack{}, err
		} else {
			klog.V(4).Info("Successfully set config")
		}
	}

	if skipRefresh, err := strconv.ParseBool(os.Getenv(EnvPulumiSkipRefresh)); err == nil && skipRefresh {
		klog.V(4).Info("Skipping refresh")
	} else {
		klog.V(4).Info("Starting refresh")
		_, err = s.Refresh(ctx)
		if err != nil {
			klog.Errorf("Failed to refresh stack: %v", err)
			return auto.Stack{}, err
		}

		klog.V(4).Info("Refresh succeeded!")
	}

	return s, nil
}

func deployResource(target provisioning.ProvisioningTarget,
	rgName *pulumi.StringOutput,
	res provisioning.ProvisioningResource,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (pulumi.Resource, error) {

	kind := res.GetObjectKind().GroupVersionKind().Kind

	// https://github.com/kubernetes/client-go/issues/308
	if kind == "" {
		kind = reflect.Indirect(reflect.ValueOf(res)).Type().Name()
	}

	switch kind {
	case string(provisioning.ProvisioningResourceKindEntraUser):
		return deployEntraUser(target, res.(*provisioningv1.EntraUser), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindAzureDatabase):
		return deployAzureDb(target, res.(*provisioningv1.AzureDatabase), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindAzureManagedDatabase):
		return deployAzureManagedDb(target, res.(*provisioningv1.AzureManagedDatabase), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindAzurePowerShellScript):
		return deployAzurePowerShellScript(target, *rgName, res.(*provisioningv1.AzurePowerShellScript), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindHelmRelease):
		return deployHelmRelease(target, res.(*provisioningv1.HelmRelease), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindAzureVirtualMachine):
		return deployAzureVirtualMachine(target, *rgName, res.(*provisioningv1.AzureVirtualMachine), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindAzureVirtualDesktop):
		return deployAzureVirtualDesktop(target, *rgName, res.(*provisioningv1.AzureVirtualDesktop), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindMsSqlDatabase):
		return deployMsSqlDb(target, res.(*provisioningv1.MsSqlDatabase), dependencies, ctx)
	case string(provisioning.ProvisioningResourceKindLocalScript):
		return deployLocalScript(target, res.(*provisioningv1.LocalScript), dependencies, ctx)
	default:
		return nil, fmt.Errorf("unknown resource kind: %s", kind)
	}
}

func deployResourceWithDeps(target provisioning.ProvisioningTarget, rgName *pulumi.StringOutput, res provisioning.ProvisioningResource, provisionedRes provisionedResourceMap, infra *provisioning.InfrastructureManifests, ctx *pulumi.Context) (pulumi.Resource, error) {

	id := provisioningv1.ProvisioningResourceIdendtifier{Name: res.GetName(), Kind: provisioningv1.ProvisioningResourceKind(res.GetObjectKind().GroupVersionKind().Kind)}
	if pulumiRes, found := provisionedRes[id]; found {
		return pulumiRes, nil
	}

	pulumiDeps := []pulumi.Resource{}

	for _, dep := range res.GetProvisioningMeta().DependsOn {
		if provRes, found := infra.Get(dep); found {
			pulumiRes, err := deployResourceWithDeps(target, rgName, provRes, provisionedRes, infra, ctx)
			if err != nil {
				return nil, err
			}
			pulumiDeps = append(pulumiDeps, pulumiRes)
		}
	}

	pulumiRes, err := deployResource(target, rgName, res, pulumiDeps, ctx)
	if err != nil {
		return nil, err
	}

	provisionedRes[id] = pulumiRes

	return pulumiRes, nil
}

func deployFunc(target provisioning.ProvisioningTarget, domain string,
	infra *provisioning.InfrastructureManifests, needsResourceGroup bool) pulumi.RunFunc {

	return func(ctx *pulumi.Context) error {

		provisionedRes := make(provisionedResourceMap)

		var rgName *pulumi.StringOutput

		if needsResourceGroup {
			resGroupName, err := deployAzureRG(target, domain)(ctx)
			if err != nil {
				return err
			}

			rgName = &resGroupName
		}

		for _, user := range infra.EntraUsers {
			_, err := deployResourceWithDeps(target, rgName, user, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, db := range infra.AzureDbs {
			_, err := deployResourceWithDeps(target, rgName, db, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, db := range infra.AzureManagedDbs {
			_, err := deployResourceWithDeps(target, rgName, db, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, db := range infra.AzurePowerShellScripts {
			_, err := deployResourceWithDeps(target, rgName, db, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, hr := range infra.HelmReleases {
			_, err := deployResourceWithDeps(target, rgName, hr, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, vm := range infra.AzureVirtualMachines {
			_, err := deployResourceWithDeps(target, rgName, vm, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, avd := range infra.AzureVirtualDesktops {
			_, err := deployResourceWithDeps(target, rgName, avd, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, db := range infra.MsSqlDbs {
			_, err := deployResourceWithDeps(target, rgName, db, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		for _, ls := range infra.LocalScripts {
			_, err := deployResourceWithDeps(target, rgName, ls, provisionedRes, infra, ctx)
			if err != nil {
				return err
			}
		}

		return nil
	}
}
