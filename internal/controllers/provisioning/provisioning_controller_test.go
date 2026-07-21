package provisioning

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"time"

	"github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"totalsoft.ro/platform-controllers/internal/messaging"
	messagingMock "totalsoft.ro/platform-controllers/internal/messaging/mock"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
)

func TestProvisioningController_processNextWorkItem(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	t.Run("add three tenants", func(t *testing.T) {
		domain := "my-domain"
		objects := []runtime.Object{
			newTenant("dev1", "dev", domain),
			newTenant("dev2", "dev", domain),
			newTenant("dev3", "qa", domain),
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
		if (*outputs)[0].target.GetName() != "dev1" {
			t.Error("expected output tenant dev1, got", (*outputs)[0].target.GetName())
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

	t.Run("ignores same tenant-domain updates while an update for same tenant-domain in progress", func(t *testing.T) {
		//Arrange
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		objects := []runtime.Object{tenant}

		wg := &sync.WaitGroup{}
		wg.Add(1)

		var outputs []provisionerResult
		infraCreator := func(target ProvisioningTarget, domain string, infra *InfrastructureManifests) ProvisioningResult {
			outputs = append(outputs, provisionerResult{target.GetPlatformName(), target, domain, infra})
			wg.Wait() //wait for other tenant updates
			return ProvisioningResult{}
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
		if outputs[0].target.GetName() != "dev1" {
			t.Error("expected output tenant dev1, got", outputs[0].target.GetName())
		}
	})

	t.Run("add one tenant and one database same platform", func(t *testing.T) {
		domain := "my-domain"
		objects := []runtime.Object{
			newTenant("dev1", "dev", domain),
			newAzureDb("db1", "dev", domain),
			newAzureManagedDb("db1", "dev", domain),
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
		domain := "my-domain"
		objects := []runtime.Object{
			newTenant("dev1", "dev", domain),
			newAzureDb("db1", "dev2", domain),
			newAzureManagedDb("db1", "dev2", domain),
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
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		azureDb := newAzureDb("db1", "dev", domain)
		azureDb.Spec.Target.Filter.Kind = provisioningv1.ProvisioningFilterKindBlacklist
		azureDb.Spec.Target.Filter.Values = []string{"dev1"}

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

	t.Run("filter resource by tenant Category (whitelist match)", func(t *testing.T) {
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		tenant.Spec.CategoryRef = "CEEAS"
		azureDb := newAzureDb("db1", "dev", domain)
		azureDb.Spec.Target.Filter.Kind = provisioningv1.ProvisioningFilterKindWhitelist
		azureDb.Spec.Target.Filter.By = provisioningv1.ProvisioningFilterByCategory
		azureDb.Spec.Target.Filter.Values = []string{"CEEAS"}

		objects := []runtime.Object{
			tenant,
			azureDb,
			newTenantCategory("CEEAS", "dev", "", "", nil),
		}
		c, outputs, _ := runControllerWithDefaultFakes(objects)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 1 {
			t.Error("expected one db for tenant in whitelisted category, got", len((*outputs)[0].infra.AzureDbs))
		}
	})

	t.Run("filter resource by tenant Category (whitelist no match)", func(t *testing.T) {
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		tenant.Spec.CategoryRef = "OTHER"
		azureDb := newAzureDb("db1", "dev", domain)
		azureDb.Spec.Target.Filter.Kind = provisioningv1.ProvisioningFilterKindWhitelist
		azureDb.Spec.Target.Filter.By = provisioningv1.ProvisioningFilterByCategory
		azureDb.Spec.Target.Filter.Values = []string{"CEEAS"}

		objects := []runtime.Object{
			tenant,
			azureDb,
			newTenantCategory("OTHER", "dev", "", "", nil),
		}
		c, outputs, _ := runControllerWithDefaultFakes(objects)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if len(*outputs) != 1 {
			t.Error("expected 1 output, got", len(*outputs))
			return
		}
		if len((*outputs)[0].infra.AzureDbs) != 0 {
			t.Error("expected zero dbs for tenant not in whitelisted category, got", len((*outputs)[0].infra.AzureDbs))
		}
	})

	t.Run("tenant with unresolvable categoryRef fails provisioning", func(t *testing.T) {
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		tenant.Spec.CategoryRef = "does-not-exist"

		objects := []runtime.Object{
			tenant,
			newAzureDb("db1", "dev", domain),
		}
		c, outputs, _ := runControllerWithDefaultFakes(objects)

		if result := c.processNextWorkItem(1); !result {
			t.Error("processing failed")
		}

		if len(*outputs) != 0 {
			t.Error("expected 0 outputs since the categoryRef cannot be resolved, got", len(*outputs))
		}
	})

	t.Run("updating a TenantCategory re-enqueues tenants referencing it", func(t *testing.T) {
		domain := "my-domain"
		tenant := newTenant("dev1", "dev", domain)
		tenant.Spec.CategoryRef = "cat1"
		category := newTenantCategory("cat1", "dev", "", "", nil)

		objects := []runtime.Object{tenant, category}
		c, _, _ := runControllerWithDefaultFakes(objects)

		// drain the initial enqueue triggered by adding the tenant
		for c.workqueue.Len() > 0 {
			item, _ := c.workqueue.Get()
			c.workqueue.Done(item)
			c.workqueue.Forget(item)
		}

		updatedCategory := category.DeepCopy()
		updatedCategory.Spec.Description = "changed"
		_, err := c.clientset.PlatformV1alpha1().TenantCategories(metav1.NamespaceDefault).Update(context.TODO(), updatedCategory, metav1.UpdateOptions{})
		if err != nil {
			t.Error(err)
		}
		time.Sleep(time.Second)

		if c.workqueue.Len() != 1 {
			t.Error("expected 1 item enqueued after TenantCategory update, but queue has", c.workqueue.Len())
		}
	})

	t.Run("filter resource by Domain", func(t *testing.T) {
		tenant := newTenant("dev1", "dev", "p1")
		azureDb := newAzureDb("db1", "dev", "p2")
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

func TestProvisioningController_applyTargetOverrides(t *testing.T) {

	t.Run("apply managedDb tenant overrides", func(t *testing.T) {
		tenantName := "tenant1"
		overrides := map[string]any{
			"restoreFrom": map[string]any{
				"backupFileName": "afterBackupFileName",
			},
		}

		overridesBytes, _ := json.Marshal(overrides)

		db := provisioningv1.AzureManagedDatabase{
			Spec: provisioningv1.AzureManagedDatabaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					PlatformRef: "platform",
					TenantOverrides: map[string]*v1.JSON{
						tenantName: {Raw: overridesBytes},
					},
				},
				RestoreFrom: provisioningv1.AzureManagedDatabaseRestoreSpec{
					BackupFileName: "beforeBackupFileName",
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "afterBackupFileName", result[0].Spec.RestoreFrom.BackupFileName)
	})

	t.Run("override helmRelease version", func(t *testing.T) {
		tenantName := "tenant1"
		overrides := map[string]any{
			"release": map[string]any{
				"chart": map[string]any{
					"spec": map[string]any{
						"version": "1.1.1",
					},
				},
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		hr := provisioningv1.HelmRelease{
			Spec: provisioningv1.HelmReleaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					TenantOverrides: map[string]*v1.JSON{
						tenantName: {Raw: overridesBytes},
					},
				},
				Release: v2beta1.HelmReleaseSpec{
					Chart: &v2beta1.HelmChartTemplate{
						Spec: v2beta1.HelmChartTemplateSpec{
							Version: "0.0.0-0",
						},
					},
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.HelmRelease{&hr}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "1.1.1", result[0].Spec.Release.Chart.Spec.Version)
	})

	t.Run("override avd params", func(t *testing.T) {
		tenantName := "tenant1"
		overrides := map[string]any{
			"initScriptArgs": []map[string]any{
				{"name": "arg1NameAfter", "value": "arg1ValueAfter"},
			},
			"users": map[string]any{
				"applicationUsers": []string{
					"user1After",
					"user2After",
				},
			},
			"vmNumberOfInstances": 2,
		}
		overridesBytes, _ := json.Marshal(overrides)

		avd := provisioningv1.AzureVirtualDesktop{
			Spec: provisioningv1.AzureVirtualDesktopSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					TenantOverrides: map[string]*v1.JSON{
						tenantName: {Raw: overridesBytes},
					},
				},
				InitScriptArguments: []provisioningv1.InitScriptArgs{
					{Name: "arg1NameBefore", Value: "arg1ValueBefore"},
					{Name: "arg2NameBefore", Value: "arg2ValueBefore"},
				},
				Users: provisioningv1.AzureVirtualDesktopUsersSpec{
					ApplicationUsers: []string{"user1Before", "user2Before"},
				},
				VmNumberOfInstances: 1,
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.AzureVirtualDesktop{&avd}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, []provisioningv1.InitScriptArgs{{Name: "arg1NameAfter", Value: "arg1ValueAfter"}},
			result[0].Spec.InitScriptArguments)

		assert.Equal(t, []string{"user1After", "user2After"}, result[0].Spec.Users.ApplicationUsers)
		assert.Equal(t, 2, result[0].Spec.VmNumberOfInstances)
	})

	t.Run("override contents of JSON field", func(t *testing.T) {
		tenantName := "tenant1"
		overrides := map[string]any{
			"release": map[string]any{
				"values": map[string]any{
					"env": map[string]any{
						"key1": "envValue1After",
						"key3": "envValue3After",
					},
				},
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		valuesBefore := map[string]any{
			"env": map[string]any{
				"key1": "envValue1Before",
				"key2": "envValue2Before",
			},
		}
		valuesBeforeBytes, _ := json.Marshal(valuesBefore)

		hr := provisioningv1.HelmRelease{
			Spec: provisioningv1.HelmReleaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					TenantOverrides: map[string]*v1.JSON{
						tenantName: {Raw: overridesBytes},
					},
				},
				Release: v2beta1.HelmReleaseSpec{
					Values: &v1.JSON{Raw: valuesBeforeBytes},
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.HelmRelease{&hr}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)

		var valuesMap map[string]any
		if err := json.Unmarshal(result[0].Spec.Release.Values.Raw, &valuesMap); err != nil {
			t.Error(err)
		}

		key1 := valuesMap["env"].(map[string]any)["key1"].(string)
		key2 := valuesMap["env"].(map[string]any)["key2"].(string)
		key3 := valuesMap["env"].(map[string]any)["key3"].(string)

		assert.Equal(t, key1, "envValue1After")
		assert.Equal(t, key2, "envValue2Before")
		assert.Equal(t, key3, "envValue3After")
	})

	t.Run("override with empty value", func(t *testing.T) {
		tenantName := "tenant1"
		num := 1
		overrides := map[string]any{
			"release": map[string]any{
				"targetNamespace": "",
				"maxHistory":      nil,
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		hr := provisioningv1.HelmRelease{
			Spec: provisioningv1.HelmReleaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					TenantOverrides: map[string]*v1.JSON{
						tenantName: {Raw: overridesBytes},
					},
				},
				Release: v2beta1.HelmReleaseSpec{
					TargetNamespace:  "targetNamespaceBefore",
					StorageNamespace: "storageNamespaceBefore",
					MaxHistory:       &num,
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.HelmRelease{&hr}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "", result[0].Spec.Release.TargetNamespace)
		assert.Equal(t, "storageNamespaceBefore", result[0].Spec.Release.StorageNamespace)
		assert.Nil(t, result[0].Spec.Release.MaxHistory)
	})

	t.Run("overrides don't mutate source", func(t *testing.T) {
		tenantName := "tenant1"
		overrides := map[string]any{
			"release": map[string]any{
				"releaseName": "releaseNameAfter0",
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		hrs := []*provisioningv1.HelmRelease{
			{
				Spec: provisioningv1.HelmReleaseSpec{
					ProvisioningMeta: provisioningv1.ProvisioningMeta{
						TenantOverrides: map[string]*v1.JSON{
							tenantName: {Raw: overridesBytes},
						},
					},
					Release: v2beta1.HelmReleaseSpec{
						ReleaseName: "releaseNameBefore0",
					},
				},
			},
			{
				Spec: provisioningv1.HelmReleaseSpec{
					Release: v2beta1.HelmReleaseSpec{
						ReleaseName: "releaseNameBefore1",
					},
				},
			},
		}

		result, err := applyTargetOverrides(hrs, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 2)
		assert.NotSame(t, hrs[0], result[0])
		assert.Equal(t, "releaseNameBefore0", hrs[0].Spec.Release.ReleaseName)
		assert.Equal(t, "releaseNameAfter0", result[0].Spec.Release.ReleaseName)
		assert.Same(t, hrs[1], result[1])
		assert.Equal(t, "releaseNameBefore1", result[1].Spec.Release.ReleaseName)
	})

	t.Run("apply category-map overrides", func(t *testing.T) {
		tenantName := "tenant1"
		categoryName := "CEEAS"
		overrides := map[string]any{
			"restoreFrom": map[string]any{
				"backupFileName": "fromCategoryMap",
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		db := provisioningv1.AzureManagedDatabase{
			Spec: provisioningv1.AzureManagedDatabaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					PlatformRef: "platform",
					CategoryOverrides: map[string]*v1.JSON{
						categoryName: {Raw: overridesBytes},
					},
				},
				RestoreFrom: provisioningv1.AzureManagedDatabaseRestoreSpec{
					BackupFileName: "beforeBackupFileName",
				},
			},
		}

		tenant := newTenant(tenantName, "platform", "domain")
		tenant.Spec.CategoryRef = categoryName

		result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, tenant, nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "fromCategoryMap", result[0].Spec.RestoreFrom.BackupFileName)
	})

	t.Run("normalize missing TypeMeta on LocalScript", func(t *testing.T) {
		script := provisioningv1.LocalScript{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script1",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: provisioningv1.LocalScriptSpec{
				CreateScriptContent: "Write-Host 'Hello'",
				DeleteScriptContent: "Write-Host 'Bye'",
				Shell:               provisioningv1.LocalScriptShellPwsh,
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					PlatformRef: "platform",
					DomainRef:   "domain",
					Target: provisioningv1.ProvisioningTarget{
						Category: provisioningv1.ProvisioningTargetCategoryTenant,
					},
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.LocalScript{&script}, newTenant("tenant1", "platform", "domain"), nil)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d", len(result))
		}

		gvk := result[0].GetObjectKind().GroupVersionKind()
		assert.Equal(t, provisioningv1.SchemeGroupVersion.String(), gvk.GroupVersion().String())
		assert.Equal(t, "LocalScript", gvk.Kind)
	})

	t.Run("apply TenantCategory overrides", func(t *testing.T) {
		tenantName := "tenant1"
		categoryName := "CEEAS"
		overrides := map[string]any{
			"restoreFrom": map[string]any{
				"backupFileName": "fromCategory",
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		db := provisioningv1.AzureManagedDatabase{
			TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String(), Kind: "AzureManagedDatabase"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db1",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: provisioningv1.AzureManagedDatabaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					PlatformRef: "platform",
				},
				RestoreFrom: provisioningv1.AzureManagedDatabaseRestoreSpec{
					BackupFileName: "beforeBackupFileName",
				},
			},
		}

		tenant := newTenant(tenantName, "platform", "domain")
		tenant.Spec.CategoryRef = categoryName

		category := newTenantCategory(categoryName, "platform", "AzureManagedDatabase", "db1",overridesBytes)

		result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, tenant, category)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "fromCategory", result[0].Spec.RestoreFrom.BackupFileName)
	})

	t.Run("category-map, TenantCategory, tenant name and tenant-spec overrides precedence", func(t *testing.T) {
		tenantName := "tenant1"
		categoryName := "CEEAS"

		categoryMapOverrideBytes, _ := json.Marshal(map[string]any{
			"restoreFrom": map[string]any{"backupFileName": "fromCategoryMap"},
		})
		categoryOverrideBytes, _ := json.Marshal(map[string]any{
			"restoreFrom": map[string]any{"backupFileName": "fromCategory"},
		})
		tenantNameOverrideBytes, _ := json.Marshal(map[string]any{
			"restoreFrom": map[string]any{"backupFileName": "fromTenantName"},
		})
		tenantSpecOverrideBytes, _ := json.Marshal(map[string]any{
			"restoreFrom": map[string]any{"backupFileName": "fromTenantSpec"},
		})

		newDb := func() provisioningv1.AzureManagedDatabase {
			return provisioningv1.AzureManagedDatabase{
				TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String(), Kind: "AzureManagedDatabase"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: provisioningv1.AzureManagedDatabaseSpec{
					ProvisioningMeta: provisioningv1.ProvisioningMeta{
						PlatformRef: "platform",
						CategoryOverrides: map[string]*v1.JSON{
							categoryName: {Raw: categoryMapOverrideBytes},
						},
						TenantOverrides: map[string]*v1.JSON{
							tenantName: {Raw: tenantNameOverrideBytes},
						},
					},
					RestoreFrom: provisioningv1.AzureManagedDatabaseRestoreSpec{
						BackupFileName: "original",
					},
				},
			}
		}

		tenant := newTenant(tenantName, "platform", "domain")
		tenant.Spec.CategoryRef = categoryName
		tenant.ObjectMeta.Namespace = metav1.NamespaceDefault

		t.Run("TenantCategory override wins over category-map override", func(t *testing.T) {
			db := newDb()
			db.Spec.TenantOverrides = nil // isolate from the tenant-name-glob tier for this assertion
			category := newTenantCategory(categoryName, "platform", "AzureManagedDatabase", "db1",categoryOverrideBytes)
			result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, tenant, category)
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, result, 1)
			assert.Equal(t, "fromCategory", result[0].Spec.RestoreFrom.BackupFileName)
		})

		t.Run("tenant name override wins over TenantCategory and category-map overrides", func(t *testing.T) {
			db := newDb()
			category := newTenantCategory(categoryName, "platform", "AzureManagedDatabase", "db1",categoryOverrideBytes)
			result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, tenant, category)
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, result, 1)
			assert.Equal(t, "fromTenantName", result[0].Spec.RestoreFrom.BackupFileName)
		})

		t.Run("tenant-specific ProvisioningOverrides wins over all other overrides", func(t *testing.T) {
			db := newDb()
			category := newTenantCategory(categoryName, "platform", "AzureManagedDatabase", "db1",categoryOverrideBytes)
			tenantWithSpecOverride := tenant.DeepCopy()
			tenantWithSpecOverride.Spec.ProvisioningOverrides = []platformv1.ProvisioningResourcePatch{
				{
					Target: platformv1.ProvisioningResourcePatchTarget{
						Kind:       "AzureManagedDatabase",
						APIVersion: provisioningv1.SchemeGroupVersion.String(),
						Name:       "db1",
						Namespace:  metav1.NamespaceDefault,
					},
					Spec: &v1.JSON{Raw: tenantSpecOverrideBytes},
				},
			}

			result, err := applyTargetOverrides([]*provisioningv1.AzureManagedDatabase{&db}, tenantWithSpecOverride, category)
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, result, 1)
			assert.Equal(t, "fromTenantSpec", result[0].Spec.RestoreFrom.BackupFileName)
		})
	})

	t.Run("apply wildcard tenant overrides", func(t *testing.T) {
		tenantName := "tenantPrefix-abc"
		wildcardKey := "tenantPrefix-*"
		overrides := map[string]any{
			"release": map[string]any{
				"releaseName": "overriddenReleaseName",
			},
		}
		overridesBytes, _ := json.Marshal(overrides)

		hr := provisioningv1.HelmRelease{
			Spec: provisioningv1.HelmReleaseSpec{
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					TenantOverrides: map[string]*v1.JSON{
						wildcardKey: {Raw: overridesBytes},
					},
				},
				Release: v2beta1.HelmReleaseSpec{
					ReleaseName: "originalReleaseName",
				},
			},
		}

		result, err := applyTargetOverrides([]*provisioningv1.HelmRelease{&hr}, newTenant(tenantName, "platform", "domain"), nil)
		if err != nil {
			t.Error(err)
		}

		assert.Len(t, result, 1)
		assert.Equal(t, "overriddenReleaseName", result[0].Spec.Release.ReleaseName)
	})

}

func newTenant(name, platform string, domains ...string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Description: name + " description",
			DomainRefs:  domains,
		},
	}
}

func newTenantCategory(name, platform, patchKind, patchName string, overridesBytes []byte) *platformv1.TenantCategory {
	category := &platformv1.TenantCategory{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantCategorySpec{
			PlatformRef: platform,
		},
	}
	if overridesBytes != nil {
		category.Spec.ProvisioningOverrides = []platformv1.ProvisioningResourcePatch{
			{
				Target: platformv1.ProvisioningResourcePatchTarget{
					Kind:       patchKind,
					APIVersion: provisioningv1.SchemeGroupVersion.String(),
					Name:       patchName,
					Namespace:  metav1.NamespaceDefault,
				},
				Spec: &v1.JSON{Raw: overridesBytes},
			},
		}
	}
	return category
}

func newAzureDb(name, platform, domain string) *provisioningv1.AzureDatabase {
	return &provisioningv1.AzureDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureDatabaseSpec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
				DomainRef:   domain,
				Target: provisioningv1.ProvisioningTarget{
					Category: provisioningv1.ProvisioningTargetCategoryTenant,
				},
			},
			DbName: name,
		},
	}
}

func newAzureManagedDb(dbName, platform string, domain string) *provisioningv1.AzureManagedDatabase {
	return &provisioningv1.AzureManagedDatabase{
		TypeMeta: metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureManagedDatabaseSpec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
				DomainRef:   domain,
				Target: provisioningv1.ProvisioningTarget{
					Category: provisioningv1.ProvisioningTargetCategoryTenant,
				},
			},
			DbName: dbName,
		},
	}
}

func runController(objects []runtime.Object, provisioner CreateInfrastructureFunc, msgPublisher messaging.MessagingPublisher) *ProvisioningController {
	clientset := fakeClientset.NewSimpleClientset(objects...)

	c := NewProvisioningController(clientset, provisioner, nil, nil, msgPublisher)
	c.factory.Start(nil)
	c.factory.WaitForCacheSync(nil)

	return c
}

func runControllerWithDefaultFakes(objects []runtime.Object) (*ProvisioningController, *[]provisionerResult, chan messagingMock.RcvMsg) {
	var outputs []provisionerResult

	infraCreator := func(target ProvisioningTarget, domain string, infra *InfrastructureManifests) ProvisioningResult {
		outputs = append(outputs, provisionerResult{target.GetPlatformName(), target, domain, infra})
		return ProvisioningResult{}
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
	target   ProvisioningTarget
	domain   string
	infra    *InfrastructureManifests
}
