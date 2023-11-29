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

func deployAzureDb(target provisioning.ProvisioningTarget,
	azureDb *provisioningv1.AzureDatabase,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*azureSql.Database, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureDatabase")

	server, err := azureSql.LookupServer(ctx, &azureSql.LookupServerArgs{
		ResourceGroupName: azureDb.Spec.SqlServer.ResourceGroupName,
		ServerName:        azureDb.Spec.SqlServer.ServerName,
	})
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf("sqlServer %s not found", azureDb.Spec.SqlServer.ServerName)
	}

	dbArgs := &azureSql.DatabaseArgs{
		ResourceGroupName: pulumi.String(azureDb.Spec.SqlServer.ResourceGroupName),
		ServerName:        pulumi.String(server.Name),
	}

	if azureDb.Spec.SourceDatabaseId != "" {
		//https://www.pulumi.com/registry/packages/azure-native/api-docs/sql/database/#createmode_go
		dbArgs.CreateMode = pulumi.String("Copy")
		dbArgs.SourceDatabaseId = pulumi.String(azureDb.Spec.SourceDatabaseId)
	}

	if azureDb.Spec.SqlServer.ElasticPoolName != "" {
		pool, err := azureSql.LookupElasticPool(ctx, &azureSql.LookupElasticPoolArgs{
			ResourceGroupName: azureDb.Spec.SqlServer.ResourceGroupName,
			ServerName:        azureDb.Spec.SqlServer.ServerName,
			ElasticPoolName:   azureDb.Spec.SqlServer.ElasticPoolName,
		})
		if err != nil {
			return nil, err
		}
		if pool == nil {
			return nil, fmt.Errorf("elasticPool %s not found", azureDb.Spec.SqlServer.ElasticPoolName)
		}
		dbArgs.ElasticPoolId = pulumi.String(pool.Id)
	} else {
		sku := "S0"
		if azureDb.Spec.Sku != "" {
			sku = azureDb.Spec.Sku
		}
		dbArgs.Sku = &azureSql.SkuArgs{
			Name: pulumi.String(sku),
		}
	}

	pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources

	ignoreChanges := []string{}
	if pulumiRetainOnDelete {
		ignoreChanges = []string{"resourceGroupName", "serverName", "elasticPoolId", "createMode", "sourceDatabaseId", "maxSizeBytes", "readScale", "requestedBackupStorageRedundancy", "catalogCollation", "collation", "sku", "zoneRedundant", "maintenanceConfigurationId", "isLedgerOn"}
	}

	dbNameV1 := provisioning.Match(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s_%s_%s", azureDb.Spec.DbName, tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s_%s", azureDb.Spec.DbName, platform.GetName())
		},
	)

	dbName := strings.ReplaceAll(dbNameV1, ".", "_")
	db, err := azureSql.NewDatabase(ctx, dbName, dbArgs,
		pulumi.RetainOnDelete(pulumiRetainOnDelete),
		pulumi.IgnoreChanges(ignoreChanges),
		pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String(dbNameV1)}}),
		pulumi.Import(pulumi.ID(azureDb.Spec.ImportDatabaseId)),
		pulumi.DependsOn(dependencies),
	)
	if err != nil {
		return nil, err
	}
	ctx.Export("azureDbName", db.Name)

	for _, exp := range azureDb.Spec.Exports {
		err = valueExporter(newExportContext(ctx, exp.Domain, azureDb.Name, azureDb.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}})
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}
