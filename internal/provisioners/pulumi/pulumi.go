// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	azureResources "github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"
	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

const (
	PULUMI_AZURE_PLUGIN_VERSION = "PULUMI_AZURE_PLUGIN_VERSION"
	PULUMI_VAULT_PLUGIN_VERSION = "PULUMI_VAULT_PLUGIN_VERSION"

	PULUMI_SKIP_AZURE_MANAGED_DB = "PULUMI_SKIP_AZURE_MANAGED_DB"
	PULUMI_SKIP_AZURE_DB         = "PULUMI_SKIP_AZURE_DB"
)

func azureRGDeployFunc(platform string, tenant *provisioningv1.Tenant) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		resourceGroup, err := azureResources.NewResourceGroup(ctx, fmt.Sprintf("%s_%s_RG", platform, tenant.Spec.Code), nil)
		if err != nil {
			return err
		}

		ctx.Export("azureRGName", resourceGroup.Name)

		return nil
	}
}

func azureDbDeployFunc(platform, resourceGroupName string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		if len(azureDbs) > 0 {
			const pwdKey = "pass"
			const userKey = "user"
			adminPwd := generatePassword()
			adminUser := fmt.Sprintf("sqlUser%s", tenant.Spec.Code)
			secretPath := fmt.Sprintf("%s/%s/azure-databases/server", platform, tenant.Spec.Code)
			secret, err := vault.LookupSecret(ctx, &vault.LookupSecretArgs{Path: secretPath})
			if secret != nil && secret.Data[pwdKey] != nil {
				adminPwd = secret.Data[pwdKey].(string)
			}

			server, err := azureSql.NewServer(ctx, fmt.Sprintf("%s-sqlserver", tenant.Spec.Code),
				&azureSql.ServerArgs{
					ResourceGroupName:          pulumi.String(resourceGroupName),
					AdministratorLogin:         pulumi.String(adminUser),
					AdministratorLoginPassword: pulumi.String(adminPwd),
					Version:                    pulumi.String("12.0"),
				})
			if err != nil {
				return err
			}

			//pool, err := azureSql.NewElasticPool(ctx, fmt.Sprintf("%s-sqlserver-pool", tenant.Spec.Code),
			//	&azureSql.ElasticPoolArgs{
			//		ResourceGroupName: resourceGroup.Name,
			//		ServerName:        server.Name,
			//	})
			//if err != nil {
			//	return err
			//}

			for _, dbSpec := range azureDbs {
				db, err := azureSql.NewDatabase(ctx, dbSpec.Spec.DbName, &azureSql.DatabaseArgs{
					ResourceGroupName: pulumi.String(resourceGroupName),
					ServerName:        server.Name,
					Sku: &azureSql.SkuArgs{
						Name: pulumi.String("S0"),
					},
				})
				if err != nil {
					return err
				}

				ctx.Export(fmt.Sprintf("azureDb:%s", dbSpec.Spec.DbName), db.Name)
				//ctx.Export(fmt.Sprintf("azureDbPassword_%s", dbSpec.Spec.Name), pulumi.ToSecret(pwd))

				dbSecretPath := fmt.Sprintf("%s/%s/azure-databases/%s", platform, tenant.Spec.Code, dbSpec.Spec.DbName)
				_, err = vault.NewSecret(ctx, dbSecretPath, &vault.SecretArgs{
					DataJson: pulumi.String(fmt.Sprintf(`{"%s":"%s", "%s":"%s"}`, pwdKey, adminPwd, userKey, adminUser)),
					Path:     pulumi.String(dbSecretPath),
				})
				if err != nil {
					return err
				}
			}

			_, err = vault.NewSecret(ctx, secretPath, &vault.SecretArgs{
				DataJson: pulumi.String(fmt.Sprintf(`{"%s":"%s", "%s":"%s"}`, pwdKey, adminPwd, userKey, adminUser)),
				Path:     pulumi.String(secretPath),
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func azureManagedDbDeployFunc(platform string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureManagedDatabase) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		for _, dbSpec := range azureDbs {
			dbName := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, tenant.Spec.Code)
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
			db, err := azureSql.NewManagedDatabase(ctx, dbName, &args)
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("azureManagedDb:%s", dbSpec.Spec.DbName), db.Name)
		}
		return nil
	}
}

