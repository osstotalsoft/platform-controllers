package pulumi

import (
	"encoding/json"
	"os"

	"github.com/pulumi/pulumi-azure-native-sdk/managedidentity/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	"totalsoft.ro/platform-controllers/internal/template"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployAzurePowerShellScript(target provisioning.ProvisioningTarget,
	resourceGroupName pulumi.StringOutput,
	azurePowerShellScript *provisioningv1.AzurePowerShellScript,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*resources.AzurePowerShellScript, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzurePowerShellScript")

	tc := provisioning.GetTemplateContext(target)

	parsedArgs, err := template.ParseTemplate(azurePowerShellScript.Spec.ScriptArguments, tc)
	if err != nil {
		return nil, err
	}

	os.Getenv("AZURE_MANAGED_IDENTITY_RG")

	managedIdentity, err := managedidentity.LookupUserAssignedIdentity(ctx, &managedidentity.LookupUserAssignedIdentityArgs{
		ResourceGroupName: os.Getenv("AZURE_MANAGED_IDENTITY_RG"),
		ResourceName:      os.Getenv("AZURE_MANAGED_IDENTITY_NAME"),
	})

	if err != nil {
		return nil, err
	}

	script, err := resources.NewAzurePowerShellScript(ctx, azurePowerShellScript.Name, &resources.AzurePowerShellScriptArgs{
		Kind:              pulumi.String("AzurePowerShell"),
		ForceUpdateTag:    pulumi.String(azurePowerShellScript.Spec.ForceUpdateTag), // Change to force redeploying the script if desired
		ResourceGroupName: resourceGroupName,
		Arguments:         pulumi.String(parsedArgs), // Set the arguments for the script'"),
		ScriptContent:     pulumi.String(azurePowerShellScript.Spec.ScriptContent),
		CleanupPreference: pulumi.String("OnSuccess"), // Set the cleanup preference for the script
		Timeout:           pulumi.String("PT1H"),      // Set an appropriate timeout for the script
		Identity: &resources.ManagedServiceIdentityArgs{
			Type: pulumi.String(resources.ManagedServiceIdentityTypeUserAssigned),
			UserAssignedIdentities: pulumi.StringArray{
				pulumi.String(managedIdentity.Id),
			},
		},
		AzPowerShellVersion: pulumi.String("11.0"), // Specify the desired version of Az PowerShell module
		RetentionInterval:   pulumi.String("P1D"),  // Set the retention time for the script's logs
	})
	if err != nil {
		return nil, err
	}

	for _, exp := range azurePowerShellScript.Spec.Exports {
		domain := exp.Domain
		if domain == "" {
			domain = azurePowerShellScript.Spec.DomainRef
		}

		err = valueExporter(newExportContext(ctx, domain, azurePowerShellScript.Name, azurePowerShellScript.ObjectMeta, gvk),

			map[string]exportTemplateWithValue{"scriptOutputs": {exp.ScriptOutputs, script.Outputs.ApplyT(func(outputs map[string]interface{}) (string, error) {
				outputsJson, err := json.Marshal(outputs)
				if err != nil {
					return "", err
				}

				return string(outputsJson), err
			}).(pulumi.StringOutput)}})
		if err != nil {
			return nil, err
		}
	}
	return script, nil
}
