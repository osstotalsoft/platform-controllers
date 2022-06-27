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

type configurationHandler struct {
	kubeClientset kubernetes.Interface

	configMapsLister coreListers.ConfigMapLister

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func newConfigurationHandler(
	kubeClientset kubernetes.Interface,
	configMapsLister coreListers.ConfigMapLister,
	recorder record.EventRecorder,
) *configurationHandler {
	handler := &configurationHandler{
		kubeClientset:    kubeClientset,
		configMapsLister: configMapsLister,
		recorder:         recorder,
	}
	return handler
}

func (c *configurationHandler) Cleanup(platform, namespace, domain string) error {
	outputConfigMapName := getOutputConfigmapName(platform, domain)
	err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), outputConfigMapName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *configurationHandler) Sync(platformObj *platformv1.Platform, configDomain *v1alpha1.ConfigurationDomain) error {
	outputConfigMapName := getOutputConfigmapName(platformObj.Name, configDomain.Name)

	configMaps, err := c.getConfigMapsFor(platformObj, configDomain.Namespace, configDomain.Name)
	if err != nil {
		if errors.Is(err, ErrNonRetryAble) {
			return nil
		}
		return err
	}

	aggregatedConfigMap := c.aggregateConfigMaps(configDomain, configMaps, outputConfigMapName)

	// Get the output config map for this namespace::domain
	outputConfigMap, err := c.configMapsLister.ConfigMaps(configDomain.Namespace).Get(outputConfigMapName)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		outputConfigMap, err = c.kubeClientset.CoreV1().ConfigMaps(configDomain.Namespace).Create(context.TODO(), aggregatedConfigMap, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	// If the ConfigMap is not controlled by this ConfigMapAggregate resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputConfigMap, configDomain) {
		msg := fmt.Sprintf(MessageResourceExists, outputConfigMap.Name)
		c.recorder.Event(configDomain, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil
	}

	// If the existing ConfigMap data differs from the aggregation result we
	// should update the ConfigMap resource.
	if !reflect.DeepEqual(aggregatedConfigMap.Data, outputConfigMap.Data) {
		klog.V(4).Infof("Configuration values changed")
		outputConfigMap = outputConfigMap.DeepCopy()
		outputConfigMap.Data = aggregatedConfigMap.Data
		_, err = c.kubeClientset.CoreV1().ConfigMaps(configDomain.Namespace).Update(context.TODO(), outputConfigMap, metav1.UpdateOptions{})
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	return nil
}

func (c *configurationHandler) getConfigMapsFor(platform *platformv1.Platform, namespace, domain string) ([]*corev1.ConfigMap, error) {
	domainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			domainLabelName:   domain,
			platformLabelName: platform.Name,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil, ErrNonRetryAble
	}

	globalDomainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			domainLabelName:   globalDomainLabelValue,
			platformLabelName: platform.Name,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil, ErrNonRetryAble
	}

	platformConfigMaps, err := c.configMapsLister.ConfigMaps(platform.Spec.TargetNamespace).List(globalDomainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	globalDomainConfigMaps, err := c.configMapsLister.ConfigMaps(namespace).List(globalDomainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	configMaps, err := c.configMapsLister.ConfigMaps(namespace).List(domainAndPlatformLabelSelector)
	if err != nil {
		return nil, err
	}

	configMaps = append(append(platformConfigMaps, globalDomainConfigMaps...), configMaps...)
	return configMaps, nil
}

func (c *configurationHandler) aggregateConfigMaps(configurationDomain *v1alpha1.ConfigurationDomain, configMaps []*corev1.ConfigMap, outputName string) *corev1.ConfigMap {
	mergedData := map[string]string{}
	for _, configMap := range configMaps {
		if configMap.Name == outputName {
			continue
		}

		for k, v := range configMap.Data {
			if existingValue, ok := mergedData[k]; ok {
				msg := fmt.Sprintf("Key %s already exists with value %s. It will be replaced by config map %s with value %s", k, existingValue, configMap.Name, v)
				c.recorder.Event(configurationDomain, corev1.EventTypeWarning, ErrResourceExists, msg)
			}
			mergedData[k] = v
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: outputName,
			Labels: map[string]string{
				domainLabelName:   configurationDomain.Name,
				platformLabelName: configurationDomain.Spec.PlatformRef,
			},
			Namespace: configurationDomain.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(configurationDomain, v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomain")),
			},
		},
		Data: mergedData,
	}
}

func getOutputConfigmapName(platform, domain string) string {
	return fmt.Sprintf("%s-%s-aggregate", platform, domain)
}

func isOutputConfigMap(configMap *corev1.ConfigMap) bool {
	owner := metav1.GetControllerOf(configMap)
	return (owner != nil &&
		owner.Kind == "ConfigurationDomain" &&
		owner.APIVersion == "configuration.totalsoft.ro/v1alpha1")
}
