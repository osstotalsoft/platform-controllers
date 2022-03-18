package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sync"
	"testing"
	"time"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
)

func TestProvisioningController_processNextWorkItem(t *testing.T) {
	type result struct {
		platform string
		tenant   *provisioningv1.Tenant
		azureDbs []*provisioningv1.AzureDatabase
	}

	t.Run("add three tenants", func(t *testing.T) {
		var outputs []result
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newTenant("dev2", "dev"),
			newTenant("dev3", "qa"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) error {
			outputs = append(outputs, result{platform, tenant, azureDbs})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 3 {
			t.Error("queue should have only 3 items, but it has", c.workqueue.Len())
		}

		//c.factory.Provisioning().V1alpha1().Tenants().Informer().GetIndexer().Add(tenant)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}
		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}
		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		if len(outputs) != 3 {
			t.Error("expected 3 outputs, got", len(outputs))
			return
		}
		if outputs[0].tenant.Name != "dev1" {
			t.Error("expected output tenant dev1, got", outputs[0].tenant.Name)
		}

	})

	t.Run("add same tenant multiple times", func(t *testing.T) {
		var outputs []result
		tenant := newTenant("dev1", "dev")
		clientset := fakeClientset.NewSimpleClientset(tenant)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) error {
			outputs = append(outputs, result{platform, tenant, azureDbs})
			wg.Wait()
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		go c.processNextWorkItem(1)
		go c.factory.Provisioning().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
		go c.factory.Provisioning().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
		time.Sleep(time.Second)
		wg.Done()

		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		if len(outputs) != 1 {
			t.Error("expected 1 output, got", len(outputs))
			return
		}
		if outputs[0].tenant.Name != "dev1" {
			t.Error("expected output tenant dev1, got", outputs[0].tenant.Name)
		}

	})

	t.Run("add one tenant and one database same platform", func(t *testing.T) {
		var outputs []result
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newDb("db1", "dev"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) error {
			outputs = append(outputs, result{platform, tenant, azureDbs})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		//c.factory.Provisioning().V1alpha1().Tenants().Informer().GetIndexer().Add(tenant)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		if len(outputs) != 1 {
			t.Error("expected 1 output, got", len(outputs))
			return
		}
		if len(outputs[0].azureDbs) != 1 {
			t.Error("expected one db, got", len(outputs[0].azureDbs))
		}
	})
	t.Run("add one tenant and one database different platforms", func(t *testing.T) {
		var outputs []result
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newDb("db1", "dev2"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, azureDbs []*provisioningv1.AzureDatabase) error {
			outputs = append(outputs, result{platform, tenant, azureDbs})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		//c.factory.Provisioning().V1alpha1().Tenants().Informer().GetIndexer().Add(tenant)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		if len(outputs) != 1 {
			t.Error("expected 1 output, got", len(outputs))
			return
		}
		if len(outputs[0].azureDbs) != 0 {
			t.Error("expected no db, got", len(outputs[0].azureDbs))
		}
	})
}

func newTenant(name, platform string) *provisioningv1.Tenant {
	return &provisioningv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.TenantSpec{
			PlatformRef: platform,
			Code:        name,
		},
	}
}

func newDb(name, platform string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureDatabaseSpec{
			PlatformRef: platform,
			Name:        name,
		},
	}
}
