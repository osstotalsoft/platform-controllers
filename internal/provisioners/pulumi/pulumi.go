// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	"strings"

	"math/rand"
	"os"
	"strconv"
	"time"

	"totalsoft.ro/platform-controllers/internal/template"

	azureResources "github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	azureSql "github.com/pulumi/pulumi-azure-native/sdk/go/azure/sql"

	pulumiKube "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	pulumiKubeMetav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"

	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

const (
	PulumiAzurePluginVersion      = "PULUMI_AZURE_PLUGIN_VERSION"
	PulumiVaultPluginVersion      = "PULUMI_VAULT_PLUGIN_VERSION"
	PulumiKubernetesPluginVersion = "PULUMI_KUBERNETES_PLUGIN_VERSION"

	PulumiSkipAzureManagedDb = "PULUMI_SKIP_AZURE_MANAGED_DB"
	PulumiSkipAzureDb        = "PULUMI_SKIP_AZURE_DB"

	ConfigMapDomainLabel   = "platform.totalsoft.ro/domain"
	ConfigMapPlatformLabel = "platform.totalsoft.ro/platform"
)

func azureRGDeployFunc(platform string, tenant *platformv1.Tenant) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		resourceGroup, err := azureResources.NewResourceGroup(ctx, fmt.Sprintf("%s_%s_RG", platform, tenant.Spec.Code), nil)
		if err != nil {
			return err
		}

		ctx.Export("azureRGName", resourceGroup.Name)

		return nil
	}
}

func azureDbDeployFunc(platform, resourceGroupName string, tenant *platformv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		if len(azureDbs) > 0 {
			const pwdKey = "pass"
			const userKey = "user"
			adminPwd := generatePassword()
			adminUser := fmt.Sprintf("sqlUser%s", tenant.Spec.Code)
			secretPath := fmt.Sprintf("%s/__provisioner/%s/azure-databases/server", platform, tenant.Spec.Code)
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
				sku := "S0"
				if dbSpec.Spec.Sku != "" {
					sku = dbSpec.Spec.Sku
				}
				db, err := azureSql.NewDatabase(ctx, dbSpec.Spec.DbName, &azureSql.DatabaseArgs{
					ResourceGroupName: pulumi.String(resourceGroupName),
					ServerName:        server.Name,
					Sku: &azureSql.SkuArgs{
						Name: pulumi.String(sku),
					},
				})
				if err != nil {
					return err
				}

				ctx.Export(fmt.Sprintf("azureDb:%s", dbSpec.Spec.DbName), db.Name)
				//ctx.Export(fmt.Sprintf("azureDbPassword_%s", dbSpec.Spec.Name), pulumi.ToSecret(pwd))

				err = handleValueExport(ctx, platform, dbSpec.Spec.Domain, dbSpec.Name, tenant,
					dbSpec.Spec.Exports.UserName, dbSpec.Namespace, pulumi.String(adminUser))
				if err != nil {
					return err
				}
				err = handleValueExport(ctx, platform, dbSpec.Spec.Domain, dbSpec.Name, tenant,
					dbSpec.Spec.Exports.Password, dbSpec.Namespace, pulumi.String(adminPwd))
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

func handleValueExport(ctx *pulumi.Context, platform, domain, objectName string, tenant *platformv1.Tenant,
	exportTemplate provisioningv1.ValueExport, namespace string, value pulumi.StringInput) error {
	data := struct {
		Tenant   platformv1.TenantSpec
		Platform string
	}{tenant.Spec, platform}

	if exportTemplate.ToVault != (provisioningv1.VaultSecretTemplate{}) {
		name := objectNamingConvention(platform, domain, tenant.Spec.Code, objectName, "/")
		return exportToVault(ctx, name, exportTemplate.ToVault.KeyTemplate, data, value)
	}

	if exportTemplate.ToConfigMap != (provisioningv1.ConfigMapTemplate{}) {
		name := objectNamingConvention(platform, domain, tenant.Spec.Code, objectName, "-")
		return exportToConfigMap(ctx, name, exportTemplate.ToConfigMap.KeyTemplate, data, namespace, domain, platform, value)
	}
	return nil
}

func exportToVault(ctx *pulumi.Context, secretPath, keyTemplate string,
	templateContext interface{}, value pulumi.StringInput) error {
	secretKey, err := template.ParseTemplate(keyTemplate, templateContext)
	if err != nil {
		return err
	}

	dataJson := value.ToStringOutput().ApplyT(func(v string) string {
		return fmt.Sprintf(`{"%s":"%s"}`, secretKey, v)
	}).(pulumi.StringOutput)
	_, err = vault.NewSecret(ctx, secretPath, &vault.SecretArgs{
		DataJson: dataJson,
		Path:     pulumi.String(secretPath),
	})
	return err
}

func exportToConfigMap(ctx *pulumi.Context, configMapName, keyTemplate string,
	templateContext interface{}, namespace, domain, platform string, value pulumi.StringInput) error {
	configMapKey, err := template.ParseTemplate(keyTemplate, templateContext)
	if err != nil {
		return err
	}
	data := value.ToStringOutput().ApplyT(func(v string) map[string]string {
		return map[string]string{configMapKey: v}
	}).(pulumi.StringMapOutput)

	_, err = pulumiKube.NewConfigMap(ctx, configMapName, &pulumiKube.ConfigMapArgs{
		Metadata: pulumiKubeMetav1.ObjectMetaArgs{
			Name:      pulumi.String(configMapName),
			Namespace: pulumi.String(namespace),
			Labels: pulumi.ToStringMap(map[string]string{
				ConfigMapDomainLabel:   domain,
				ConfigMapPlatformLabel: platform,
			}),
		},
		Immutable: pulumi.Bool(true),
		Data:      data,
	})
	return err
}

func azureManagedDbDeployFunc(platform string, tenant *platformv1.Tenant, azureDbs []*provisioningv1.AzureManagedDatabase) pulumi.RunFunc {
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

			err = handleValueExport(ctx, platform, dbSpec.Spec.Domain, dbSpec.Name, tenant,
				dbSpec.Spec.Exports.DbName, dbSpec.Namespace, db.Name)
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("azureManagedDb:%s", dbSpec.Spec.DbName), db.Name)
		}
		return nil
	}
}

