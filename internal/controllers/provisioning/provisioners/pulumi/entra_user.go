package pulumi

import (
	"fmt"

	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployEntraUser(target provisioning.ProvisioningTarget,
	entraUser *provisioningv1.EntraUser,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*azuread.User, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("EntraUser")

	initialPassword := pulumi.String(entraUser.Spec.InitialPassword).ToStringOutput()
	if entraUser.Spec.InitialPassword == "" {
		var err error
		initialPassword, err = newRandomPassword(ctx, fmt.Sprintf("%s-initial", entraUser.Spec.UserPrincipalName))
		if err != nil {
			return nil, err
		}
	}

	user, err := azuread.NewUser(ctx, entraUser.Name, &azuread.UserArgs{
		UserPrincipalName:   pulumi.String(entraUser.Spec.UserPrincipalName),
		DisplayName:         pulumi.String(entraUser.Spec.DisplayName),
		Password:            initialPassword,
		ForcePasswordChange: pulumi.Bool(true),
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	for _, exp := range entraUser.Spec.Exports {
		domain := exp.Domain
		if domain == "" {
			domain = entraUser.Spec.DomainRef
		}

		err = valueExporter(newExportContext(ctx, domain, entraUser.Name, entraUser.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{
				"initialPassword":   {exp.InitialPassword, initialPassword},
				"userPrincipalName": {exp.UserPrincipalName, pulumi.String(entraUser.Spec.UserPrincipalName)},
			})
		if err != nil {
			return nil, err
		}
	}
	return user, nil
}
