package pulumi

import (
	"fmt"
	"strings"

	azureSql "github.com/pulumi/pulumi-azure-native-sdk/sql/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
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
	ignoreChanges := []string{"resourceGroupName", "serverName", "elasticPoolId", "createMode", "sourceDatabaseId", "maxSizeBytes", "readScale", "requestedBackupStorageRedundancy", "catalogCollation", "collation", "sku", "zoneRedundant", "maintenanceConfigurationId", "isLedgerOn"}

	dbNameV1 := provisioning.MatchTarget(target,
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

	var username string
	var password, identityClientId, identityPrincipalId pulumi.StringOutput
	if azureDb.Spec.User != nil || azureDb.Spec.ManagedIdentity != nil {
		provider, err := newMssqlAzureAuthProvider(ctx, azureDb.Name,
			fmt.Sprintf("%s.database.windows.net", azureDb.Spec.SqlServer.ServerName))
		if err != nil {
			return nil, err
		}

		dbLookup := mssql.LookupDatabaseOutput(ctx, mssql.LookupDatabaseOutputArgs{
			Name: db.Name,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{db}))
		databaseId := dbLookup.ApplyT(func(r mssql.LookupDatabaseResult) string { return r.Id }).(pulumi.StringOutput)

		if azureDb.Spec.User != nil {
			username, password, err = deployContainedUser(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.User, dbName, []pulumi.Resource{db}, pulumiRetainOnDelete)
			if err != nil {
				return nil, err
			}
		}

		if azureDb.Spec.ManagedIdentity != nil {
			identityClientId, identityPrincipalId, err = deployManagedIdentity(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.ManagedIdentity, dbName, []pulumi.Resource{db}, pulumiRetainOnDelete)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, exp := range azureDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}}
		if azureDb.Spec.User != nil {
			values["username"] = exportTemplateWithValue{exp.Username, pulumi.String(username)}
			values["password"] = exportTemplateWithValue{exp.Password, password}
		}
		if azureDb.Spec.ManagedIdentity != nil {
			values["identityClientId"] = exportTemplateWithValue{exp.IdentityClientId, identityClientId}
			values["identityPrincipalId"] = exportTemplateWithValue{exp.IdentityPrincipalId, identityPrincipalId}
		}
		err = valueExporter(newExportContext(ctx, exp.Domain, azureDb.Name, azureDb.ObjectMeta, gvk),
			values)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}
