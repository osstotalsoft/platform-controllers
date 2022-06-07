// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	"strings"

	"math/rand"
	"os"
	"strconv"
	"time"

	azureResources "github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	PulumiSkipAzureManagedDb = "PULUMI_SKIP_AZURE_MANAGED_DB"
	PulumiSkipAzureDb        = "PULUMI_SKIP_AZURE_DB"
)

func azureRGDeployFunc(platform string, tenant *platformv1.Tenant) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		resourceGroup, err := azureResources.NewResourceGroup(ctx, fmt.Sprintf("%s_%s_RG", platform, tenant.Spec.Code), nil)
		if err != nil {
			return err
		}

		ctx.Export("azureRGName", resourceGroup.Name)

		return nil
	}
}

func Create(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {

	azureRGStackName := fmt.Sprintf("%s_rg", tenant.Spec.Code)
	res, err := updateStack(azureRGStackName, platform, azureRGDeployFunc(platform, tenant))
	if err != nil {
		return err
	}

	if s, _ := strconv.ParseBool(os.Getenv(PulumiSkipAzureDb)); !s {
		azureRGName, ok := res.Outputs["azureRGName"].Value.(string)
		if !ok {
			klog.Errorf("Failed to get azureRGName: %v", err)
			return err
		}
		azureDbStackName := fmt.Sprintf("%s_azure_db", tenant.Spec.Code)
		res, err = updateStack(azureDbStackName, platform, azureDbDeployFunc(platform, tenant, azureRGName, infra.AzureDbs))
		if err != nil {
			return err
		}
	}

	if s, _ := strconv.ParseBool(os.Getenv(PulumiSkipAzureManagedDb)); !s {
		azureManagedDbStackName := fmt.Sprintf("%s_azure_managed_db", tenant.Spec.Code)
		res, err = updateStack(azureManagedDbStackName, platform, azureManagedDbDeployFunc(platform, tenant, infra.AzureManagedDbs))
		if err != nil {
			return err
		}
	}
	return nil
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
	err = w.InstallPlugin(ctx, "azure-native", "v1.64.1")
	if err != nil {
		klog.Errorf("Failed to install azure-native plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "vault", "v5.5.0")
	if err != nil {
		klog.Errorf("Failed to install vault plugin: %v", err)
		return auto.Stack{}, err
	}
	err = w.InstallPlugin(ctx, "kubernetes", "v3.19.2")
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
		"azure-native:clientSecret":   {Value: os.Getenv("AZURE_CLIENT_SECRET"), Secret: true}})

	klog.V(4).Info("Successfully set config")
	klog.V(4).Info("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		klog.Errorf("Failed to refresh stack: %v", err)
		return auto.Stack{}, err
	}

	klog.V(4).Info("Refresh succeeded!")
	return s, nil
}

func generatePassword() string {
	rand.Seed(time.Now().UnixNano())
	digits := "0123456789"
	specials := "~=+%^*/()[]{}/!@#$?|"
	all := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		digits + specials
	length := 24
	buf := make([]byte, length)
	buf[0] = digits[rand.Intn(len(digits))]
	buf[1] = specials[rand.Intn(len(specials))]
	for i := 2; i < length; i++ {
		buf[i] = all[rand.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})
	return string(buf)
}

func objectNamingConvention(platform, domain, tenant, object, separator string) string {
	return strings.Join([]string{platform, domain, tenant, object}, separator)
}
