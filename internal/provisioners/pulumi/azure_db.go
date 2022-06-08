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
	return func(ctx *pulumi.Context) error {
		if len(azureDbs) > 0 {
			const pwdKey = "pass"
			const userKey = "user"
			adminPwd := generatePassword()
			adminUser := fmt.Sprintf("sqlUser%s", tenant.Spec.Code)
			secretPath := fmt.Sprintf("%s/__provisioner/%s/azure-databases/server", platform, tenant.Spec.Code)
			secret, err := vault.LookupSecret(ctx, &vault.LookupSecretArgs{Path: secretPath})
			if secret != nil && secret.Data[pwdKey] != nil {
				adminPwd = secret.Data[pwdKey].(string)
			}

			server, err := azureSql.NewServer(ctx, fmt.Sprintf("%s-sqlserver", tenant.Spec.Code),
				&azureSql.ServerArgs{
					ResourceGroupName:          pulumi.String(resourceGroupName),
					AdministratorLogin:         pulumi.String(adminUser),
					AdministratorLoginPassword: pulumi.String(adminPwd),
					Version:                    pulumi.String("12.0"),
				})
			if err != nil {
				return err
			}

			//pool, err := azureSql.NewElasticPool(ctx, fmt.Sprintf("%s-sqlserver-pool", tenant.Spec.Code),
			//	&azureSql.ElasticPoolArgs{
			//		ResourceGroupName: resourceGroup.Name,
			//		ServerName:        server.Name,
			//	})
			//if err != nil {
			//	return err
			//}

			for _, dbSpec := range azureDbs {
				sku := "S0"
				if dbSpec.Spec.Sku != "" {
					sku = dbSpec.Spec.Sku
				}
				db, err := azureSql.NewDatabase(ctx, dbSpec.Spec.DbName, &azureSql.DatabaseArgs{
					ResourceGroupName: pulumi.String(resourceGroupName),
					ServerName:        server.Name,
					Sku: &azureSql.SkuArgs{
						Name: pulumi.String(sku),
					},
				})
				if err != nil {
					return err
				}

				ctx.Export(fmt.Sprintf("azureDb:%s", dbSpec.Spec.DbName), db.Name)
				//ctx.Export(fmt.Sprintf("azureDbPassword_%s", dbSpec.Spec.Name), pulumi.ToSecret(pwd))

				for _, domain := range dbSpec.Spec.Domains {
					err = valueExporter(newExportContext(ctx, domain, dbSpec.Name, dbSpec.ObjectMeta, dbSpec.TypeMeta),
						dbSpec.Spec.Exports.UserName, pulumi.String(adminUser))
					if err != nil {
						return err
					}
				}
				for _, domain := range dbSpec.Spec.Domains {
					err = valueExporter(newExportContext(ctx, domain, dbSpec.Name, dbSpec.ObjectMeta, dbSpec.TypeMeta),
						dbSpec.Spec.Exports.Password, pulumi.String(adminPwd))
					if err != nil {
						return err
					}
				}
			}
			_, err = vault.NewSecret(ctx, secretPath, &vault.SecretArgs{
				DataJson: pulumi.String(fmt.Sprintf(`{"%s":"%s", "%s":"%s"}`, pwdKey, adminPwd, userKey, adminUser)),
				Path:     pulumi.String(secretPath),
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
}
