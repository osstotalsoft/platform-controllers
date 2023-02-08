package pulumi

import (
	"fmt"
	"strings"

	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureDbDeployFunc(platform string, tenant *platformv1.Tenant,
	azureDbs []*provisioningv1.AzureDatabase) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, tenant)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureDatabase")

	return func(ctx *pulumi.Context) error {
		for _, dbSpec := range azureDbs {

			server, err := azureSql.LookupServer(ctx, &azureSql.LookupServerArgs{
				ResourceGroupName: dbSpec.Spec.SqlServer.ResourceGroupName,
				ServerName:        dbSpec.Spec.SqlServer.ServerName,
			})
			if err != nil {
				return err
			}
			if server == nil {
				return fmt.Errorf("sqlServer %s not found", dbSpec.Spec.SqlServer.ServerName)
			}

			dbArgs := &azureSql.DatabaseArgs{
				ResourceGroupName: pulumi.String(dbSpec.Spec.SqlServer.ResourceGroupName),
				ServerName:        pulumi.String(server.Name),
			}

			if dbSpec.Spec.SourceDatabaseId != "" {
				//https://www.pulumi.com/registry/packages/azure-native/api-docs/sql/database/#createmode_go
				dbArgs.CreateMode = pulumi.String("Copy")
				dbArgs.SourceDatabaseId = pulumi.String(dbSpec.Spec.SourceDatabaseId)
			}

			if dbSpec.Spec.SqlServer.ElasticPoolName != "" {
				pool, err := azureSql.LookupElasticPool(ctx, &azureSql.LookupElasticPoolArgs{
					ResourceGroupName: dbSpec.Spec.SqlServer.ResourceGroupName,
					ServerName:        dbSpec.Spec.SqlServer.ServerName,
					ElasticPoolName:   dbSpec.Spec.SqlServer.ElasticPoolName,
				})
				if err != nil {
					return err
				}
				if pool == nil {
					return fmt.Errorf("elasticPool %s not found", dbSpec.Spec.SqlServer.ElasticPoolName)
				}
				dbArgs.ElasticPoolId = pulumi.String(pool.Id)
			} else {
				sku := "S0"
				if dbSpec.Spec.Sku != "" {
					sku = dbSpec.Spec.Sku
				}
				dbArgs.Sku = &azureSql.SkuArgs{
					Name: pulumi.String(sku),
				}
			}

			ignoreChanges := []string{}
			if PulumiRetainOnDelete {
				ignoreChanges = []string{"resourceGroupName", "serverName", "createMode", "sourceDatabaseId"}
			}

			dbNameV1 := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, tenant.Name)
			dbName := strings.ReplaceAll(dbNameV1, ".", "_")
			db, err := azureSql.NewDatabase(ctx, dbName, dbArgs,
				pulumi.RetainOnDelete(PulumiRetainOnDelete),
				pulumi.IgnoreChanges(ignoreChanges),
				pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String(dbNameV1)}}),
			)
			if err != nil {
				return err
			}
			ctx.Export("azureDbName", db.Name)

			for _, exp := range dbSpec.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, dbSpec.Name, dbSpec.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}})
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}
