package pulumi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	keyvault "github.com/pulumi/pulumi-azure-native-sdk/keyvault/v2"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	vault "github.com/pulumi/pulumi-vault/sdk/v5/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	"totalsoft.ro/platform-controllers/internal/template"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"

	pulumiKube "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	pulumiKubeMetav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
)

const (
	DomainLabel     = "platform.totalsoft.ro/domain"
	PlatformLabel   = "platform.totalsoft.ro/platform"
	EnvVaultEnabled = "VAULT_ENABLED"
)

// randomPasswordSpecialChars is the terraform-random provider's default special-character set
// (see pulumi-random's RandomPasswordArgs.OverrideSpecial doc comment) minus '{' and '}': Prisma
// Migrate rejects DATABASE_URL connection strings whose credentials contain curly braces, so they
// must never appear in a generated password.
const randomPasswordSpecialChars = "!@#$%^&*()-_=+[]<>:?"

// newRandomPassword generates a random password suitable for a SQL login/user or Entra user.
func newRandomPassword(ctx *pulumi.Context, name string) (pulumi.StringOutput, error) {
	randomPassword, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-password", name), &random.RandomPasswordArgs{
		Length:          pulumi.Int(17),
		Upper:           pulumi.Bool(true),
		MinUpper:        pulumi.Int(1),
		Lower:           pulumi.Bool(true),
		MinLower:        pulumi.Int(1),
		Numeric:         pulumi.Bool(true),
		MinNumeric:      pulumi.Int(1),
		Special:         pulumi.Bool(true),
		MinSpecial:      pulumi.Int(1),
		OverrideSpecial: pulumi.String(randomPasswordSpecialChars),
	})
	if err != nil {
		return pulumi.StringOutput{}, err
	}
	return randomPassword.Result, nil
}

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

