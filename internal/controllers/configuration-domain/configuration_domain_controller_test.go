package configuration

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

	fakeCsiClientSet "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	csiinformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions"

	configurationv1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"

	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"

	"totalsoft.ro/platform-controllers/internal/controllers"
	messaging "totalsoft.ro/platform-controllers/internal/messaging/mock"
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
			newConfigurationDomain(domain, namespace, platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
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
		outputConfigmap := getOutputConfigmapName(domain)
		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate two kube secrets", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "dev", "team1", "domain1"
		secrets := []runtime.Object{
			newSecret("secret1", domain, namespace, platform, map[string][]byte{"k1": []byte("v1")}),
			newSecret("secret2", domain, namespace, platform, map[string][]byte{"k2": []byte("v2")}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		configMaps := []runtime.Object{}
		spcs := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, secrets, spcs)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{}, nil
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
		outputSecret := getOutputSecretName(domain)
		output, err := c.kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), outputSecret, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output secret ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate domain specific and global config map", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "pl1", "n1", "domain1"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace, platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", controllers.GlobalDomainLabelValue, namespace, platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
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

		outputConfigmap := getOutputConfigmapName(domain)
		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k2": "v2"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message published to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate domain specific and global kube secret", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "pl1", "n1", "domain1"
		kubeSecrets := []runtime.Object{
			newSecret("secret1", domain, namespace, platform, map[string][]byte{"k1": []byte("v1")}),
			newSecret("secret2", controllers.GlobalDomainLabelValue, namespace, platform, map[string][]byte{"k2": []byte("v2")}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		configMaps := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 1 {
			items := c.workqueue.Len()
			t.Error("queue should have only 1 item, but it has", items)
			return
		}

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{}, nil
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

		outputSecret := getOutputSecretName(domain)
		output, err := c.kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), outputSecret, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output secret ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message published to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate domain specific config maps from a different namespace", func(t *testing.T) {
		// Arrange
		platform, namespace1, namespace2, domain := "pl1", "n1", "n2", "domain"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace1, platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", domain+"."+namespace2, namespace1, platform, map[string]string{"k2": "v2"}),
			newConfigMap("configMap3", controllers.GlobalDomainLabelValue, namespace1, platform, map[string]string{"k3": "v3"}),
			newConfigMap("configMap4", controllers.GlobalDomainLabelValue, namespace2, platform, map[string]string{"k4": "v4"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace1, platform, true, false),
			newConfigurationDomain(domain, namespace2, platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 2 {
			items := c.workqueue.Len()
			t.Error("queue should have only 2 items, but it has", items)
			return
		}

		// Act
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

		outputConfigmap := getOutputConfigmapName(domain)
		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace1).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1", "k3": "v3"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		output, err = c.kubeClientset.CoreV1().ConfigMaps(namespace2).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput = map[string]string{"k2": "v2", "k4": "v4"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate domain specific kube secrets from a different namespace", func(t *testing.T) {
		// Arrange
		platform, namespace1, namespace2, domain := "pl1", "n1", "n2", "domain"
		kubeSecrets := []runtime.Object{
			newSecret("secret1", domain, namespace1, platform, map[string][]byte{"k1": []byte("v1")}),
			newSecret("secret2", domain+"."+namespace2, namespace1, platform, map[string][]byte{"k2": []byte("v2")}),
			newSecret("secret3", controllers.GlobalDomainLabelValue, namespace1, platform, map[string][]byte{"k3": []byte("v3")}),
			newSecret("secret4", controllers.GlobalDomainLabelValue, namespace2, platform, map[string][]byte{"k4": []byte("v4")}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace1, platform, false, true),
			newConfigurationDomain(domain, namespace2, platform, false, true),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		configMaps := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 2 {
			items := c.workqueue.Len()
			t.Error("queue should have only 2 items, but it has", items)
			return
		}

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{}, nil
		}

		// Act
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

		outputSecret := getOutputSecretName(domain)
		output, err := c.kubeClientset.CoreV1().Secrets(namespace1).Get(context.TODO(), outputSecret, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string][]byte{"k1": []byte("v1"), "k3": []byte("v3")}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		output, err = c.kubeClientset.CoreV1().Secrets(namespace2).Get(context.TODO(), outputSecret, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput = map[string][]byte{"k2": []byte("v2"), "k4": []byte("v4")}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate platform config map", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "p1", "p1-team1", "domain1"
		platformNamespace := namespace

		configMaps := []runtime.Object{
			newConfigMap("configMap1", controllers.GlobalDomainLabelValue, platformNamespace, platform, map[string]string{"k1": "v1"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
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

		outputConfigmap := getOutputConfigmapName(domain)
		output, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{"k1": "v1"}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate platform kube secrets", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "p1", "p1-team1", "domain1"
		platformNamespace := namespace

		kubeSecrets := []runtime.Object{
			newSecret("secret1", controllers.GlobalDomainLabelValue, platformNamespace, platform, map[string][]byte{"k1": []byte("v1")}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		configMaps := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{}, nil
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

		outputSecret := getOutputSecretName(domain)
		output, err := c.kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), outputSecret, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string][]byte{"k1": []byte("v1")}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output secret ", expectedOutput, ", got", output.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("should update output when platform changes", func(t *testing.T) {
		// Arrange
		old_platform, new_platform, namespace, domain := "p1", "p2", "p1-ns1", "domain1"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", controllers.GlobalDomainLabelValue, old_platform, old_platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", controllers.GlobalDomainLabelValue, new_platform, new_platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, old_platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(old_platform, old_platform),
			newPlatform(new_platform, new_platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		foundConfigurationDomain, err := c.platformClientset.ConfigurationV1alpha1().ConfigurationDomains(namespace).Get(context.TODO(), domain, metav1.GetOptions{})
		if err != nil {
			t.Error("configurationDomain not found")
		}
		foundConfigurationDomain = foundConfigurationDomain.DeepCopy()
		foundConfigurationDomain.Spec.PlatformRef = new_platform
		c.platformClientset.ConfigurationV1alpha1().ConfigurationDomains(namespace).Update(context.TODO(), foundConfigurationDomain, metav1.UpdateOptions{})

		time.Sleep(100 * time.Millisecond)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have 1 item, but contains ", c.workqueue.Len())
		}

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		aggregateConfigMap := getOutputConfigmapName(domain)
		foundConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), aggregateConfigMap, metav1.GetOptions{})
		if foundConfigMap == nil || err != nil {
			t.Errorf("output config map %s should be present ", aggregateConfigMap)
		}
		expectedOutput := map[string]string{"k2": "v2"}
		if !reflect.DeepEqual(foundConfigMap.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", foundConfigMap.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("should perform cleanup when aggregateConfigMaps is false", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "qa", "qa-t1", "domain1"
		outputConfigMap := getOutputConfigmapName(domain)
		configMaps := []runtime.Object{
			newConfigMap(outputConfigMap, domain, namespace, platform, map[string]string{"k1": "v1"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, _ := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
			return
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		time.Sleep(100 * time.Millisecond)

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		foundConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), outputConfigMap, metav1.GetOptions{})
		if foundConfigMap != nil || err == nil {
			t.Errorf("output config map %s should be deleted ", outputConfigMap)
		}
	})

	t.Run("should not aggregate configMaps from other platform namespaces", func(t *testing.T) {
		// Arrange
		platform, namespace1, namespace2, domain := "qa", "qa-n1", "qa-n2", "domain"
		configMaps := []runtime.Object{
			newConfigMap("configMap1", domain, namespace1, platform, map[string]string{"k1": "v1"}),
			newConfigMap("configMap2", controllers.GlobalDomainLabelValue, namespace1, platform, map[string]string{"k2": "v2"}),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace1, platform, true, false),
			newConfigurationDomain(domain, namespace2, platform, true, false),
		}
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)
		if c.workqueue.Len() != 2 {
			items := c.workqueue.Len()
			t.Error("queue should have 2 items, but it has", items)
			return
		}

		// Act
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

		outputConfigmap := getOutputConfigmapName(domain)
		output2, err := c.kubeClientset.CoreV1().ConfigMaps(namespace2).Get(context.TODO(), outputConfigmap, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{}
		if !reflect.DeepEqual(output2.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output2.Data)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("aggregate two vault secrets", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "qa", "qa-t1", "domain1"
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		configMaps := []runtime.Object{}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{
				{Key: "key1", Path: "path1"},
				{Key: "key2", Path: "path2"},
			}, nil
		}

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
		outputSpcName := getOutputSpcName(domain)
		output, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), outputSpcName, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := `
- objectName: "key1"
  secretPath: "path1"
  secretKey: "key1"
- objectName: "key2"
  secretPath: "path2"
  secretKey: "key2"`
		if output.Spec.Parameters["objects"] != expectedOutput {
			t.Error("expected output secret objects", expectedOutput, ", got", output.Spec.Parameters["objects"])
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("should re-generate SPC on delete", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "qa", "qa-t1", "domain1"
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		configMaps := []runtime.Object{}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{
				{Key: "key1", Path: "path1"},
				{Key: "key2", Path: "path2"},
			}, nil
		}

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		outputSpcName := getOutputSpcName(domain)
		_, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), outputSpcName, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		// Act
		err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Delete(context.TODO(), outputSpcName, metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(10 * time.Millisecond)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have 1 item, but it has", c.workqueue.Len())
			return
		}

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
			return
		}

		output, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), outputSpcName, metav1.GetOptions{})
		if output == nil || err != nil {
			t.Errorf("output SPC %s should be re-generated ", outputSpcName)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("should re-queue key when vault secrets synced", func(t *testing.T) {
		// Arrange
		platform, namespace, domain := "qa", "qa-t1", "domain1"
		platforms := []runtime.Object{
			newPlatform(platform, platform),
		}
		configurationDomains := []runtime.Object{
			newConfigurationDomain(domain, namespace, platform, false, true),
		}
		configMaps := []runtime.Object{}
		spcs := []runtime.Object{}
		kubeSecrets := []runtime.Object{}
		c, msgChan := runController(platforms, configurationDomains, configMaps, kubeSecrets, spcs)

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
			return []secretSpec{
				{Key: "key1", Path: "path1"},
				{Key: "key2", Path: "path2"},
			}, nil
		}

		requeueInterval = 1 * time.Millisecond

		// Act
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		time.Sleep(30 * time.Millisecond)

		// Assert
		if c.workqueue.Len() != 1 {
			t.Error("queue should have 1 item, but it has", c.workqueue.Len())
			return
		}

		obj, _ := c.workqueue.Get()
		actualKey, _ := obj.(string)
		expectedKey := encodeDomainKey(namespace, domain)

		if actualKey != expectedKey {
			t.Error("expected key", expectedKey, ", got", actualKey)
		}

		msg := <-msgChan
		if msg.Topic != syncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", syncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

}

func newConfigurationDomain(name, namespace, platform string, aggregateConfigMaps, aggregateSecrets bool) *configurationv1.ConfigurationDomain {
	return &configurationv1.ConfigurationDomain{
		TypeMeta: metav1.TypeMeta{APIVersion: configurationv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: configurationv1.ConfigurationDomainSpec{
			PlatformRef:         platform,
			AggregateConfigMaps: aggregateConfigMaps,
			AggregateSecrets:    aggregateSecrets,
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
				controllers.DomainLabelName:   domain,
				controllers.PlatformLabelName: platform,
			},
		},
		Data: data,
	}
}

func newSecret(name, domain, namespace, platform string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				controllers.DomainLabelName:   domain,
				controllers.PlatformLabelName: platform,
			},
		},
		Data: data,
	}
}

func runController(platforms []runtime.Object, configurationDomains, configMaps []runtime.Object, kubeSecrets []runtime.Object, spcs []runtime.Object) (*ConfigurationDomainController, chan messaging.RcvMsg) {
	platformClient := fakeClientset.NewSimpleClientset(append(platforms, configurationDomains...)...)
	kubeClient := kubeFakeClientSet.NewSimpleClientset(append(configMaps, kubeSecrets...)...)
	csiClient := fakeCsiClientSet.NewSimpleClientset(spcs...)

	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	csiInformerFactory := csiinformers.NewSharedInformerFactory(csiClient, time.Second*30)

	msgChan := make(chan messaging.RcvMsg)
	msgPublisher := messaging.MessagingPublisherMock(msgChan)

	c := NewConfigurationDomainController(platformClient, kubeClient, csiClient,
		platformInformerFactory.Platform().V1alpha1().Platforms(),
		platformInformerFactory.Configuration().V1alpha1().ConfigurationDomains(),
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().Secrets(),
		csiInformerFactory.Secretsstore().V1().SecretProviderClasses(),
		nil,
		msgPublisher,
	)

	platformInformerFactory.Start(nil)
	kubeInformerFactory.Start(nil)
	csiInformerFactory.Start(nil)

	platformInformerFactory.WaitForCacheSync(nil)
	kubeInformerFactory.WaitForCacheSync(nil)
	csiInformerFactory.WaitForCacheSync(nil)

	return c, msgChan
}
