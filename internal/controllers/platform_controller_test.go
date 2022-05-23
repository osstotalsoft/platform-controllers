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
