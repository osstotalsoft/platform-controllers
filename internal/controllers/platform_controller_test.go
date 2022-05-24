package controllers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubeFakeClientSet "k8s.io/client-go/kubernetes/fake"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
)

func TestPlatformController_processNextWorkItem(t *testing.T) {

	t.Run("one platform with two tenants", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		tenant1 := _newTenant("tenant1", "charismaonline.qa")
		tenant2 := _newTenant("tenant2", "charismaonline.qa")

		c := _runController([]runtime.Object{platform, tenant1, tenant2})
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

		output, err := c.kubeClientset.CoreV1().ConfigMaps("qa").Get(context.TODO(), "charismaonline.qa-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{
			"MultiTenancy__Tenants__tenant1____Code":     tenant1.Spec.Code,
			"MultiTenancy__Tenants__tenant1____TenantId": tenant1.Spec.Id,
			"MultiTenancy__Tenants__tenant2____Code":     tenant2.Spec.Code,
			"MultiTenancy__Tenants__tenant2____TenantId": tenant2.Spec.Id,
		}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	t.Run("orphan tenants", func(t *testing.T) {
		// Arrange
		tenant1 := _newTenant("tenant1", "charismaonline.qa")
		tenant2 := _newTenant("tenant2", "charismaonline.qa")

		c := _runController([]runtime.Object{tenant1, tenant2})
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

		//expect no error
	})

	t.Run("a platform with no tenants", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")

		c := _runController([]runtime.Object{platform})
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

		output, err := c.kubeClientset.CoreV1().ConfigMaps("qa").Get(context.TODO(), "charismaonline.qa-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}
	})

	t.Run("tenant platformRef updated", func(t *testing.T) {
		// Arrange
		platformQa := _newPlatform("qa", "charismaonline.qa")
		platformUat := _newPlatform("uat", "charismaonline.uat")
		tenant1 := _newTenant("tenant1", "charismaonline.qa")

		c := _runController([]runtime.Object{platformQa, platformUat, tenant1})
		if c.workqueue.Len() != 2 {
			t.Error("queue should have 2 items, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		t1, err := c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Get(context.TODO(), "tenant1", metav1.GetOptions{})
		if err != nil {
			t.Error("tenant not found")
		}
		t1 = t1.DeepCopy()
		t1.Spec.PlatformRef = "charismaonline.uat"
		t1, _ = c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Update(context.TODO(), t1, metav1.UpdateOptions{})
		c.platformInformer.Informer().GetIndexer().Update(t1) //fix stale cache
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

		qaConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps("qa").Get(context.TODO(), "charismaonline.qa-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		if expectedOutput := map[string]string{}; !reflect.DeepEqual(qaConfigMap.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", qaConfigMap.Data)
		}

		uatConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps("uat").Get(context.TODO(), "charismaonline.uat-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		if expectedOutput := map[string]string{
			"MultiTenancy__Tenants__tenant1____Code":     tenant1.Spec.Code,
			"MultiTenancy__Tenants__tenant1____TenantId": tenant1.Spec.Id,
		}; !reflect.DeepEqual(uatConfigMap.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", uatConfigMap.Data)
		}
	})
}

func _newPlatform(code, name string) *platformv1.Platform {
	return &platformv1.Platform{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: platformv1.PlatformSpec{
			Code: code,
		},
	}
}

func _newTenant(name, platform string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Code:        name,
			Id:          uuid.New().String(),
		},
	}
}

func _runController(objects []runtime.Object) *PlatformController {
	kubeClient := kubeFakeClientSet.NewSimpleClientset()
	platformClient := fakeClientset.NewSimpleClientset(objects...)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	c := NewPlatformController(kubeClient, platformClient, kubeInformerFactory.Core().V1().ConfigMaps(),
		platformInformerFactory.Platform().V1alpha1().Platforms(), platformInformerFactory.Platform().V1alpha1().Tenants(), nil)
	kubeInformerFactory.Start(nil)
	platformInformerFactory.Start(nil)

	kubeInformerFactory.WaitForCacheSync(nil)
	platformInformerFactory.WaitForCacheSync(nil)

	return c
}
