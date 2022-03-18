// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	azureResources "github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"os"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployFunc(platform string, tenant *provisioningv1.Tenant, dbList []*provisioningv1.AzureDatabase) pulumi.RunFunc {
	const pwd = "sqlPasswo234@#$@#$@#$44rd"
	const username = "pulumi"

	return func(ctx *pulumi.Context) error {
		resourceGroup, err := azureResources.NewResourceGroup(ctx, fmt.Sprintf("pulumi_%s_tenant_%s_RG", platform, tenant.Spec.Code), nil)
		if err != nil {
			return err
		}

		server, err := azureSql.NewServer(ctx, fmt.Sprintf("%s-sqlserver", tenant.Spec.Code),
			&azureSql.ServerArgs{
				ResourceGroupName:          resourceGroup.Name,
				AdministratorLogin:         pulumi.String(username),
				AdministratorLoginPassword: pulumi.String(pwd),
				Version:                    pulumi.String("12.0"),
			})
		if err != nil {
			return err
		}

		for _, dbSpec := range dbList {
			db, err := azureSql.NewDatabase(ctx, dbSpec.Spec.Name, &azureSql.DatabaseArgs{
				ResourceGroupName: resourceGroup.Name,
				ServerName:        server.Name,
				Sku: &azureSql.SkuArgs{
					Name: pulumi.String("S0"),
				},
			})
			if err != nil {
				return err
			}

			// export the db name
			ctx.Export(fmt.Sprintf("azureDatabase_%s", dbSpec.Spec.Name), db.Name)
		}

		return nil
	}
}

func Create(platform string, tenant *provisioningv1.Tenant, dbList []*provisioningv1.AzureDatabase) error {
	ctx := context.Background()

	stackName := fmt.Sprintf("tenant_%s", tenant.Spec.Code)
	s, err := auto.UpsertStackInlineSource(ctx, stackName, platform, deployFunc(platform, tenant, dbList))
	//auto.WorkDir(fmt.Sprintf("~/pulumi_ws/workspace_%s", platform)),
	//auto.EnvVars(map[string]string{})

	if err != nil {
		klog.Errorf("Failed to create or select stack: %v", err)
		return err
	}

	klog.V(4).Info("Installing the azure-native plugin")
	w := s.Workspace()

	// for inline source programs, we must manage plugins ourselves
	err = w.InstallPlugin(ctx, "azure-native", "v1.60.0")
	if err != nil {
		klog.Errorf("Failed to install program plugins: %v", err)
		return err
	}
	klog.V(4).Info("Successfully installed azure-native plugin")

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
		return err
	}

	klog.V(4).Info("Refresh succeeded!")
	klog.V(4).Info("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	// run the update to deploy our fargate web service
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		klog.Errorf("Failed to update stack: %v", err)
		return err
	}

	klog.V(4).Info("Update succeeded!")
	klog.V(4).Info("Stack results", "Outputs", res.Outputs)

	return nil
}

func getConfigValueFromEnv(key string, secret bool) auto.ConfigValue {
	return auto.ConfigValue{Value: os.Getenv(key), Secret: secret}
}
