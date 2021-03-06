package pulumi

import (
	"fmt"

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
			dbName := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, tenant.Name)
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
			ignoreProps := []string{}
			if PulumiRetainOnDelete {
				ignoreProps = []string{"lastBackupName", "storageContainerUri"}
			}

			db, err := azureSql.NewManagedDatabase(ctx, dbName, &args,
				pulumi.RetainOnDelete(PulumiRetainOnDelete),
				pulumi.IgnoreChanges(ignoreProps),
			)

			if err != nil {
				return err
			}

			for _, domain := range dbSpec.Spec.Domains {
				err = valueExporter(newExportContext(ctx, domain, dbSpec.Name, dbSpec.ObjectMeta, gvk),
					dbSpec.Spec.Exports.DbName, db.Name)
				if err != nil {
					return err
				}
			}
			ctx.Export(fmt.Sprintf("azureManagedDb:%s", dbSpec.Spec.DbName), db.Name)
		}
		return nil
	}
}
