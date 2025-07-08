package pulumi

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	mssql "github.com/pulumiverse/pulumi-mssql/sdk/go/mssql"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"

	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployMsSqlDb(target provisioning.ProvisioningTarget,
	mssqlDb *provisioningv1.MsSqlDatabase,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*mssql.Database, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("MsSqlDatabase")

	provider, err := mssql.NewProvider(ctx, "provider-mssql", &mssql.ProviderArgs{
		Hostname: pulumi.String(mssqlDb.Spec.SqlServer.HostName),
		Port:     pulumi.Int(mssqlDb.Spec.SqlServer.Port),
		SqlAuth: &mssql.ProviderSqlAuthArgs{
			Username: pulumi.String(mssqlDb.Spec.SqlServer.SqlAuth.Username),
			Password: pulumi.String(mssqlDb.Spec.SqlServer.SqlAuth.Password),
		},
	})
	if err != nil {
		return nil, err
	}

	dbName := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s_%s_%s", mssqlDb.Spec.DbName, tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s_%s", mssqlDb.Spec.DbName, platform.GetName())
		},
	)
	dbName = strings.ReplaceAll(dbName, ".", "_")

	pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources
	ignoreChanges := []string{}
	if pulumiRetainOnDelete {
		ignoreChanges = []string{"name", "collation"}
	}

	db, err := mssql.NewDatabase(ctx, mssqlDb.Name, &mssql.DatabaseArgs{
		Name: pulumi.String(dbName),
	},
		pulumi.Provider(provider),
		pulumi.RetainOnDelete(pulumiRetainOnDelete),
		pulumi.IgnoreChanges(ignoreChanges),
		pulumi.Import(pulumi.ID(mssqlDb.Spec.ImportDatabaseId)),
		pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	restoreFrom := mssqlDb.Spec.RestoreFrom
	if restoreFrom.BackupFilePath != "" {
		_, err = mssql.NewScript(ctx, "restore-db", &mssql.ScriptArgs{
			DatabaseId: db.ID(),
			ReadScript: pulumi.String("SELECT CASE WHEN EXISTS (SELECT 1 FROM sys.tables) THEN 'Initialized' ELSE 'Empty' END AS [DatabaseStatus]"),
			UpdateScript: db.Name.ApplyT(func(n string) pulumi.StringOutput {
				return pulumi.Sprintf(`
IF (SELECT COUNT(1) FROM sys.tables) = 0
BEGIN
	USE master; 
	RESTORE DATABASE [%v] FROM DISK = '%v' WITH REPLACE; 
END
				`, n, mssqlDb.Spec.RestoreFrom.BackupFilePath)
			}).(pulumi.StringOutput),
			State: pulumi.StringMap{
				"DatabaseStatus": pulumi.String("Initialized"),
			},
		},
			pulumi.Provider(provider),
			pulumi.Parent(db))
		if err != nil {
			return nil, err
		}
	}

	ctx.Export("mssqlDbName", db.Name)

	for _, exp := range mssqlDb.Spec.Exports {
		err = valueExporter(newExportContext(ctx, exp.Domain, mssqlDb.Name, mssqlDb.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{"dbName": {exp.DbName, db.Name}})
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}
