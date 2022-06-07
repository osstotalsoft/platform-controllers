package controllers

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubeFakeClientSet "k8s.io/client-go/kubernetes/fake"
	configurationv1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
)

func TestConfigAggregateController_processNextWorkItem(t *testing.T) {

	t.Run("aggregate two config maps", func(t *testing.T) {
		// Arrange
		configMaps := []runtime.Object{
			newConfigMap("configMap1", "domain1", "dev", map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", "domain1", "dev", map[string]string{"k2": "v2"}),
		}
		configAggregates := []runtime.Object{
			newConfigAggregate("configAggregate1", "domain1", "dev"),
		}
		c := runController(configAggregates, configMaps)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
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

		output, err := c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
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
		configMaps := []runtime.Object{
			newConfigMap("configMap1", "domain1", "dev", map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", globalDomainLabelValue, "dev", map[string]string{"k2": "v2"}),
		}
		configAggregates := []runtime.Object{
			newConfigAggregate("configAggregate1", "domain1", "dev"),
		}
		c := runController(configAggregates, configMaps)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
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

		output, err := c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	// t.Run("aggregate config maps with overlapping keys", func(t *testing.T) {
	// 	// Arrange
	// 	configMaps := []runtime.Object{
	// 		newConfigMap("configMap1", "domain1", "dev", map[string]string{"k1": "v1", "k2": "toOverwrite"}),
	// 		newConfigMap("configMap2", "domain1", "dev", map[string]string{"k2": "v2"}),
	// 	}
	// 	configAggregates := []runtime.Object{
	// 		newConfigAggregate("configAggregate1", "domain1", "dev"),
	// 	}
	// 	c := runController(configAggregates, configMaps)
	// 	if c.workqueue.Len() != 1 {
	// 		t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
	// 	}

	// 	// Act
	// 	if result := c.processNextWorkItem(); !result {
	// 		t.Error("processing failed")
	// 	}

	// 	// Assert
	// 	if c.workqueue.Len() != 0 {
	// 		item, _ := c.workqueue.Get()
	// 		t.Error("queue should be empty, but contains ", item)
	// 	}

	// 	output, err := c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
	// 	if err != nil {
	// 		t.Error(err)
	// 		return
	// 	}
	// 	expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
	// 	if !reflect.DeepEqual(output.Data, expectedOutput) {
	// 		t.Error("expected output config ", expectedOutput, ", got", output.Data)
	// 	}
	// })

	t.Run("multiple configAggregates for the same platform and domain should throw error", func(t *testing.T) {
		// Arrange
		configMaps := []runtime.Object{
			newConfigMap("configMap1", "domain1", "dev", map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", "domain1", "dev", map[string]string{"k2": "v2"}),
		}
		configAggregates := []runtime.Object{
			newConfigAggregate("configAggregate1", "domain1", "dev"),
			newConfigAggregate("configAggregate2", "domain1", "dev"),
		}
		c := runController(configAggregates, configMaps)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
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

		foundConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
		if foundConfigMap != nil || err == nil {
			t.Error("output config map should not be generated ")
		}
	})

	t.Run("invalidating the configurationAggregate should remove output", func(t *testing.T) {
		// Arrange
		configMaps := []runtime.Object{
			newConfigMap("configMap1", "domain1", "dev", map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", "domain1", "dev", map[string]string{"k2": "v2"}),
		}
		configAggregates := []runtime.Object{
			newConfigAggregate("configAggregate1", "domain1", "dev"),
		}
		c := runController(configAggregates, configMaps)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		foundConfigAggregate, err := c.configurationClientset.ConfigurationV1alpha1().ConfigurationAggregates(metav1.NamespaceDefault).Get(context.TODO(), "configAggregate1", metav1.GetOptions{})
		if err != nil {
			t.Error("configurationAggregate not found")
		}
		foundConfigAggregate = foundConfigAggregate.DeepCopy()
		foundConfigAggregate.Spec.Domain = "domain2"
		c.configurationClientset.ConfigurationV1alpha1().ConfigurationAggregates(metav1.NamespaceDefault).Update(context.TODO(), foundConfigAggregate, metav1.UpdateOptions{})

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
			//item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", c.workqueue.Len())
		}

		foundConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
		if foundConfigMap != nil || err == nil {
			t.Error("output config map dev-domain1-aggregate should be deleted ")
		}
		foundConfigMap, err = c.kubeClientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain2-aggregate", metav1.GetOptions{})
		if foundConfigMap == nil || err != nil {
			t.Error("output config map dev-domain2-aggregate should be present ")
		}
	})
}

func newConfigAggregate(name, domain, platform string) *configurationv1.ConfigurationAggregate {
	return &configurationv1.ConfigurationAggregate{
		TypeMeta: metav1.TypeMeta{APIVersion: configurationv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: configurationv1.ConfigurationAggregateSpec{
			PlatformRef: platform,
			Domain:      domain,
		},
	}
}

func newConfigMap(name, domain, platform string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				domainLabelName:   domain,
				platformLabelName: platform,
			},
		},
		Data: data,
	}
}

func runController(configAggregates, configMaps []runtime.Object) *ConfigurationController {
	kubeClient := kubeFakeClientSet.NewSimpleClientset(configMaps...)
	platformClient := fakeClientset.NewSimpleClientset(configAggregates...)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	c := NewConfigurationController(kubeClient, platformClient, kubeInformerFactory.Core().V1().ConfigMaps(),
		platformInformerFactory.Configuration().V1alpha1().ConfigurationAggregates(), nil)
	kubeInformerFactory.Start(nil)
	platformInformerFactory.Start(nil)

	kubeInformerFactory.WaitForCacheSync(nil)
	platformInformerFactory.WaitForCacheSync(nil)

	return c
}
