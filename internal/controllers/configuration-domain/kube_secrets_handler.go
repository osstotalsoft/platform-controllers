package configuration

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	controllers "totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

// var ErrNonRetryAble = errors.New("non retry-able handled error")

type kubeSecretsHandler struct {
	kubeClientset kubernetes.Interface

	secretsLister coreListers.SecretLister

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func newKubeSecretsHandler(
	kubeClientset kubernetes.Interface,
	secretsLister coreListers.SecretLister,
	recorder record.EventRecorder,
) *kubeSecretsHandler {
	handler := &kubeSecretsHandler{
		kubeClientset: kubeClientset,
		secretsLister: secretsLister,
		recorder:      recorder,
	}
	return handler
}

func (c *kubeSecretsHandler) Cleanup(namespace, domain string) error {
	outputSecretName := getOutputSecretName(domain)
	err := c.kubeClientset.CoreV1().Secrets(namespace).Delete(context.TODO(), outputSecretName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (h *kubeSecretsHandler) Sync(platformObj *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain) error {
	outputSecretName := getOutputSecretName(configDomain.Name)

	secrets, err := h.getSecretsFor(platformObj, configDomain.Namespace, configDomain.Name)
	if err != nil {
		if errors.Is(err, ErrNonRetryAble) {
			return nil
		}
		return err
	}

	aggregatedSecret := h.aggregateSecrets(configDomain, secrets, outputSecretName)

	// Get the output secret for this namespace::domain
	outputSecret, err := h.secretsLister.Secrets(configDomain.Namespace).Get(outputSecretName)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		outputSecret, err = h.kubeClientset.CoreV1().Secrets(configDomain.Namespace).Create(context.TODO(), aggregatedSecret, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		h.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	// If the Secret is not controlled by this SecretAggregate resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputSecret, configDomain) {
		msg := fmt.Sprintf(MessageResourceExists, outputSecret.Name)
		h.recorder.Event(configDomain, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil
	}

	// If the existing Secret data differs from the aggregation result we
	// should update the Secret resource.
	if !reflect.DeepEqual(aggregatedSecret.Data, outputSecret.Data) {
		klog.V(4).Infof("Secret values changed")
		outputSecret = outputSecret.DeepCopy()
		outputSecret.Data = aggregatedSecret.Data
		_, err = h.kubeClientset.CoreV1().Secrets(configDomain.Namespace).Update(context.TODO(), outputSecret, metav1.UpdateOptions{})
		if err != nil {
			h.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
			return err
		}
	}

	return nil
}

func (h *kubeSecretsHandler) getSecretsFor(platform *platformv1.Platform, namespace, domain string) ([]*corev1.Secret, error) {
	domainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			controllers.DomainLabelName:   domain,
			controllers.PlatformLabelName: platform.Name,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil, ErrNonRetryAble
	}

	domainNsAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			controllers.DomainLabelName:   domain + "." + namespace,
			controllers.PlatformLabelName: platform.Name,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil, ErrNonRetryAble
	}

	globalDomainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			controllers.DomainLabelName:   controllers.GlobalDomainLabelValue,
			controllers.PlatformLabelName: platform.Name,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil, ErrNonRetryAble
	}

	platformSecrets, err := h.secretsLister.Secrets(platform.Spec.TargetNamespace).List(globalDomainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	globalDomainSecrets, err := h.secretsLister.Secrets(namespace).List(globalDomainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	currentNsSecrets, err := h.secretsLister.Secrets(namespace).List(domainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	nsSecrets, err := h.secretsLister.List(domainNsAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	currentNsSecrets = append(append(append(platformSecrets, globalDomainSecrets...), currentNsSecrets...), nsSecrets...)
	return currentNsSecrets, nil
}

func (h *kubeSecretsHandler) aggregateSecrets(configurationDomain *v1alpha1.ConfigurationDomain, secrets []*corev1.Secret, outputName string) *corev1.Secret {
	mergedData := map[string][]byte{}
	for _, secret := range secrets {
		if secret.Name == outputName {
			continue
		}

		for k, v := range secret.Data {
			if existingValue, ok := mergedData[k]; ok {
				klog.V(4).Infof("Key %s already exists with value %s. It will be replaced in secret %s with value %s", k, existingValue, secret.Name, v)
			}
			mergedData[k] = v
		}
	}

	return &corev1.Secret{
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
		Data: mergedData,
	}
}

func getOutputSecretName(domain string) string {
	return fmt.Sprintf("%s-aggregate", domain)
}

func isOutputSecret(secret *corev1.Secret) bool {
	owner := metav1.GetControllerOf(secret)
	return (owner != nil &&
		owner.Kind == "ConfigurationDomain" &&
		owner.APIVersion == "configuration.totalsoft.ro/v1alpha1")
}