// handleValueExport returns the exporter of the values of target's resources. imports tells whether the stack is
// recovering a lost state, in which case the Azure Key Vault secrets that already exist are adopted rather than created.
func handleValueExport(target provisioning.ProvisioningTarget, imports *importOptions) ValueExporterFunc {
	templateContext := provisioning.GetTemplateContext(target)

	vaultEnabled, err := strconv.ParseBool(os.Getenv(EnvVaultEnabled))
	if err != nil {
		vaultEnabled = true
	}

	return func(exportContext ExportContext, values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error {
		v := onlyVaultValues(values)
		if len(v) > 0 {
			if !vaultEnabled {
				return fmt.Errorf("vault is not enabled, but there are values to be exported to the vault")
			}

			err := exportToVault(exportContext.pulumiContext, exportPath(target, exportContext), templateContext, v, opts...)
			if err != nil {
				return err
			}
		}

		v = onlyAzureKeyVaultValues(values)
		if len(v) > 0 {
			ctx := exportContext.pulumiContext
			vaultName := config.Get(ctx, azureKeyVaultNameConfigKey(ctx.Project()))
			vaultResourceGroupName := config.Get(ctx, azureKeyVaultResourceGroupNameConfigKey(ctx.Project()))
			if vaultName == "" || vaultResourceGroupName == "" {
				// a tenant whose platform doesn't exist gets no vault either
				return fmt.Errorf("there are values to be exported to Azure Key Vault, but platform %q doesn't exist or has no spec.azureKeyVault, or %s is not true",
					target.GetPlatformName(), EnvAzureEnabled)
			}

			err := exportToAzureKeyVault(ctx, vaultResourceGroupName, vaultName, exportContext.domain, exportPath(target, exportContext),
				azureKeyVaultTags(target, exportContext), templateContext, v, imports, opts...)
			if err != nil {
				return err
			}
		}

		v = onlyConfigMapValues(values)
		if len(v) > 0 {
			name := provisioning.MatchTarget(target,
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

		v = onlyKubeSecretValues(values)
		if len(v) > 0 {
			name := provisioning.MatchTarget(target,
				func(tenant *platformv1.Tenant) string {
					return strings.Join([]string{exportContext.domain, tenant.GetName(), exportContext.objectName}, "-")
				},
				func(*platformv1.Platform) string {
					return strings.Join([]string{exportContext.domain, exportContext.objectName}, "-")
				},
			)

			err := exportToKubeSecret(exportContext, name, templateContext, target.GetPlatformName(), v, opts...)
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

// exportPath identifies the exported object in the platform: the Vault path its values are exported to, which also
// makes the names of the Azure Key Vault secrets its values are exported to unique.
func exportPath(target provisioning.ProvisioningTarget, exportContext ExportContext) string {
	return provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return strings.Join([]string{tenant.Spec.PlatformRef, exportContext.ownerMeta.Namespace, exportContext.domain, tenant.GetName(), exportContext.objectName}, "/")
		},
		func(platform *platformv1.Platform) string {
			return strings.Join([]string{platform.GetName(), exportContext.ownerMeta.Namespace, exportContext.domain, exportContext.objectName}, "/")
		},
	)
}

// The tags of an Azure Key Vault secret, which say where its value is exported from, since the vault is flat.
const (
	AzureKeyVaultTagPlatform  = "platform"
	AzureKeyVaultTagNamespace = "namespace"
	AzureKeyVaultTagDomain    = "domain"
	AzureKeyVaultTagTenant    = "tenant"
	AzureKeyVaultTagObject    = "object"
	AzureKeyVaultTagConfigKey = "configKey"

	azureKeyVaultSecretNameMaxLength = 127
	azureKeyVaultTagValueMaxLength   = 256
	azureKeyVaultSecretHashLength    = 8
)

var azureKeyVaultInvalidSecretNameChars = regexp.MustCompile(`[^0-9A-Za-z-]`)

// The stack configuration keys that hold the platform's Azure Key Vault.
func azureKeyVaultNameConfigKey(projectName string) string {
	return projectName + ":azureKeyVaultName"
}

func azureKeyVaultResourceGroupNameConfigKey(projectName string) string {
	return projectName + ":azureKeyVaultResourceGroupName"
}

// azureKeyVaultConfig is the stack configuration that tells the exporters the platform's Azure Key Vault. It is always
// set, and empty when there is no vault or Azure is not enabled, so that a vault removed from the platform is cleared.
func azureKeyVaultConfig(projectName string, azureEnabled bool, keyVault *platformv1.AzureKeyVaultSpec) map[string]auto.ConfigValue {
	name, resourceGroupName := "", ""
	if azureEnabled && keyVault != nil {
		name, resourceGroupName = keyVault.Name, keyVault.ResourceGroupName
	}
	return map[string]auto.ConfigValue{
		azureKeyVaultNameConfigKey(projectName):              {Value: name},
		azureKeyVaultResourceGroupNameConfigKey(projectName): {Value: resourceGroupName},
	}
}

// azureKeyVaultSecretName names the secret that the value of key, exported from the object at objectPath for domain,
// is exported to: "<domain>-<key>-<hash>". The domain comes first, so that a domain's secrets are listed together.
// Key Vault allows only letters, digits and dashes, up to 127 characters, so "__" becomes "--" (the separator that
// .NET's Key Vault configuration provider maps to ":"), any other character it doesn't allow becomes "-", and the end
// of the key is cut to fit. The hash, of objectPath and key, keeps the same key exported from different platforms,
// namespaces, domains, tenants or objects from colliding. Changing this renames (replaces) every secret.
func azureKeyVaultSecretName(domain, objectPath, key string) string {
	sum := sha256.Sum256([]byte(objectPath + "/" + key))
	hash := hex.EncodeToString(sum[:])[:azureKeyVaultSecretHashLength]

	name := azureKeyVaultSecretNamePart(domain)
	if keyPart := azureKeyVaultSecretNamePart(strings.ReplaceAll(key, "__", "--")); keyPart != "" {
		name = strings.TrimLeft(name+"-"+keyPart, "-")
	}
	if maxLength := azureKeyVaultSecretNameMaxLength - len(hash) - 1; len(name) > maxLength {
		name = strings.TrimRight(name[:maxLength], "-")
	}
	if name == "" {
		return hash
	}
	return name + "-" + hash
}

// azureKeyVaultSecretNamePart replaces the characters Key Vault doesn't allow in names with "-", and trims dashes.
func azureKeyVaultSecretNamePart(s string) string {
	return strings.Trim(azureKeyVaultInvalidSecretNameChars.ReplaceAllString(s, "-"), "-")
}

// azureKeyVaultTags are the tags of the secrets that values exported from exportContext's object are exported to,
// except configKey, which is per value.
func azureKeyVaultTags(target provisioning.ProvisioningTarget, exportContext ExportContext) map[string]string {
	tags := map[string]string{
		AzureKeyVaultTagPlatform:  target.GetPlatformName(),
		AzureKeyVaultTagNamespace: exportContext.ownerMeta.Namespace,
		AzureKeyVaultTagDomain:    exportContext.domain,
		AzureKeyVaultTagObject:    exportContext.objectName,
	}
	if tenant, ok := target.(*platformv1.Tenant); ok {
		tags[AzureKeyVaultTagTenant] = tenant.GetName()
	}
	return tags
}

// exportToAzureKeyVault exports each value to its own secret of the vault: Key Vault has no paths, and
// SecretProviderClasses can't pick keys out of a JSON secret, unlike with Vault. azure-native writes the secrets through
// ARM, and deletes them through the vault's data plane, where they are only soft-deleted: until one is purged or its
// retention ends, a secret with the same name can't be created again (azure-native v2 doesn't recover it).
func exportToAzureKeyVault(ctx *pulumi.Context, vaultResourceGroupName, vaultName, domain, objectPath string, tags map[string]string, templateContext interface{},
	values map[string]exportTemplateWithValue, imports *importOptions, opts ...pulumi.ResourceOption) error {

	valueNames := make([]string, 0, len(values))
	for k := range values {
		valueNames = append(valueNames, k)
	}
	sort.Strings(valueNames)

	exportedBy := map[string]string{}
	for _, valueName := range valueNames {
		v := values[valueName]
		key, err := template.ParseTemplate(v.valueExport.ToAzureKeyVault.KeyTemplate, templateContext)
		if err != nil {
			return err
		}
		if key == "" {
			return fmt.Errorf("the Azure Key Vault keyTemplate of %s renders an empty key", valueName)
		}
		if len(key) > azureKeyVaultTagValueMaxLength {
			return fmt.Errorf("the Azure Key Vault key %q of %s is longer than %d characters", key, valueName, azureKeyVaultTagValueMaxLength)
		}

		secretName := azureKeyVaultSecretName(domain, objectPath, key)
		if other, found := exportedBy[secretName]; found {
			return fmt.Errorf("%s and %s are both exported to the Azure Key Vault key %q", other, valueName, key)
		}
		exportedBy[secretName] = valueName

		secretTags := pulumi.StringMap{AzureKeyVaultTagConfigKey: pulumi.String(key)}
		for k, t := range tags {
			secretTags[k] = pulumi.String(t)
		}

		secretOpts := opts
		if imports.shouldImport(azureKeyVaultSecretType, secretName) {
			importOpts, err := importExistingAzureKeyVaultSecret(ctx, imports, vaultResourceGroupName, vaultName, secretName)
			if err != nil {
				return err
			}
			secretOpts = append(append([]pulumi.ResourceOption{}, opts...), importOpts...)
		}

		_, err = keyvault.NewSecret(ctx, secretName, &keyvault.SecretArgs{
			ResourceGroupName: pulumi.String(vaultResourceGroupName),
			VaultName:         pulumi.String(vaultName),
			SecretName:        pulumi.String(secretName),
			Properties: keyvault.SecretPropertiesArgs{
				// azure-native doesn't mark the value as a secret itself
				Value: pulumi.ToSecret(v.value.ToStringOutput()).(pulumi.StringOutput),
			},
			Tags: secretTags,
		}, secretOpts...)
		if err != nil {
			return err
		}
	}
	return nil
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
				DomainLabel:   exportContext.domain,
				PlatformLabel: platform,
			}),
			OwnerReferences: mapOwnersToReferences(exportContext.ownerMeta, exportContext.ownerKind),
		},
		Immutable: pulumi.Bool(true),
		Data:      data,
	}, opts...)
	return err
}

