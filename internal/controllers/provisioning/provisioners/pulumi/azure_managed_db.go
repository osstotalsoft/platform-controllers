package pulumi

import (
	"fmt"
	"strings"

	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureManagedDbDeployFunc(platform string, tenant *platformv1.Tenant,
	azureDbs []*provisioningv1.AzureManagedDatabase) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, tenant)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureManagedDatabase")
	return func(ctx *pulumi.Context) error {
		for _, dbSpec := range azureDbs {
			dbNameV1 := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, tenant.Name)
			dbName := strings.ReplaceAll(dbNameV1, ".", "_")
			args := azureSql.ManagedDatabaseArgs{
				ManagedInstanceName: pulumi.String(dbSpec.Spec.ManagedInstance.Name),
				ResourceGroupName:   pulumi.String(dbSpec.Spec.ManagedInstance.ResourceGroup),
			}
			restoreFrom := dbSpec.Spec.RestoreFrom
			if (restoreFrom != provisioningv1.AzureManagedDatabaseRestoreSpec{}) {
				args.CreateMode = pulumi.String("RestoreExternalBackup")
				args.AutoCompleteRestore = pulumi.Bool(true)
				args.LastBackupName = pulumi.String(restoreFrom.BackupFileName)
				args.StorageContainerSasToken = pulumi.String(restoreFrom.StorageContainer.SasToken)
				args.StorageContainerUri = pulumi.String(restoreFrom.StorageContainer.Uri)
			}
			ignoreChanges := []string{}
			if PulumiRetainOnDelete {
				ignoreChanges = []string{"lastBackupName", "storageContainerUri"}
			}

			db, err := azureSql.NewManagedDatabase(ctx, dbName, &args,
				pulumi.RetainOnDelete(PulumiRetainOnDelete),
				pulumi.IgnoreChanges(ignoreChanges),
				pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String(dbNameV1)}}),
			)
			if err != nil {
				return err
			}

			for _, exp := range dbSpec.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, dbSpec.Name, dbSpec.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{"dbname": {exp.DbName, db.Name}})
				if err != nil {
					return err
				}
			}
			ctx.Export(fmt.Sprintf("azureManagedDb:%s", dbSpec.Spec.DbName), db.Name)
		}
		return nil
	}
}
