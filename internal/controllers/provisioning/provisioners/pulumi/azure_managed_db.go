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

func deployAzureManagedDb(
	target provisioning.ProvisioningTarget,
	azureDb *provisioningv1.AzureManagedDatabase,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*azureSql.ManagedDatabase, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureManagedDatabase")

	if err := validateUniqueNames(azureDb.Spec.Users, "users"); err != nil {
		return nil, err
	}
	if err := validateUniqueNames(azureDb.Spec.ManagedIdentities, "managedIdentities"); err != nil {
		return nil, err
	}
	if err := validateNoCrossListNameCollision(azureDb.Spec.Users, azureDb.Spec.ManagedIdentities); err != nil {
		return nil, err
	}
	if err := validateUniqueDomains(azureDb.Spec.Exports, "exports"); err != nil {
		return nil, err
	}

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

	usersByName := map[string]deployedUser{}
	identitiesByName := map[string]deployedIdentity{}
	if len(azureDb.Spec.Users) > 0 || len(azureDb.Spec.ManagedIdentities) > 0 {
		// The managed instance's real T-SQL endpoint FQDN is
		// "<mi-name>.<dnsZone>.database.windows.net", where dnsZone is an Azure-generated
		// virtual-cluster identifier that is NOT the resource group — it cannot be constructed from
		// spec fields alone, so it's resolved via LookupManagedInstance (mirroring how
		// azureSql.LookupServer resolves the logical-server FQDN in azure_db.go). Port is left at the
		// mssql provider's default (1433): this repo has no public-vs-private MI endpoint toggle
		// (AzureManagedInstanceSpec carries no such field, unlike e.g. AzureVirtualMachine/
		// AzureVirtualDesktop, which reference an explicit VNet subnet) and every other resource that
		// talks to Azure SQL in this repo assumes private/VNet connectivity, where the MI's regular
		// 1433 endpoint (not the public 3342 one) applies.
		mi, err := azureSql.LookupManagedInstance(ctx, &azureSql.LookupManagedInstanceArgs{
			ManagedInstanceName: azureDb.Spec.ManagedInstance.Name,
			ResourceGroupName:   azureDb.Spec.ManagedInstance.ResourceGroup,
		})
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, fmt.Errorf("managedInstance %s not found", azureDb.Spec.ManagedInstance.Name)
		}

		provider, err := newMssqlAzureAuthProvider(ctx, azureDb.Name, mi.FullyQualifiedDomainName)
		if err != nil {
			return nil, err
		}

		dbLookup := mssql.LookupDatabaseOutput(ctx, mssql.LookupDatabaseOutputArgs{
			Name: db.Name,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{db}))
		databaseId := dbLookup.ApplyT(func(r mssql.LookupDatabaseResult) string { return r.Id }).(pulumi.StringOutput)

		for i := range azureDb.Spec.Users {
			user := azureDb.Spec.Users[i]
			resourceNamePrefix := fmt.Sprintf("%s-%s", azureDb.Name, user.Name)
			var username string
			var password pulumi.StringOutput
			if azureDb.Spec.ContainedUser {
				username, password, err = deployContainedUser(ctx, provider, resourceNamePrefix, databaseId,
					&user, user.Name, []pulumi.Resource{db}, pulumiRetainOnDelete)
			} else {
				username, password, err = deployLoginUser(ctx, provider, resourceNamePrefix, databaseId,
					&user, dbName, []pulumi.Resource{db}, pulumiRetainOnDelete)
			}
			if err != nil {
				return nil, err
			}
			usersByName[user.Name] = deployedUser{username: username, password: password}
		}

		for i := range azureDb.Spec.ManagedIdentities {
			identity := azureDb.Spec.ManagedIdentities[i]
			clientId, principalId, err := deployManagedIdentity(ctx, provider, fmt.Sprintf("%s-%s", azureDb.Name, identity.Name), databaseId,
				&identity, dbName, []pulumi.Resource{db}, pulumiRetainOnDelete)
			if err != nil {
				return nil, err
			}
			identitiesByName[identity.Name] = deployedIdentity{clientId: clientId, principalId: principalId}
		}
	}

	for _, exp := range azureDb.Spec.Exports {
		values := map[string]exportTemplateWithValue{"dbname": {exp.DbName, db.Name}}

		if exp.UserRef != "" || exp.Username != (provisioningv1.ValueExport{}) || exp.Password != (provisioningv1.ValueExport{}) {
			user, err := resolveRef(usersByName, exp.UserRef, exp.Domain, "userRef", "users")
			if err != nil {
				return nil, err
			}
			values["username"] = exportTemplateWithValue{exp.Username, pulumi.String(user.username)}
			values["password"] = exportTemplateWithValue{exp.Password, user.password}
		}

		if exp.IdentityRef != "" || exp.IdentityClientId != (provisioningv1.ValueExport{}) || exp.IdentityPrincipalId != (provisioningv1.ValueExport{}) {
			identity, err := resolveRef(identitiesByName, exp.IdentityRef, exp.Domain, "identityRef", "managedIdentities")
			if err != nil {
				return nil, err
			}
			values["identityClientId"] = exportTemplateWithValue{exp.IdentityClientId, identity.clientId}
			values["identityPrincipalId"] = exportTemplateWithValue{exp.IdentityPrincipalId, identity.principalId}
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
