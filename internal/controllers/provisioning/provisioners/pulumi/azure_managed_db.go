package pulumi

import (
	"fmt"
	"os"
	"strings"

	azureSql "github.com/pulumi/pulumi-azure-native-sdk/sql/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployAzureManagedDb(
	target provisioning.ProvisioningTarget,
	azureDb *provisioningv1.AzureManagedDatabase,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*azureSql.ManagedDatabase, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureManagedDatabase")

	dbNameV1 := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s_%s_%s", azureDb.Spec.DbName, tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s_%s", azureDb.Spec.DbName, platform.GetName())
		},
	)
	dbName := strings.ReplaceAll(dbNameV1, ".", "_")
	args := azureSql.ManagedDatabaseArgs{
		ManagedInstanceName: pulumi.String(azureDb.Spec.ManagedInstance.Name),
		ResourceGroupName:   pulumi.String(azureDb.Spec.ManagedInstance.ResourceGroup),
	}
	restoreFrom := azureDb.Spec.RestoreFrom
	if (restoreFrom != provisioningv1.AzureManagedDatabaseRestoreSpec{}) {
		args.CreateMode = pulumi.String("RestoreExternalBackup")
		args.AutoCompleteRestore = pulumi.Bool(true)
		args.LastBackupName = pulumi.String(restoreFrom.BackupFileName)
		args.StorageContainerSasToken = pulumi.String(restoreFrom.StorageContainer.SasToken)
		args.StorageContainerUri = pulumi.String(restoreFrom.StorageContainer.Uri)
	}

	pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources
	ignoreChanges := []string{"managedInstanceName", "resourceGroupName", "createMode", "autoCompleteRestore", "lastBackupName", "storageContainerSasToken", "storageContainerUri", "collation"}

	db, err := azureSql.NewManagedDatabase(ctx, dbName, &args,
		pulumi.RetainOnDelete(pulumiRetainOnDelete),
		pulumi.IgnoreChanges(ignoreChanges),
		pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String(dbNameV1)}}),
		pulumi.Import(pulumi.ID(azureDb.Spec.ImportDatabaseId)),
		pulumi.DependsOn(dependencies),
	)
	if err != nil {
		return nil, err
	}

	var username string
	var password, identityClientId, identityPrincipalId pulumi.StringOutput
	if azureDb.Spec.User != nil || azureDb.Spec.ManagedIdentity != nil {
		provider, err := mssql.NewProvider(ctx, "mssql-provider", &mssql.ProviderArgs{
			Hostname: pulumi.String(fmt.Sprintf("%s.%s.database.windows.net", azureDb.Spec.ManagedInstance.Name, azureDb.Spec.ManagedInstance.ResourceGroup)),
			AzureAuth: &mssql.ProviderAzureAuthArgs{
				ClientId:     pulumi.String(os.Getenv("AZURE_CLIENT_ID")),
				ClientSecret: pulumi.String(os.Getenv("AZURE_CLIENT_SECRET")),
				TenantId:     pulumi.String(os.Getenv("AZURE_TENANT_ID")),
			},
		})
		if err != nil {
			return nil, err
		}

		dbLookup := mssql.LookupDatabaseOutput(ctx, mssql.LookupDatabaseOutputArgs{
			Name: db.Name,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{db}))
		databaseId := dbLookup.ApplyT(func(r mssql.LookupDatabaseResult) string { return r.Id }).(pulumi.StringOutput)

		if azureDb.Spec.User != nil {
			if azureDb.Spec.ContainedUser {
				username, password, err = deployContainedUser(ctx, provider, azureDb.Name, databaseId,
					azureDb.Spec.User, dbName, []pulumi.Resource{db})
			} else {
				username, password, err = deployLoginUser(ctx, provider, azureDb.Name, databaseId,
					azureDb.Spec.User, dbName, []pulumi.Resource{db})
			}
			if err != nil {
				return nil, err
			}
		}

		if azureDb.Spec.ManagedIdentity != nil {
			identityClientId, identityPrincipalId, err = deployManagedIdentity(ctx, provider, azureDb.Name, databaseId,
				azureDb.Spec.ManagedIdentity, dbName, []pulumi.Resource{db})
			if err != nil {
				return nil, err
			}
		}
	}

	for _, exp := range azureDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbname": {exp.DbName, db.Name}}
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
	ctx.Export(fmt.Sprintf("azureManagedDb:%s", azureDb.Spec.DbName), db.Name)

	return db, nil
}
