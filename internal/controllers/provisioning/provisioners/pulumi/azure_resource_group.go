package pulumi

import (
	"fmt"

	azureResources "github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func deployAzureRG(target provisioning.ProvisioningTarget, domain string) func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
	return func(ctx *pulumi.Context) (pulumi.StringOutput, error) {
		resourceGroupName := provisioning.MatchTarget(target,
			func(tenant *platformv1.Tenant) string {
				return fmt.Sprintf("%s-%s-%s", tenant.Spec.PlatformRef, tenant.GetName(), domain)
			},
			func(platform *platformv1.Platform) string {
				return fmt.Sprintf("%s-%s", platform.GetName(), domain)
			},
		)

		pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources

		resourceGroup, err := azureResources.NewResourceGroup(ctx, resourceGroupName, &azureResources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
		}, pulumi.RetainOnDelete(pulumiRetainOnDelete))
		if err != nil {
			return pulumi.StringOutput{}, err
		}
		ctx.Export("azureRGName", resourceGroup.Name)
		return resourceGroup.Name, nil
	}
}
