// https://github.com/pulumi/pulumi/tree/master/sdk/go/auto

package pulumi

import (
	"context"
	"fmt"
	"math/rand"
	"os"
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
)

func deployFunc(platform string, tenant *provisioningv1.Tenant, infra *provisioners.InfrastructureManifests) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		resourceGroup, err := azureResources.NewResourceGroup(ctx, fmt.Sprintf("%s_tenant_%s_RG", platform, tenant.Spec.Code), nil)
		if err != nil {
			return err
		}

		if err = createAzureDatabases(ctx, platform, tenant, resourceGroup, infra.AzureDbs); err != nil {
			return err
		}

		if err = createAzureManagedDatabases(ctx, platform, tenant, infra.AzureManagedDbs); err != nil {
			return err
		}
		return nil
	}
}
func createAzureManagedDatabases(ctx *pulumi.Context, platform string,
	tenant *provisioningv1.Tenant,
	azureDbs []*provisioningv1.AzureManagedDatabase) error {

	for _, dbSpec := range azureDbs {
		dbName := fmt.Sprintf("%s_%s_%s", dbSpec.Spec.DbName, platform, tenant.Spec.Code)
		db, err := azureSql.NewManagedDatabase(ctx, dbName, &azureSql.ManagedDatabaseArgs{
			ManagedInstanceName: pulumi.String(dbSpec.Spec.ManagedInstance.Name),
			ResourceGroupName:   pulumi.String(dbSpec.Spec.ManagedInstance.ResourceGroup),
		})
		if err != nil {
			return err
		}

		ctx.Export(fmt.Sprintf("azureManagedDb:%s:%s", dbSpec.Spec.DbName, tenant.Spec.Code), db.Name)
	}
	return nil
}

func createAzureDatabases(ctx *pulumi.Context, platform string,
	tenant *provisioningv1.Tenant,
	resourceGroup *azureResources.ResourceGroup,
	azureDbs []*provisioningv1.AzureDatabase) error {

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
				ResourceGroupName:          resourceGroup.Name,
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
				ResourceGroupName: resourceGroup.Name,
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

func Create(platform string, tenant *provisioningv1.Tenant, infra *provisioners.InfrastructureManifests) error {
	ctx := context.Background()

	stackName := fmt.Sprintf("tenant_%s", tenant.Spec.Code)
	s, err := auto.UpsertStackInlineSource(ctx, stackName, platform, deployFunc(platform, tenant, infra))
	//auto.WorkDir(fmt.Sprintf("~/pulumi_ws/workspace_%s", platform)),
	//auto.EnvVars(map[string]string{})

	if err != nil {
		klog.Errorf("Failed to create or select stack: %v", err)
		return err
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
		return err
	}
	v = os.Getenv(PULUMI_VAULT_PLUGIN_VERSION)
	if v == "" {
		v = "v5.4.0"
	}
	err = w.InstallPlugin(ctx, "vault", v)
	if err != nil {
		klog.Errorf("Failed to install vault plugin: %v", err)
		return err
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
		return err
	}

	klog.V(4).Info("Refresh succeeded!")
	klog.V(4).Info("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	// run the update to deploy our fargate web service
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		klog.Errorf("Failed to update stack: %v", err)
		return err
	}

	klog.V(4).Info("Update succeeded!")
	klog.V(4).Info("Stack results", "Outputs", res.Outputs)

	return nil
}

func getConfigValueFromEnv(key string, secret bool) auto.ConfigValue {
	return auto.ConfigValue{Value: os.Getenv(key), Secret: secret}
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
