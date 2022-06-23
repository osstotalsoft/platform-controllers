package configuration

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubeFakeClientSet "k8s.io/client-go/kubernetes/fake"
	configurationv1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
)

func TestConfigurationDomainController_processNextWorkItem(t *testing.T) {

	t.Run("aggregate two config maps", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "dev", "team1", "domain1"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace, platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", domain, namespace, platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		c := runController(configurationDomains, configMaps, platforms)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), fmt.Sprintf("%s-%s-aggregate", platform, domain), metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	t.Run("aggregate domain specific and global config map", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "pl1", "n1", "domain1"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace, platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", globalDomainLabelValue, namespace, platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		c := runController(configurationDomains, configMaps, platforms)
		if c.workqueue.Len() != 1 {
			items := c.workqueue.Len()
			t.Error("queue should have only 1 item, but it has", items)
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), fmt.Sprintf("%s-%s-aggregate", platform, domain), metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	t.Run("aggregate platform config map", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "p1", "p1-team1", "domain1"
		platformNamespace := namespace

		configMaps := []runtime.Object{
			newConfigMap("configMap1", globalDomainLabelValue, platformNamespace, platform, map[string]string{"k1": "v1"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		c := runController(configurationDomains, configMaps, platforms)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), fmt.Sprintf("%s-%s-aggregate", platform, domain), metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	t.Run("should remove output when platform changes", func(t *testing.T) {
		// Arrange
		old_platform, new_platform, namespace, domain := "p3", "p4", "p3-ns1", "domain1"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace, old_platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", domain, namespace, old_platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, old_platform),
		}
		platforms := []runtime.Object{
			newPlatform(old_platform, old_platform),
			newPlatform(new_platform, new_platform),
		}
		c := runController(configurationDomains, configMaps, platforms)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		foundConfigurationDomain, err := c.configurationClientset.ConfigurationV1alpha1().ConfigurationDomains(namespace).Get(context.TODO(), domain, metav1.GetOptions{})
		if err != nil {
			t.Error("configurationDomain not found")
		}
		foundConfigurationDomain = foundConfigurationDomain.DeepCopy()
		foundConfigurationDomain.Spec.PlatformRef = new_platform
		c.configurationClientset.ConfigurationV1alpha1().ConfigurationDomains(namespace).Update(context.TODO(), foundConfigurationDomain, metav1.UpdateOptions{})

		time.Sleep(100 * time.Millisecond)

		if c.workqueue.Len() != 2 {
			item, _ := c.workqueue.Get()
			t.Error("queue should have 2 items, but contains ", item)
		}

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		oldAggregateConfigMap := fmt.Sprintf("%s-%s-aggregate", old_platform, domain)
		foundConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), oldAggregateConfigMap, metav1.GetOptions{})
		if foundConfigMap != nil || err == nil {
			t.Errorf("output config map %s should be deleted ", oldAggregateConfigMap)
		}
		newAggregateConfigMap := fmt.Sprintf("%s-%s-aggregate", new_platform, domain)
		foundConfigMap, err = c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), newAggregateConfigMap, metav1.GetOptions{})
		if foundConfigMap == nil || err != nil {
			t.Errorf("output config map %s should be present ", newAggregateConfigMap)
		}
	})

}

func newConfigurationDomain(name, namespace, platform string) *configurationv1.ConfigurationDomain {
	return &configurationv1.ConfigurationDomain{
		TypeMeta: metav1.TypeMeta{APIVersion: configurationv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: configurationv1.ConfigurationDomainSpec{
			PlatformRef:         platform,
			AggregateConfigMaps: true,
			AggregateSecrets:    false,
		},
	}
}

func newPlatform(name, targetNamespace string) *platformv1.Platform {
	return &platformv1.Platform{
		TypeMeta: metav1.TypeMeta{APIVersion: configurationv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: platformv1.PlatformSpec{
			TargetNamespace: targetNamespace,
		},
	}
}

func newConfigMap(name, domain, namespace, platform string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				domainLabelName:   domain,
				platformLabelName: platform,
			},
		},
		Data: data,
	}
}

func runController(configurationDomains, configMaps []runtime.Object, platforms []runtime.Object) *ConfigurationDomainController {
	kubeClient := kubeFakeClientSet.NewSimpleClientset(configMaps...)
	platformClient := fakeClientset.NewSimpleClientset(append(platforms, configurationDomains...)...)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	c := NewConfigurationDomainController(kubeClient, platformClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		platformInformerFactory.Configuration().V1alpha1().ConfigurationDomains(),
		platformInformerFactory.Platform().V1alpha1().Platforms(),
		nil)
	kubeInformerFactory.Start(nil)
	platformInformerFactory.Start(nil)

	platformInformerFactory.WaitForCacheSync(nil)
	kubeInformerFactory.WaitForCacheSync(nil)

	return c
}
