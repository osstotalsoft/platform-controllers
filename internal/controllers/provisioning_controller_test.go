package controllers

import (
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
)

func TestProvisioningController_processNextWorkItem(t *testing.T) {
	type result struct {
		platform string
		tenant   *platformv1.Tenant
		infra    *provisioners.InfrastructureManifests
	}

	t.Run("add three tenants", func(t *testing.T) {
		var outputs []result
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newTenant("dev2", "dev"),
			newTenant("dev3", "qa"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
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

	t.Run("skip tenant provisioning", func(t *testing.T) {
		var outputs []result
		aSkipProvisioningTenant := newTenant("dev1", "dev")
		aSkipProvisioningTenant.ObjectMeta.Labels = map[string]string{
			SkipProvisioningLabel: "true",
		}
		objects := []runtime.Object{
			aSkipProvisioningTenant,
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 0 {
			t.Error("queue should have only 0 items, but it has", c.workqueue.Len())
		}

	})

	t.Run("add same tenant multiple times", func(t *testing.T) {
		var outputs []result
		tenant := newTenant("dev1", "dev")
		clientset := fakeClientset.NewSimpleClientset(tenant)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			wg.Wait()
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		go c.processNextWorkItem(1)
		go c.factory.Platform().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
		go c.factory.Platform().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
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
			newAzureDb("db1", "dev"),
			newAzureManagedDb("db1", "dev"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
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
		if len(outputs[0].infra.AzureDbs) != 1 {
			t.Error("expected one db, got", len(outputs[0].infra.AzureDbs))
		}
		if len(outputs[0].infra.AzureManagedDbs) != 1 {
			t.Error("expected one managed db, got", len(outputs[0].infra.AzureManagedDbs))
		}
	})

	t.Run("add one tenant and one database different platforms", func(t *testing.T) {
		var outputs []result
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newAzureDb("db1", "dev2"),
			newAzureManagedDb("db1", "dev2"),
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
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
		if len(outputs[0].infra.AzureDbs) != 0 {
			t.Error("expected no db, got", len(outputs[0].infra.AzureDbs))
		}
		if len(outputs[0].infra.AzureManagedDbs) != 0 {
			t.Error("expected no managed db, got", len(outputs[0].infra.AzureManagedDbs))
		}
	})

	t.Run("skip tenant resource provisioning", func(t *testing.T) {
		var outputs []result

		tenant := newTenant("dev1", "dev")
		azureDb := newAzureDb("db1", "dev")
		azureDb.ObjectMeta.Labels = map[string]string{
			"provisioning.totalsoft.ro/skip-tenant-dev1": "true",
		}
		objects := []runtime.Object{
			tenant,
			azureDb,
		}
		clientset := fakeClientset.NewSimpleClientset(objects...)
		infraCreator := func(platform string, tenant *provisioningv1.Tenant, infra *provisioners.InfrastructureManifests) error {
			outputs = append(outputs, result{platform, tenant, infra})
			return nil
		}
		c := NewProvisioningController(clientset, infraCreator, nil, nil)
		c.factory.Start(nil)
		c.factory.WaitForCacheSync(nil)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

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
		if len(outputs[0].infra.AzureDbs) != 0 {
			t.Error("expected zero dbs, got", len(outputs[0].infra.AzureDbs))
		}
	})
}

func newTenant(name, platform string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Code:        name,
		},
	}
}

func newAzureDb(name, platform string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureDatabaseSpec{
			PlatformRef: platform,
			DbName:      name,
		},
	}
}

func newAzureManagedDb(dbName, platform string) *provisioningv1.AzureManagedDatabase {
	return &provisioningv1.AzureManagedDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureManagedDatabaseSpec{
			PlatformRef: platform,
			DbName:      dbName,
		},
	}
}