func exportToKubeSecret(exportContext ExportContext, kubeSecretName string,
	templateContext interface{}, platform string, values map[string]exportTemplateWithValue, opts ...pulumi.ResourceOption) error {

	var parsedKeys = map[string]string{}
	for k, v := range values {
		secretKey, err := template.ParseTemplate(v.valueExport.ToKubeSecret.KeyTemplate, templateContext)
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

	_, err := pulumiKube.NewSecret(exportContext.pulumiContext, kubeSecretName, &pulumiKube.SecretArgs{
		Metadata: pulumiKubeMetav1.ObjectMetaArgs{
			Name:      pulumi.String(kubeSecretName),
			Namespace: pulumi.String(exportContext.ownerMeta.Namespace),
			Labels: pulumi.ToStringMap(map[string]string{
				DomainLabel:   exportContext.domain,
				PlatformLabel: platform,
			}),
			OwnerReferences: mapOwnersToReferences(exportContext.ownerMeta, exportContext.ownerKind),
		},
		Immutable:  pulumi.Bool(true),
		StringData: data,
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

func onlyAzureKeyVaultValues(values map[string]exportTemplateWithValue) map[string]exportTemplateWithValue {
	var output = map[string]exportTemplateWithValue{}
	for k, v := range values {
		if v.valueExport.ToAzureKeyVault != (provisioningv1.AzureKeyVaultSecretTemplate{}) {
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

func onlyKubeSecretValues(values map[string]exportTemplateWithValue) map[string]exportTemplateWithValue {
	var output = map[string]exportTemplateWithValue{}
	for k, v := range values {
		if v.valueExport.ToKubeSecret != (provisioningv1.KubeSecretTemplate{}) {
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
