package pulumi

import (
	"fmt"
	"strings"

	azureSql "github.com/pulumi/pulumi-azure-native-sdk/sql/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureDbDeployFunc[T provisioning.ProvisioningTarget](platform string, target T,
	azureDbs []*provisioningv1.AzureDatabase) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, target)
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
			pulumiRetainOnDelete := target.GetDeletePolicy() == platformv1.DeletePolicyRetainStatefulResources
			ignoreChanges := []string{}
			if pulumiRetainOnDelete {
				ignoreChanges = []string{"resourceGroupName", "serverName", "elasticPoolId", "createMode", "sourceDatabaseId", "maxSizeBytes", "readScale", "requestedBackupStorageRedundancy", "catalogCollation", "collation", "sku", "zoneRedundant", "maintenanceConfigurationId"}
			}

			dbNameV1 := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, target.GetName())
			dbName := strings.ReplaceAll(dbNameV1, ".", "_")
			db, err := azureSql.NewDatabase(ctx, dbName, dbArgs,
				pulumi.RetainOnDelete(pulumiRetainOnDelete),
				pulumi.IgnoreChanges(ignoreChanges),
				pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String(dbNameV1)}}),
				pulumi.Import(pulumi.ID(dbSpec.Spec.ImportDatabaseId)),
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
