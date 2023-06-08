package provisioning

import (
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	provisioners "totalsoft.ro/platform-controllers/internal/controllers/provisioning/provisioners"
	"totalsoft.ro/platform-controllers/internal/messaging"
	messagingMock "totalsoft.ro/platform-controllers/internal/messaging/mock"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
)

func TestProvisioningController_processNextWorkItem(t *testing.T) {

	t.Run("add three tenants", func(t *testing.T) {
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newTenant("dev2", "dev"),
			newTenant("dev3", "qa"),
		}

		c, outputs, msgChan := runControllerWithDefaultFakes(objects)

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

		if len(*outputs) != 3 {
			t.Error("expected 3 outputs, got", len(*outputs))
			return
		}
		if (*outputs)[0].tenant.Name != "dev1" {
			t.Error("expected output tenant dev1, got", (*outputs)[0].tenant.Name)
		}

		for i := 0; i < 3; i++ {
			msg := <-msgChan
			if msg.Topic != tenantProvisionedSuccessfullyTopic {
				t.Error("expected message pblished to topic ", tenantProvisionedSuccessfullyTopic, ", got", msg.Topic)
			}
		}

	})

	t.Run("skip tenant provisioning", func(t *testing.T) {
		aSkipProvisioningTenant := newTenant("dev1", "dev")
		aSkipProvisioningTenant.ObjectMeta.Labels = map[string]string{
			SkipProvisioningLabel: "true",
		}
		objects := []runtime.Object{
			aSkipProvisioningTenant,
		}
		c, _, _ := runControllerWithDefaultFakes(objects)

		if c.workqueue.Len() != 0 {
			t.Error("queue should have only 0 items, but it has", c.workqueue.Len())
		}

	})

	t.Run("ignores same tenant updates while an update for same tenant in progress", func(t *testing.T) {
		//Arrange
		tenant := newTenant("dev1", "dev")
		objects := []runtime.Object{tenant}

		wg := &sync.WaitGroup{}
		wg.Add(1)

		var outputs []provisionerResult
		infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) provisioners.ProvisioningResult {
			outputs = append(outputs, provisionerResult{platform, tenant, infra})
			wg.Wait() //wait for other tenant updates
			return provisioners.ProvisioningResult{}
		}
		c := runController(objects, infraCreator, messaging.NilMessagingPublisher)

		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		//Act
		go c.processNextWorkItem(1)
		go c.factory.Platform().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
		go c.factory.Platform().V1alpha1().Tenants().Informer().GetIndexer().Update(tenant)
		time.Sleep(time.Second)
		wg.Done()

		//Assert
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
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newAzureDb("db1", "dev"),
			newAzureManagedDb("db1", "dev"),
		}
		c, outputs, msgChan := runControllerWithDefaultFakes(objects)

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

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 1 {
			t.Error("expected one db, got", len((*outputs)[0].infra.AzureDbs))
		}
		if len((*outputs)[0].infra.AzureManagedDbs) != 1 {
			t.Error("expected one managed db, got", len((*outputs)[0].infra.AzureManagedDbs))
		}

		msg := <-msgChan
		if msg.Topic != tenantProvisionedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", tenantProvisionedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("add one tenant and one database different platforms", func(t *testing.T) {
		objects := []runtime.Object{
			newTenant("dev1", "dev"),
			newAzureDb("db1", "dev2"),
			newAzureManagedDb("db1", "dev2"),
		}
		c, outputs, msgChan := runControllerWithDefaultFakes(objects)

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

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 0 {
			t.Error("expected no db, got", len((*outputs)[0].infra.AzureDbs))
		}
		if len((*outputs)[0].infra.AzureManagedDbs) != 0 {
			t.Error("expected no managed db, got", len((*outputs)[0].infra.AzureManagedDbs))
		}

		msg := <-msgChan
		if msg.Topic != tenantProvisionedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", tenantProvisionedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("skip tenant resource provisioning", func(t *testing.T) {

		tenant := newTenant("dev1", "dev")
		azureDb := newAzureDb("db1", "dev")
		azureDb.ObjectMeta.Labels = map[string]string{
			"provisioning.totalsoft.ro/skip-tenant-dev1": "true",
		}
		objects := []runtime.Object{
			tenant,
			azureDb,
		}
		c, outputs, _ := runControllerWithDefaultFakes(objects)

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

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 0 {
			t.Error("expected zero dbs, got", len((*outputs)[0].infra.AzureDbs))
		}
	})

	t.Run("filter resource by Domain", func(t *testing.T) {

		tenant := newTenantWithService("dev1", "dev", "p1")
		azureDb := newAzureDbWithService("db1", "dev", "p2")
		objects := []runtime.Object{
			tenant,
			azureDb,
		}
		c, outputs, _ := runControllerWithDefaultFakes(objects)

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

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 0 {
			t.Error("expected zero dbs, got", len((*outputs)[0].infra.AzureDbs))
		}
	})
}

func newTenantWithService(name, platform, domain string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Description: name + " description",
			DomainRefs:  []string{domain},
		},
	}
}

func newTenant(name, platform string) *platformv1.Tenant {
	return newTenantWithService(name, platform, "")
}

func newAzureDbWithService(name, platform, domain string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureDatabaseSpec{
			PlatformRef: platform,
			DbName:      name,
			DomainRef:   domain,
		},
	}
}

func newAzureDb(name, platform string) *provisioningv1.AzureDatabase {
	return newAzureDbWithService(name, platform, "")
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

func runController(objects []runtime.Object, provisioner provisioners.CreateInfrastructureFunc, msgPublisher messaging.MessagingPublisher) *ProvisioningController {
	clientset := fakeClientset.NewSimpleClientset(objects...)

	c := NewProvisioningController(clientset, provisioner, nil, nil, msgPublisher)
	c.factory.Start(nil)
	c.factory.WaitForCacheSync(nil)

	return c
}

func runControllerWithDefaultFakes(objects []runtime.Object) (*ProvisioningController, *[]provisionerResult, chan messagingMock.RcvMsg) {
	var outputs []provisionerResult

	infraCreator := func(platform string, tenant *platformv1.Tenant, infra *provisioners.InfrastructureManifests) provisioners.ProvisioningResult {
		outputs = append(outputs, provisionerResult{platform, tenant, infra})
		return provisioners.ProvisioningResult{}
	}

	msgChan := make(chan messagingMock.RcvMsg)
	msgPublisher := messagingMock.MessagingPublisherMock(msgChan)

	c := runController(objects, infraCreator, msgPublisher)
	c.factory.Start(nil)
	c.factory.WaitForCacheSync(nil)

	return c, &outputs, msgChan
}

type provisionerResult struct {
	platform string
	tenant   *platformv1.Tenant
	infra    *provisioners.InfrastructureManifests
}
