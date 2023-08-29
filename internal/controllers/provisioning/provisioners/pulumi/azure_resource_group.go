package pulumi

import (
	"fmt"

	azureResources "github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func azureRGDeployFunc(platform string, tenant *platformv1.Tenant, domain string) func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
	return func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
		resourceGroupName := fmt.Sprintf("%s-%s-%s", platform, tenant.Name, domain)
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
