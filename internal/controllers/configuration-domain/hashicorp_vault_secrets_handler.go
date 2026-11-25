package configuration

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	controllers "totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"

	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
)

var getSecrets = getSecretWithKubernetesAuth

type secretSpec struct {
	Path string `json:"platformRef"`
	Key  string `json:"domain"`
}
// secretsHandler aggregates a configuration domain's HashiCorp Vault secrets into its SecretProviderClass.
type secretsHandler struct {
	spc *spcWriter
}

func newVaultSecretsHandler(spc *spcWriter) *secretsHandler {
	return &secretsHandler{spc: spc}
}

func (c *secretsHandler) Sync(platformObj *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain) error {
	role := fmt.Sprintf("%s-readonly", platformObj.Name)

	platformSecrets, err := getSecrets(platformObj.Name, platformObj.Spec.TargetNamespace, controllers.GlobalDomainLabelValue, role)
	if err != nil {
		return err
	}

	globalSecrets, err := getSecrets(platformObj.Name, configDomain.Namespace, controllers.GlobalDomainLabelValue, role)
	if err != nil {
		return err
	}

	secrets, err := getSecrets(platformObj.Name, configDomain.Namespace, configDomain.Name, role)
	if err != nil {
		return err
	}

	secrets = append(append(platformSecrets, globalSecrets...), secrets...)

	return c.spc.Apply(configDomain, c.aggregateSecrets(configDomain, secrets, role))
}

func (c *secretsHandler) aggregateSecrets(configurationDomain *v1alpha1.ConfigurationDomain, secrets []secretSpec, role string) *csiv1.SecretProviderClass {
	mergedSecrets := map[string]secretSpec{}
	for _, secret := range secrets {
		if existingValue, ok := mergedSecrets[secret.Key]; ok && existingValue != secret {
			klog.V(4).Infof("Key %s already exists with value %s. It will be replaced by secret %s with value %s", secret.Key, existingValue, secret.Key, secret)
		}
		mergedSecrets[secret.Key] = secret
	}
	var orderedSecretKeys []string
	for key := range mergedSecrets {
		orderedSecretKeys = append(orderedSecretKeys, key)
	}
	sort.Strings(orderedSecretKeys)

	var sb strings.Builder
	for _, secretKey := range orderedSecretKeys {
		objString := fmt.Sprintf(`
- objectName: "%s"
  secretPath: "%s"
  secretKey: "%s"`, secretKey, mergedSecrets[secretKey].Path, mergedSecrets[secretKey].Key)
		sb.WriteString(objString)
	}
	objectsString := sb.String()

	vaultAddress := os.Getenv("VAULT_ADDR") //"http://vault.default:8200"

	return &csiv1.SecretProviderClass{
		ObjectMeta: newSpcMeta(configurationDomain),
		Spec: csiv1.SecretProviderClassSpec{
			Provider: "vault",
			Parameters: map[string]string{
				"objects":      objectsString,
				"roleName":     role,
				"vaultAddress": vaultAddress,
			},
		},
	}
}

