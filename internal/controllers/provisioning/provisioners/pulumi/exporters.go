package pulumi

import (
	"encoding/json"
	"strings"

	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
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

type ValueExporterFunc func(exportContext ExportContext, values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error

type exportTemplateWithValue struct {
	valueExport provisioningv1.ValueExport
	value       pulumi.StringInput
}

type ExportContext struct {
	pulumiContext *pulumi.Context
	domain        string
	objectName    string
	ownerMeta     metav1.ObjectMeta
	ownerKind     k8sSchema.GroupVersionKind
}

func newExportContext(pulumiContext *pulumi.Context, domain, objectName string,
	ownerMeta metav1.ObjectMeta, ownerKind k8sSchema.GroupVersionKind) ExportContext {
	return ExportContext{
		pulumiContext: pulumiContext,
		ownerMeta:     ownerMeta,
		ownerKind:     ownerKind,
		domain:        domain,
		objectName:    objectName,
	}
}

func handleValueExport(target provisioning.ProvisioningTarget) ValueExporterFunc {
	templateContext := provisioning.GetTemplateContext(target)

	return func(exportContext ExportContext, values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error {
		v := onlyVaultValues(values)
		if len(v) > 0 {
			path := provisioning.Match(target,
				func(tenant *platformv1.Tenant) string {
					return strings.Join([]string{tenant.Spec.PlatformRef, exportContext.ownerMeta.Namespace, exportContext.domain, tenant.GetName(), exportContext.objectName}, "/")
				},
				func(platform *platformv1.Platform) string {
					return strings.Join([]string{platform.GetName(), exportContext.ownerMeta.Namespace, exportContext.domain, exportContext.objectName}, "/")
				},
			)

			err := exportToVault(exportContext.pulumiContext, path, templateContext, v, opts...)
			if err != nil {
				return err
			}
		}

		v = onlyConfigMapValues(values)
		if len(v) > 0 {
			name := provisioning.Match(target,
				func(tenant *platformv1.Tenant) string {
					return strings.Join([]string{exportContext.domain, tenant.GetName(), exportContext.objectName}, "-")
				},
				func(*platformv1.Platform) string {
					return strings.Join([]string{exportContext.domain, exportContext.objectName}, "-")
				},
			)

			err := exportToConfigMap(exportContext, name, templateContext, target.GetPlatformName(), v, opts...)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func exportToVault(ctx *pulumi.Context, secretPath string, templateContext interface{},
	values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error {

	var parsedKeys = map[string]string{}
	for k, v := range values {
		secretKey, err := template.ParseTemplate(v.valueExport.ToVault.KeyTemplate, templateContext)
		if err != nil {
			return err
		}
		parsedKeys[k] = secretKey
	}

	var pulumiValues = pulumi.StringMap{}
	for k, v := range values {
		pulumiValues[k] = v.value
	}

	dataJson := pulumiValues.ToStringMapOutput().ApplyT(func(vs map[string]string) string {
		var m = map[string]string{}
		for k, v := range vs {
			m[parsedKeys[k]] = v
		}
		result, _ := json.Marshal(m)
		return string(result)
	}).(pulumi.StringOutput)

	_, err := vault.NewSecret(ctx, secretPath, &vault.SecretArgs{
		DataJson:          dataJson,
		Path:              pulumi.String(secretPath),
		DeleteAllVersions: pulumi.Bool(true),
	}, opts...)
	return err
}

func exportToConfigMap(exportContext ExportContext, configMapName string,
	templateContext interface{}, platform string, values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error {

	var parsedKeys = map[string]string{}
	for k, v := range values {
		secretKey, err := template.ParseTemplate(v.valueExport.ToConfigMap.KeyTemplate, templateContext)
		if err != nil {
			return err
		}
		parsedKeys[k] = secretKey
	}

	var pulumiValues = pulumi.StringMap{}
	for k, v := range values {
		pulumiValues[k] = v.value
	}

	data := pulumiValues.ToStringMapOutput().ApplyT(func(vs map[string]string) map[string]string {
		var m = map[string]string{}
		for k, v := range vs {
			m[parsedKeys[k]] = v
		}
		return m
	}).(pulumi.StringMapOutput)

	_, err := pulumiKube.NewConfigMap(exportContext.pulumiContext, configMapName, &pulumiKube.ConfigMapArgs{
		Metadata: pulumiKubeMetav1.ObjectMetaArgs{
			Name:      pulumi.String(configMapName),
			Namespace: pulumi.String(exportContext.ownerMeta.Namespace),
			Labels: pulumi.ToStringMap(map[string]string{
				ConfigMapDomainLabel:   exportContext.domain,
				ConfigMapPlatformLabel: platform,
			}),
			OwnerReferences: mapOwnersToReferences(exportContext.ownerMeta, exportContext.ownerKind),
		},
		Immutable: pulumi.Bool(true),
		Data:      data,
	}, opts...)
	return err
}

func onlyVaultValues(values map[string]exportTemplateWithValue) map[string]exportTemplateWithValue {
	var output = map[string]exportTemplateWithValue{}
	for k, v := range values {
		if v.valueExport.ToVault != (provisioningv1.VaultSecretTemplate{}) {
			output[k] = v
		}
	}
	return output
}

func onlyConfigMapValues(values map[string]exportTemplateWithValue) map[string]exportTemplateWithValue {
	var output = map[string]exportTemplateWithValue{}
	for k, v := range values {
		if v.valueExport.ToConfigMap != (provisioningv1.ConfigMapTemplate{}) {
			output[k] = v
		}
	}
	return output
}

func mapOwnersToReferences(ownerMeta metav1.ObjectMeta, ownerKind k8sSchema.GroupVersionKind) pulumiKubeMetav1.OwnerReferenceArray {
	return pulumiKubeMetav1.OwnerReferenceArray{
		pulumiKubeMetav1.OwnerReferenceArgs{
			ApiVersion:         pulumi.String(ownerKind.GroupVersion().String()),
			Kind:               pulumi.String(ownerKind.Kind),
			Name:               pulumi.String(ownerMeta.GetName()),
			Uid:                pulumi.String(ownerMeta.GetUID()),
			Controller:         pulumi.Bool(true),
			BlockOwnerDeletion: pulumi.Bool(true),
		},
	}
}
