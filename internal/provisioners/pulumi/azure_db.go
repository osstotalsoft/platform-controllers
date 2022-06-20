package pulumi

import (
	"fmt"

	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"
	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureDbDeployFunc(platform string, tenant *platformv1.Tenant, resourceGroupName string,
	azureDbs []*provisioningv1.AzureDatabase) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, tenant)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureDatabase")
	return func(ctx *pulumi.Context) error {
		if len(azureDbs) > 0 {
			const pwdKey = "pass"
			const userKey = "user"
			adminPwd := generatePassword()
			adminUser := fmt.Sprintf("sqlUser%s", tenant.ObjectMeta.Name)
			secretPath := fmt.Sprintf("%s/__provisioner/%s/azure-databases/server", platform, tenant.ObjectMeta.Name)
			secret, err := vault.LookupSecret(ctx, &vault.LookupSecretArgs{Path: secretPath})
			if secret != nil && secret.Data[pwdKey] != nil {
				adminPwd = secret.Data[pwdKey].(string)
			}

			allowServerDeletion := true
			for _, dbSpec := range azureDbs {
				if !dbSpec.Spec.AllowDeletion {
					allowServerDeletion = false
				}
			}

			server, err := azureSql.NewServer(ctx, fmt.Sprintf("%s-sqlserver", tenant.ObjectMeta.Name),
				&azureSql.ServerArgs{
					ResourceGroupName:          pulumi.String(resourceGroupName),
					AdministratorLogin:         pulumi.String(adminUser),
					AdministratorLoginPassword: pulumi.String(adminPwd),
					Version:                    pulumi.String("12.0"),
				},
				//pulumi.Protect(!allowServerDeletion),
				pulumi.RetainOnDelete(!allowServerDeletion),
			)
			if err != nil {
				return err
			}

			for _, dbSpec := range azureDbs {
				sku := "S0"
				if dbSpec.Spec.Sku != "" {
					sku = dbSpec.Spec.Sku
				}

				db, err := azureSql.NewDatabase(ctx, dbSpec.Spec.DbName,
					&azureSql.DatabaseArgs{
						ResourceGroupName: pulumi.String(resourceGroupName),
						ServerName:        server.Name,
						Sku: &azureSql.SkuArgs{
							Name: pulumi.String(sku),
						},
					},
					//pulumi.Protect(!dbSpec.Spec.AllowDeletion),
					pulumi.RetainOnDelete(!dbSpec.Spec.AllowDeletion),
				)

				if err != nil {
					return err
				}

				ctx.Export(fmt.Sprintf("azureDb:%s", dbSpec.Spec.DbName), db.Name)
				//ctx.Export(fmt.Sprintf("azureDbPassword_%s", dbSpec.Spec.Name), pulumi.ToSecret(pwd))

				for _, domain := range dbSpec.Spec.Domains {
					err = valueExporter(newExportContext(ctx, domain, dbSpec.Name, dbSpec.ObjectMeta, gvk),
						dbSpec.Spec.Exports.UserName, pulumi.String(adminUser))
					if err != nil {
						return err
					}
				}
				for _, domain := range dbSpec.Spec.Domains {
					err = valueExporter(newExportContext(ctx, domain, dbSpec.Name, dbSpec.ObjectMeta, gvk),
						dbSpec.Spec.Exports.Password, pulumi.String(adminPwd))
					if err != nil {
						return err
					}
				}
			}
			_, err = vault.NewSecret(ctx, secretPath,
				&vault.SecretArgs{
					DataJson: pulumi.String(fmt.Sprintf(`{"%s":"%s", "%s":"%s"}`, pwdKey, adminPwd, userKey, adminUser)),
					Path:     pulumi.String(secretPath),
				},
				pulumi.RetainOnDelete(!allowServerDeletion),
			)
			if err != nil {
				return err
			}
		}
		return nil
	}
}
