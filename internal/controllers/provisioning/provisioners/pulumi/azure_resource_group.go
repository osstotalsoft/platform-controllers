package pulumi

import (
	"fmt"

	azureResources "github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
)

func azureRGDeployFunc[T provisioning.ProvisioningTarget](platform string, target T, domain string) func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
	return func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
		resourceGroupName := fmt.Sprintf("%s-%s-%s", platform, target.GetName(), domain)
		resourceGroup, err := azureResources.NewResourceGroup(ctx, resourceGroupName, &azureResources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
		}, pulumi.RetainOnDelete(true))
		if err != nil {
			return pulumi.StringOutput{}, err
		}
		ctx.Export("azureRGName", resourceGroup.Name)
		return resourceGroup.Name, nil
	}
}
