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

	for _, exp := range azureDb.Spec.Exports {
		err = valueExporter(newExportContext(ctx, exp.Domain, azureDb.Name, azureDb.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{"dbname": {exp.DbName, db.Name}})
		if err != nil {
			return nil, err
		}
	}
	ctx.Export(fmt.Sprintf("azureManagedDb:%s", azureDb.Spec.DbName), db.Name)

	return db, nil
}
