// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	"os"

	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
)

var (
	EnvPulumiSkipRefresh = "PULUMI_SKIP_REFRESH"
)

func Create[T provisioning.ProvisioningTarget](target T, domain string, infra *provisioning.InfrastructureManifests) provisioning.ProvisioningResult {
	result := provisioning.ProvisioningResult{}
	upRes := auto.UpResult{}
	destroyRes := auto.DestroyResult{}
	emptyDeployFunc := func(ctx *pulumi.Context) error { return nil }

	anyAzureDb := len(infra.AzureDbs) > 0
	anyManagedAzureDb := len(infra.AzureManagedDbs) > 0
	anyHelmRelease := len(infra.HelmReleases) > 0
	anyVirtualMachine := len(infra.AzureVirtualMachines) > 0
	anyVirtualDesktop := len(infra.AzureVirtualDesktops) > 0

	anyResource := anyAzureDb || anyManagedAzureDb || anyHelmRelease || anyVirtualMachine || anyVirtualDesktop
	needsResourceGroup := anyVirtualMachine || anyVirtualDesktop

	stackName := fmt.Sprintf("%s-%s", target.GetPathSegment(), domain)
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

	// for inline source programs, we must manage plugins ourselves
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
	err = w.InstallPlugin(ctx, "kubernetes", "v3.28.1")
	if err != nil {
		klog.Errorf("Failed to install kubernetes plugin: %v", err)
		return auto.Stack{}, err
	}
	klog.V(4).Info("Successfully installed plugins")

	// set stack configuration
	_ = s.SetAllConfig(ctx, map[string]auto.ConfigValue{
		"azure-native:location":       {Value: os.Getenv("AZURE_LOCATION")},
		"azure-native:clientId":       {Value: os.Getenv("AZURE_CLIENT_ID")},
		"azure-native:subscriptionId": {Value: os.Getenv("AZURE_SUBSCRIPTION_ID")},
		"azure-native:tenantId":       {Value: os.Getenv("AZURE_TENANT_ID")},
		"azure-native:clientSecret":   {Value: os.Getenv("AZURE_CLIENT_SECRET"), Secret: true},
		"azuread:clientId":            {Value: os.Getenv("ARM_CLIENT_ID")},
		"azuread:tenantId":            {Value: os.Getenv("ARM_TENANT_ID")},
		"azuread:clientSecret":        {Value: os.Getenv("ARM_CLIENT_SECRET"), Secret: true}})

	klog.V(4).Info("Successfully set config")

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

func deployFunc[T provisioning.ProvisioningTarget](target T, domain string,
	infra *provisioning.InfrastructureManifests, needsResourceGroup bool) pulumi.RunFunc {

	return func(ctx *pulumi.Context) error {
		err := azureDbDeployFunc(target, infra.AzureDbs)(ctx)
		if err != nil {
			return err
		}

		err = azureManagedDbDeployFunc(target, infra.AzureManagedDbs)(ctx)
		if err != nil {
			return err
		}

		err = helmReleaseDeployFunc(target, infra.HelmReleases)(ctx)
		if err != nil {
			return err
		}

		if needsResourceGroup {
			rgName, err := azureRGDeployFunc(target, domain)(ctx)
			if err != nil {
				return err
			}

			err = azureVirtualMachineDeployFunc(target, rgName, infra.AzureVirtualMachines)(ctx)
			if err != nil {
				return err
			}

			err = azureVirtualDesktopDeployFunc(target, rgName, infra.AzureVirtualDesktops)(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	}
}