// Fetches a key-value secret (kv-v2) after authenticating to Vault with a Kubernetes service account.
// For a more in-depth setup explanation, please see the relevant readme in the hashicorp/vault-examples repo.
func getSecretWithKubernetesAuth(platform, namespace, domain, role string) ([]secretSpec, error) {
	// If set, the VAULT_ADDR environment variable will be the address that
	// your pod uses to communicate with Vault.
	config := vault.DefaultConfig() // modify for more granular configuration

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	// 	jwt := `eyJhbGciOiJSUzI1NiIsImtpZCI6IlJwUmsxTFhOVkRZdXNIVFBxazVHTmxOdFU3QWdhX3B3LVU5eXdXdmlRX2sifQ.eyJhdWQiOlsiaHR0cHM6Ly90ZXN0a3ViZS1kbnMtMWI2MjNsZG0uaGNwLnN3ZWRlbmNlbnRyYWwuYXptazhzLmlvIiwiXCJ0ZXN0a3ViZS1kbnMtMWI2MjNsZG0uaGNwLnN3ZWRlbmNlbnRyYWwuYXptazhzLmlvXCIiXSwiZXhwIjoxNzY0MzM3MzI5LCJpYXQiOjE3MzI4MDEzMjksIm
	// lzcyI6Imh0dHBzOi8vdGVzdGt1YmUtZG5zLTFiNjIzbGRtLmhjcC5zd2VkZW5jZW50cmFsLmF6bWs4cy5pbyIsImt1YmVybmV0ZXMuaW8iOnsibmFtZXNwYWNlIjoiZGVmYXVsdCIsInBvZCI6eyJuYW1lIjoidmF1bHQtY3NpLXByb3ZpZGVyLWpmZjZ3IiwidWlkIjoiMGQ5MmM3OWEtZTQ0OC00MzkzLWE2MDQtZTM3ZDc4YTA2ZDdjIn0sInNlcnZpY2VhY2NvdW50Ijp7Im5hbWUiOiJ2YXVsdC1jc2ktcHJ
	// vdmlkZXIiLCJ1aWQiOiI2MmJjNTY4OC0yOGYyLTQwOWItYmMzZS1jZTBhMzJkNWRiOTYifSwid2FybmFmdGVyIjoxNzMyODA0OTM2fSwibmJmIjoxNzMyODAxMzI5LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDp2YXVsdC1jc2ktcHJvdmlkZXIifQ.pxMwzN1ks7ADCD-sIIlVED-U3SiC6PL0yoEicLCrA8jEWdQIICdTBzM1YzK5pDtsbUMKTYoKuANcu5GljTFE5woomoChbL_juovHE
	// ZxHWB-MtI0mceL8D_6T8m_-eYZEpl0FGfmb2ajiAavFW4yKv_uaUrwSZrD3CXPfei8_qi5vsS0KuJaQBLHWRtioQitlrvMq3R4jzVAb1iswmpWgtMbrOkKne9xKFLBwQpNkBKqEZO5mbvypZPxzWdyBp0ICODZLkwFxLB39pJ4f85su0-FZwIbmhj219_JxPzVL52y-J7GLzPXtUsBGK_siwkhpnvj-QC1RFIHgm7c8GV_pSaS_sjSVWh-irqO7pmp8G3IFxFd4qcrBJ7Fr2jy7iNSaglLuz0mEr4cisXU_AFaHwW
	// GrGoqDIrzVKD4JiSK-0fGLrIgyKeOlFiQndPYEG9oMZ3M0F-XnsXLc8AvBjBYMz8m-fADRA1d_eNnbilwKGLly0J0jbHgT9kwP02TaUeHTcgURhj2kV2mEIvLMXnjo6BWNqdOyTnep8sg49WF9EJdlIAisqTyP0YUnYY_cJFdsD_0TB6lymxA0wpJdUUWX793K-LctKWhKn98-wrGG_WCAuMYPp_nUWnsk0ze-7jlj8Vza5jqrbfhtj22mlPSZWLChEziQYKWjiuIfwFlTdb0`

	// The service-account token will be read from the path where the token's
	// Kubernetes Secret is mounted. By default, Kubernetes will mount it to
	// /var/run/secrets/kubernetes.io/serviceaccount/token
	k8sAuth, err := auth.NewKubernetesAuth(role) //auth.NewKubernetesAuth(role, auth.WithServiceAccountToken(jwt))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	secretList := []secretSpec{}

	var listSecrets func(string, string) error
	listSecrets = func(secretEngine, parentPath string) (err error) {
		listedPathsSecret, err := client.Logical().List(fmt.Sprintf("%s/metadata/%s", secretEngine, parentPath))
		if err != nil {
			return fmt.Errorf("unable to list secrets: %w", err)
		}

		isLeaf := listedPathsSecret == nil
		if isLeaf {
			secretPath := fmt.Sprintf("%s/data/%s", secretEngine, parentPath)
			secret, err := client.Logical().Read(secretPath)
			if err != nil {
				return fmt.Errorf("unable to read secret: %w", err)
			}
			if secret == nil || secret.Data["data"] == nil {
				return nil
			}
			secretData, ok := secret.Data["data"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("data type assertion failed: %T %#v", secret.Data["data"], secret.Data["data"])
			}
			for secretKey := range secretData {
				secretList = append(secretList, secretSpec{Path: secretPath, Key: secretKey})
			}
			return nil
		}

		listedPaths, ok := listedPathsSecret.Data["keys"].([]interface{})
		if !ok {
			return fmt.Errorf("keys type assertion failed: %T %#v", listedPathsSecret.Data["keys"], listedPathsSecret.Data["keys"])
		}

		for _, path := range listedPaths {
			pathString, ok := path.(string)
			if !ok {
				return fmt.Errorf("key type assertion failed: %T %#v", path, path)
			}

			if err := listSecrets(secretEngine, parentPath+pathString); err != nil {
				return err
			}
		}
		return nil
	}

	path := fmt.Sprintf("%s/%s/", namespace, domain)
	err = listSecrets(platform, path)
	return secretList, err
}

func getOutputSpcName(domain string) string {
	return fmt.Sprintf("%s-aggregate", domain)
}

func getSPCPlatformAndDomain(spc *csiv1.SecretProviderClass) (platform string, domain string, ok bool) {
	domain, domainLabelExists := spc.Labels[controllers.DomainLabelName]
	if !domainLabelExists || len(domain) == 0 {
		return "", domain, false
	}

	platform, platformLabelExists := spc.Labels[controllers.PlatformLabelName]
	if !platformLabelExists || len(platform) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}

func isOutputSPC(spc *csiv1.SecretProviderClass) bool {
	owner := metav1.GetControllerOf(spc)
	return (owner != nil &&
		owner.Kind == "ConfigurationDomain" &&
		owner.APIVersion == "configuration.totalsoft.ro/v1alpha1")
}
