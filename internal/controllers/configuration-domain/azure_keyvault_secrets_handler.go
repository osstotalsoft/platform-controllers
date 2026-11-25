package configuration

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

// ReasonDuplicateConfigKey is the reason of the Event recorded when two Azure Key Vault secrets of one group are tagged
// with the same configuration key.
const ReasonDuplicateConfigKey = "DuplicateConfigKey"

// keyVaultListTimeout bounds a listing of a vault's secrets, so that a hung connection doesn't block a worker.
const keyVaultListTimeout = time.Minute

// azureKeyVaultSecretsHandler aggregates a configuration domain's Azure Key Vault secrets into its
// SecretProviderClass.
type azureKeyVaultSecretsHandler struct {
	spc *spcWriter
}

func newAzureKeyVaultSecretsHandler(spc *spcWriter) *azureKeyVaultSecretsHandler {
	return &azureKeyVaultSecretsHandler{spc: spc}
}

// Sync lists the secrets of the platform's vault, picks configDomain's (see selectKeyVaultSecrets), and writes them to
// its SecretProviderClass, in the tenant AZURE_TENANT_ID.
func (h *azureKeyVaultSecretsHandler) Sync(platform *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain) error {
	keyVault := platform.Spec.AzureKeyVault()
	if keyVault == nil {
		return fmt.Errorf("platform %q has no Azure Key Vault", platform.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), keyVaultListTimeout)
	defer cancel()
	secrets, err := listKeyVaultSecrets(ctx, keyVault.Name)
	if err != nil {
		h.spc.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	selected, conflicts := selectKeyVaultSecrets(platform, configDomain, secrets)
	for _, c := range conflicts {
		h.spc.recorder.Event(configDomain, corev1.EventTypeWarning, ReasonDuplicateConfigKey,
			fmt.Sprintf("configuration key %q is in Azure Key Vault secrets %q and %q; using %q", c.ConfigKey, c.Winner, c.Loser, c.Winner))
	}

	return h.spc.Apply(configDomain, azureKeyVaultSpc(configDomain, keyVault, os.Getenv("AZURE_TENANT_ID"), selected))
}

// keyVaultSecret is what the configuration controller knows of an Azure Key Vault secret: never its value.
type keyVaultSecret struct {
	Name    string
	Tags    map[string]string
	Enabled bool
}

// keyVaultConflict is a configuration key that two secrets of the same group are tagged with: Winner is mounted,
// Loser is not.
type keyVaultConflict struct {
	ConfigKey, Winner, Loser string
}

// selectKeyVaultSecrets picks configDomain's secrets out of all the secrets of its platform's vault, by the tags the
// provisioner sets, and returns the name of the secret to mount for each configuration key. The secrets come in the
// same three groups as with Vault, a later group winning on the same key: the platform's global secrets (its target
// namespace, domain global), the namespace's global secrets, and the domain's own secrets. Tenants' secrets are part
// of their group. Secrets without a configKey tag, or disabled, are skipped.
//
// Within a group, two secrets with the same key come from different objects or tenants: the one with the
// alphabetically first name wins, so the result doesn't depend on the vault's listing order, and the others are
// returned as conflicts.
func selectKeyVaultSecrets(platform *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain, secrets []keyVaultSecret) (map[string]string, []keyVaultConflict) {
	type group struct{ namespace, domain string }
	groups := []group{
		{platform.Spec.TargetNamespace, controllers.GlobalDomainLabelValue},
		{configDomain.Namespace, controllers.GlobalDomainLabelValue},
		{configDomain.Namespace, configDomain.Name},
	}

	selected := map[string]string{}
	var conflicts []keyVaultConflict
	seen := map[group]bool{}
	for _, g := range groups {
		// a domain in the platform's target namespace has the same first two groups
		if seen[g] {
			continue
		}
		seen[g] = true

		namesByKey := map[string][]string{}
		for _, s := range secrets {
			key := s.Tags[controllers.AzureKeyVaultTagConfigKey]
			if !s.Enabled || key == "" ||
				s.Tags[controllers.AzureKeyVaultTagPlatform] != platform.Name ||
				s.Tags[controllers.AzureKeyVaultTagNamespace] != g.namespace ||
				s.Tags[controllers.AzureKeyVaultTagDomain] != g.domain {
				continue
			}
			namesByKey[key] = append(namesByKey[key], s.Name)
		}

		for key, names := range namesByKey {
			sort.Strings(names)
			selected[key] = names[0]
			for _, loser := range names[1:] {
				conflicts = append(conflicts, keyVaultConflict{ConfigKey: key, Winner: names[0], Loser: loser})
			}
		}
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].ConfigKey != conflicts[j].ConfigKey {
			return conflicts[i].ConfigKey < conflicts[j].ConfigKey
		}
		return conflicts[i].Loser < conflicts[j].Loser
	})
	return selected, conflicts
}

// azureKeyVaultSpc builds configDomain's aggregated SecretProviderClass for the Azure provider of the Secrets Store CSI
// driver: one object per configuration key, the secret byConfigKey names mounted under the key, as with Vault. Pods
// read the vault with keyVault's workload identity, through its AKS identity binding.
func azureKeyVaultSpc(configDomain *v1alpha1.ConfigurationDomain, keyVault *platformv1.AzureKeyVaultSpec, tenantID string, byConfigKey map[string]string) *csiv1.SecretProviderClass {
	keys := make([]string, 0, len(byConfigKey))
	for key := range byConfigKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var objects strings.Builder
	if len(keys) == 0 {
		objects.WriteString("array: []\n")
	} else {
		objects.WriteString("array:\n")
		for _, key := range keys {
			fmt.Fprintf(&objects, "  - |\n    objectName: %q\n    objectType: secret\n    objectAlias: %q\n", byConfigKey[key], key)
		}
	}

	return &csiv1.SecretProviderClass{
		ObjectMeta: newSpcMeta(configDomain),
		Spec: csiv1.SecretProviderClassSpec{
			Provider: "azure",
			Parameters: map[string]string{
				"keyvaultName": keyVault.Name,
				"tenantId":     tenantID,
				"clientID":     keyVault.WorkloadIdentityClientId,
				// the provider then gets the token through the identity's AKS identity binding, so that any service
				// account the binding's ClusterRole allows can mount the secrets, without a federated credential of its
				// own (provider v1.8.1 or later)
				"useAzureTokenProxy": "true",
				"objects":            objects.String(),
			},
		},
	}
}
