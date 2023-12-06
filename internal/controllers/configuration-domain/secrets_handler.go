package configuration

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	csiClientset "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	controllers "totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"

	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	csiListers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/listers/apis/v1"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
)

var getSecrets = getSecretWithKubernetesAuth

type secretSpec struct {
	Path string `json:"platformRef"`
	Key  string `json:"domain"`
}
type secretsHandler struct {
	csiClientset csiClientset.Interface
	spcLister    csiListers.SecretProviderClassLister

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func newSecretsHandler(
	csiClientset csiClientset.Interface,
	spcLister csiListers.SecretProviderClassLister,
	recorder record.EventRecorder,
) *secretsHandler {
	handler := &secretsHandler{
		csiClientset: csiClientset,
		spcLister:    spcLister,
		recorder:     recorder,
	}
	return handler
}

func (c *secretsHandler) Cleanup(namespace, domain string) error {
	outputSpcName := getOutputSpcName(domain)
	err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Delete(context.TODO(), outputSpcName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *secretsHandler) Sync(platformObj *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain) error {
	outputSpcName := getOutputSpcName(configDomain.Name)
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

	aggregatedSpc := c.aggregateSecrets(configDomain, secrets, outputSpcName, role)

	// Get the output SPC
	outputSpc, err := c.spcLister.SecretProviderClasses(configDomain.Namespace).Get(outputSpcName)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		outputSpc, err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(configDomain.Namespace).Create(context.TODO(), aggregatedSpc, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	// If the SPC is not controlled by this SecretsAggregate resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputSpc, configDomain) {
		msg := fmt.Sprintf(MessageResourceExists, outputSpc.Name)
		c.recorder.Event(configDomain, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil
	}

	// If the existing SPC data differs from the aggregation result we
	// should update the SPC resource.
	if !reflect.DeepEqual(aggregatedSpc.Spec.Parameters, outputSpc.Spec.Parameters) {
		klog.V(4).Infof("Secret values changed")
		outputSpc := outputSpc.DeepCopy()
		outputSpc.Spec.Parameters = aggregatedSpc.Spec.Parameters
		_, err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(configDomain.Namespace).Update(context.TODO(), outputSpc, metav1.UpdateOptions{})
		if err != nil {
			c.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
			return err
		}
	}

	return nil
}

func (c *secretsHandler) aggregateSecrets(configurationDomain *v1alpha1.ConfigurationDomain, secrets []secretSpec, outputName, role string) *csiv1.SecretProviderClass {
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
		ObjectMeta: metav1.ObjectMeta{
			Name: outputName,
			Labels: map[string]string{
				controllers.DomainLabelName:   configurationDomain.Name,
				controllers.PlatformLabelName: configurationDomain.Spec.PlatformRef,
			},
			Namespace: configurationDomain.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(configurationDomain, v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomain")),
			},
		},
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

	// 	jwt := `
	// eyJhbGciOiJSUzI1NiIsImtpZCI6IjI3OVo0M1NNamlZZjRnMkR6SWNiZG5tZEhiNkxhbTZXNlN0NkExb1JQeFkifQ.eyJhdWQiOlsiaHR0cHM6Ly9jaGFyaXNtYS1vbmxpbmUtYWtzLWN
	// sdXN0ZXItMDg0ZDM5NDguaGNwLndlc3RldXJvcGUuYXptazhzLmlvIl0sImV4cCI6MTY4ODAzNDQwOCwiaWF0IjoxNjU2NDk4NDA4LCJpc3MiOiJodHRwczovL2NoYXJpc21hLW9ubGluZ
	// S1ha3MtY2x1c3Rlci0wODRkMzk0OC5oY3Aud2VzdGV1cm9wZS5hem1rOHMuaW8iLCJrdWJlcm5ldGVzLmlvIjp7Im5hbWVzcGFjZSI6InFhLWxzbmciLCJwb2QiOnsibmFtZSI6Im9yaWd
	// pbmF0aW9uLWh1YnMtNzhmZmY5N2Q0NC1uMmQ3dyIsInVpZCI6IjEyZTk2ZTkzLTc2MTYtNDc3Zi1iNTBmLWZhOGFiNzg5ZTZiMSJ9LCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoiZGVmY
	// XVsdCIsInVpZCI6IjI4ZjViYmUyLTU3ZjgtNDZlZS1iMDE1LTM5ZGVkNzI4ODRlNyJ9LCJ3YXJuYWZ0ZXIiOjE2NTY1MDIwMTV9LCJuYmYiOjE2NTY0OTg0MDgsInN1YiI6InN5c3RlbTp
	// zZXJ2aWNlYWNjb3VudDpxYS1sc25nOmRlZmF1bHQifQ.hwJxwMsQ5dTxBNqCC7g39I1azPH-IbynQEC5j6mSB9O7Zkq2t2dctW8F_piN2XTK56Q2HaVnT2oYNttYKpYaFPOYDlfuTSGiLB
	// rgHcphM3Em_JNiPxZqUw4RReMojm8clSUa7vCau4qPKrcPHGW4JMFdrhHpIJQeaNmBGqGjzgxrp3UPNshumI2AEqEo_nQUVNzJm5nqoQLf5oPkdPv6IyiBDihG_ZvKFiLW_Hp2BzcbTBgC
	// yuIlTVs3ZU28z-Y7pG69lTpwDlLIxtk3vOWy6h2OcKtQEPcHEPgF6kXeCvianghXWMj_UkeXmtxtnyveT65nwaFyP4nN_Awz9rcs0Sy8TqfTJLGN0nMGblwJN1IbF_b9pnvh07uFQxUHSd
	// f1rIXi8YQhLsaneb-fmaQ2FM1YVQjTtTqY0WkHYbL66j7HrPEB1r-d0Ei1Sc0yohEBPK2HZF7GyxOlHMf53a5YDqATsxW__H1_5QK6PTYkkO1RfTU1pR_C7Qr71jNCg5AWyVmHVxgMRtw5
	// MBojlafyvPRGiUH7MJ_US1zAy5TAFebkDnut00z1mmmLLZ_k8BlUzI58widqiHgp37tlLOWjLNKMVfNp8lWmhefrTg0LXzPizRLZiG3vaveU6DnZctRzyNYjbdBrDX-xhcMRov9JuIXKkL
	// 6WADW1MrWSDBDnAI4`

	// The service-account token will be read from the path where the token's
	// Kubernetes Secret is mounted. By default, Kubernetes will mount it to
	// /var/run/secrets/kubernetes.io/serviceaccount/token
	k8sAuth, err := auth.NewKubernetesAuth(role) //, auth.WithServiceAccountToken(jwt))
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