func Create(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {

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

	if s, _ := strconv.ParseBool(os.Getenv(PulumiSkipAzureDb)); !s {
		res, err = updateStack(azureDbStackName, platform, azureDbDeployFunc(platform, azureRGName, tenant, infra.AzureDbs))
		if err != nil {
			return err
		}
	}

	if s, _ := strconv.ParseBool(os.Getenv(PulumiSkipAzureManagedDb)); !s {
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
	v := os.Getenv(PulumiAzurePluginVersion)
	if v == "" {
		v = "v1.62.0"
	}
	err = w.InstallPlugin(ctx, "azure-native", v)
	if err != nil {
		klog.Errorf("Failed to install azure-native plugin: %v", err)
		return auto.Stack{}, err
	}
	v = os.Getenv(PulumiVaultPluginVersion)
	if v == "" {
		v = "v5.4.0"
	}
	err = w.InstallPlugin(ctx, "vault", v)
	if err != nil {
		klog.Errorf("Failed to install vault plugin: %v", err)
		return auto.Stack{}, err
	}
	v = os.Getenv(PulumiKubernetesPluginVersion)
	if v == "" {
		v = "v3.18.2"
	}
	err = w.InstallPlugin(ctx, "kubernetes", v)
	if err != nil {
		klog.Errorf("Failed to install kubernetes plugin: %v", err)
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

func objectNamingConvention(platform, domain, tenant, object, separator string) string {
	return strings.Join([]string{platform, domain, tenant, object}, separator)
}
