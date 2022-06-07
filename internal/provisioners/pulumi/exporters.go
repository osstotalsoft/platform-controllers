package pulumi

import (
	"fmt"
	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/template"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"

	pulumiKube "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	pulumiKubeMetav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
)

const (
	ConfigMapDomainLabel   = "platform.totalsoft.ro/domain"
	ConfigMapPlatformLabel = "platform.totalsoft.ro/platform"
)

type ValueExporterFunc func(exportContext ExportContext,
	exportTemplate provisioningv1.ValueExport, value pulumi.StringInput) error

type ExportContext struct {
	pulumiContext *pulumi.Context
	namespace     string
	domain        string
	objectName    string
}

func newExportContext(pulumiContext *pulumi.Context, namespace, domain, objectName string) ExportContext {
	return ExportContext{
		pulumiContext: pulumiContext,
		namespace:     namespace,
		domain:        domain,
		objectName:    objectName,
	}
}

func handleValueExport(platform string, tenant *platformv1.Tenant) ValueExporterFunc {
	data := struct {
		Tenant   platformv1.TenantSpec
		Platform string
	}{tenant.Spec, platform}

	return func(exportContext ExportContext, exportTemplate provisioningv1.ValueExport, value pulumi.StringInput) error {

		if exportTemplate.ToVault != (provisioningv1.VaultSecretTemplate{}) {
			name := objectNamingConvention(platform, exportContext.domain, tenant.Spec.Code,
				exportContext.objectName, "/")
			return exportToVault(exportContext.pulumiContext, name, exportTemplate.ToVault.KeyTemplate, data, value)
		}

		if exportTemplate.ToConfigMap != (provisioningv1.ConfigMapTemplate{}) {
			name := objectNamingConvention(platform, exportContext.domain, tenant.Spec.Code,
				exportContext.objectName, "-")
			return exportToConfigMap(exportContext.pulumiContext, name, exportTemplate.ToConfigMap.KeyTemplate, data,
				exportContext.namespace, exportContext.domain, platform, value)
		}
		return nil
	}
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
