package controllers

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeCsiClientSet "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	csiinformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions"
	configurationv1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
)

func TestSecretsAggregateController_processNextWorkItem(t *testing.T) {

	t.Run("aggregate two secrets", func(t *testing.T) {
		// Arrange
		secretsAggregates := []runtime.Object{
			newSecretsAggregate("secretsAggregate1", "domain1", "dev"),
		}
		spcs := []runtime.Object{}
		c := runSecretsController(secretsAggregates, spcs)

		var oldGetSecrets = getSecrets
		defer func() { getSecrets = oldGetSecrets }()
		getSecrets = func(platform, domain, role string) ([]secretSpec, error) {
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

		output, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(metav1.NamespaceDefault).Get(context.TODO(), "dev-domain1-aggregate", metav1.GetOptions{})
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
	})
}

func newSecretsAggregate(name, domain, platform string) *configurationv1.SecretsAggregate {
	return &configurationv1.SecretsAggregate{
		TypeMeta: metav1.TypeMeta{APIVersion: configurationv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: configurationv1.SecretsAggregateSpec{
			PlatformRef: platform,
			Domain:      domain,
		},
	}
}
func runSecretsController(secretsAggregates, spcs []runtime.Object) *SecretsController {
	csiClient := fakeCsiClientSet.NewSimpleClientset(spcs...)
	platformClient := fakeClientset.NewSimpleClientset(secretsAggregates...)

	csiInformerFactory := csiinformers.NewSharedInformerFactory(csiClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	c := NewSecretsController(csiClient, platformClient, csiInformerFactory.Secretsstore().V1().SecretProviderClasses(),
		platformInformerFactory.Configuration().V1alpha1().SecretsAggregates(), nil)
	csiInformerFactory.Start(nil)
	platformInformerFactory.Start(nil)

	csiInformerFactory.WaitForCacheSync(nil)
	platformInformerFactory.WaitForCacheSync(nil)

	return c
}