func Create(platform string, tenant *provisioningv1.Tenant, infra *provisioners.InfrastructureManifests) error {

	azureRGStackName := fmt.Sprintf("%s_rg", tenant.Spec.Code)
	res, err := updateStack(azureRGStackName, platform, azureRGDeployFunc(platform, tenant))
	if err != nil {
		return err
	}

	azureDbStackName := fmt.Sprintf("%s_azure_db", tenant.Spec.Code)
	azureRGName, ok := res.Outputs["azureRGName"].Value.(string)
	if !ok {
		klog.Errorf("Failed to get azureRGName: %v", err)
		return err
	}

	if s, _ := strconv.ParseBool(os.Getenv(PULUMI_SKIP_AZURE_DB)); !s {
		res, err = updateStack(azureDbStackName, platform, azureDbDeployFunc(platform, azureRGName, tenant, infra.AzureDbs))
		if err != nil {
			return err
		}
	}

	if s, _ := strconv.ParseBool(os.Getenv(PULUMI_SKIP_AZURE_MANAGED_DB)); !s {
		azureManagedDbStackName := fmt.Sprintf("%s_azure_managed_db", tenant.Spec.Code)
		res, err = updateStack(azureManagedDbStackName, platform, azureManagedDbDeployFunc(platform, tenant, infra.AzureManagedDbs))
		if err != nil {
			return err
		}
	}
	return nil
}

func updateStack(stackName, projectName string, deployFunc pulumi.RunFunc) (auto.UpResult, error) {
	ctx := context.Background()

	s, err := createOrSelectStack(ctx, stackName, projectName, deployFunc)
	if err != nil {
		klog.ErrorS(err, "Failed to create or select stack", "name", stackName)
		return auto.UpResult{}, err
	}
	klog.V(4).InfoS("Starting stack update", "name", stackName)

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		klog.ErrorS(err, "Failed to update stack", "name", stackName)
		return auto.UpResult{}, err
	}
	klog.V(4).InfoS("Stack update succeeded!", "name", stackName)
	klog.V(4).InfoS("Stack results", "name", stackName, "Outputs", res.Outputs)

	return res, err
}

func createOrSelectStack(ctx context.Context, stackName, projectName string, deployFunc pulumi.RunFunc) (auto.Stack, error) {
	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, deployFunc)
	//auto.EnvVars(map[string]string{})
	if err != nil {
		klog.Errorf("Failed to create or select stack: %v", err)
		return auto.Stack{}, err
	}

	klog.V(4).Info("Installing plugins")
	w := s.Workspace()

	// for inline source programs, we must manage plugins ourselves
	v := os.Getenv(PULUMI_AZURE_PLUGIN_VERSION)
	if v == "" {
		v = "v1.62.0"
	}
	err = w.InstallPlugin(ctx, "azure-native", v)
	if err != nil {
		klog.Errorf("Failed to install azure-native plugin: %v", err)
		return auto.Stack{}, err
	}
	v = os.Getenv(PULUMI_VAULT_PLUGIN_VERSION)
	if v == "" {
		v = "v5.4.0"
	}
	err = w.InstallPlugin(ctx, "vault", v)
	if err != nil {
		klog.Errorf("Failed to install vault plugin: %v", err)
		return auto.Stack{}, err
	}
	klog.V(4).Info("Successfully installed plugins")

	// set stack configuration
	_ = s.SetAllConfig(ctx, map[string]auto.ConfigValue{
		"azure-native:location":       {Value: os.Getenv("AZURE_LOCATION")},
		"azure-native:clientId":       {Value: os.Getenv("AZURE_CLIENT_ID")},
		"azure-native:subscriptionId": {Value: os.Getenv("AZURE_SUBSCRIPTION_ID")},
		"azure-native:tenantId":       {Value: os.Getenv("AZURE_TENANT_ID")},
		"azure-native:clientSecret":   {Value: os.Getenv("AZURE_CLIENT_SECRET"), Secret: true}})

	klog.V(4).Info("Successfully set config")
	klog.V(4).Info("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		klog.Errorf("Failed to refresh stack: %v", err)
		return auto.Stack{}, err
	}

	klog.V(4).Info("Refresh succeeded!")
	return s, nil
}

func generatePassword() string {
	rand.Seed(time.Now().UnixNano())
	digits := "0123456789"
	specials := "~=+%^*/()[]{}/!@#$?|"
	all := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		digits + specials
	length := 24
	buf := make([]byte, length)
	buf[0] = digits[rand.Intn(len(digits))]
	buf[1] = specials[rand.Intn(len(specials))]
	for i := 2; i < length; i++ {
		buf[i] = all[rand.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})
	return string(buf)
}
