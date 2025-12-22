package platform

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
	messaging "totalsoft.ro/platform-controllers/internal/messaging/mock"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
)

func TestPlatformController_processNextWorkItem(t *testing.T) {

	t.Run("one platform with two tenants", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		tenant1 := _newTenant("tenant1", "charismaonline.qa", []string{})
		tenant2 := _newTenant("tenant2", "charismaonline.qa", []string{})

		c, msgChan := _runController([]runtime.Object{platform, tenant1, tenant2})
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
			"MultiTenancy__Tenants__tenant1__TenantId": tenant1.Spec.Id,
			"MultiTenancy__Tenants__tenant1__Enabled":  "true",
			"MultiTenancy__Tenants__tenant2__TenantId": tenant2.Spec.Id,
			"MultiTenancy__Tenants__tenant2__Enabled":  "true",
		}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			TenantCreatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("one platform with two tenants and one domain", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		domain := _newDomain("qa-r7d", "origination", platform.Name)
		tenant1 := _newTenant("tenant1", platform.Name, []string{domain.Name})
		tenant2 := _newTenant("tenant2", platform.Name, []string{})

		c, msgChan := _runController([]runtime.Object{platform, domain, tenant1, tenant2})
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

		output, err := c.kubeClientset.CoreV1().ConfigMaps(domain.Namespace).Get(context.TODO(), "origination-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		expectedOutput := map[string]string{
			"MultiTenancy__Tenants__tenant1__Enabled": "true",
			"MultiTenancy__Tenants__tenant2__Enabled": "false",
		}
		if !reflect.DeepEqual(output.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", output.Data)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			TenantCreatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("orphan tenants", func(t *testing.T) {
		// Arrange
		tenant1 := _newTenant("tenant1", "charismaonline.qa", []string{})
		tenant2 := _newTenant("tenant2", "charismaonline.qa", []string{})

		c, _ := _runController([]runtime.Object{tenant1, tenant2})
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

		c, msgChan := _runController([]runtime.Object{platform})
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

		msg := <-msgChan
		if msg.Topic != SyncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", SyncedSuccessfullyTopic, ", got", msg.Topic)
		}
	})

	t.Run("tenant platformRef updated", func(t *testing.T) {
		// Arrange
		platformQa := _newPlatform("qa", "charismaonline.qa")
		platformUat := _newPlatform("uat", "charismaonline.uat")
		tenant1 := _newTenant("tenant1", "charismaonline.qa", []string{})

		c, msgChan := _runController([]runtime.Object{platformQa, platformUat, tenant1})
		if c.workqueue.Len() != 2 {
			t.Error("queue should have 2 items, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		t1, err := c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Get(context.TODO(), "tenant1", metav1.GetOptions{})
		if err != nil {
			t.Error("tenant not found")
		}
		t1 = t1.DeepCopy()
		t1.Spec.PlatformRef = "charismaonline.uat"
		c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Update(context.TODO(), t1, metav1.UpdateOptions{})
		//c.tenantInformer.Informer().GetIndexer().Update(t1) //fix stale cache
		time.Sleep(100 * time.Millisecond) //additional fix stale cache
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

		qaMsg := <-msgChan
		if qaMsg.Topic != SyncedSuccessfullyTopic {
			t.Error("expected message pblished to topic ", SyncedSuccessfullyTopic, ", got", qaMsg.Topic)
		}

		uatConfigMap, err := c.kubeClientset.CoreV1().ConfigMaps("uat").Get(context.TODO(), "charismaonline.uat-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		if expectedOutput := map[string]string{
			"MultiTenancy__Tenants__tenant1__TenantId": tenant1.Spec.Id,
			"MultiTenancy__Tenants__tenant1__Enabled":  "true",
		}; !reflect.DeepEqual(uatConfigMap.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", uatConfigMap.Data)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			TenantUpdatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("tenant deleted", func(t *testing.T) {
		// Arrange
		platformQa := _newPlatform("qa", "charismaonline.qa")
		tenant1 := _newTenant("tenant1", "charismaonline.qa", []string{})

		c, msgChan := _runController([]runtime.Object{platformQa, tenant1})
		if c.workqueue.Len() != 1 {
			t.Error("queue should have 1 item, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		// t1, err := c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Get(context.TODO(), "tenant1", metav1.GetOptions{})
		// if err != nil {
		// 	t.Error(err)
		// }
		err := c.platformClientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).Delete(context.TODO(), "tenant1", metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
		//c.platformInformer.Informer().GetIndexer().Delete(t1) //fix stale cache
		time.Sleep(10 * time.Millisecond) //fix stale cache

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}
		time.Sleep(10 * time.Millisecond) //fix stale cache
		configMap, err := c.kubeClientset.CoreV1().ConfigMaps("qa").Get(context.TODO(), "charismaonline.qa-tenants", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		if expectedOutput := map[string]string{}; !reflect.DeepEqual(configMap.Data, expectedOutput) {
			t.Error("expected output config ", expectedOutput, ", got", configMap.Data)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			TenantDeletedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})
	t.Run("tenant specific configs", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		tenant := _newTenant("tenant1", platform.Name, []string{})
		tenant.Spec.Configs = map[string]string{
			"config1": "value1",
			"config2": "value2",
		}

		expectedTenantData := map[string]string{
			"MultiTenancy__Tenants__tenant1__config1": "value1",
			"MultiTenancy__Tenants__tenant1__config2": "value2",
		}

		c, _ := _runController([]runtime.Object{platform, tenant})

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

		for key, value := range expectedTenantData {
			if expectedValue, ok := output.Data[key]; ok {
				if value != expectedValue {
					t.Errorf("Expected value for key %v: %v, got: %v", key, expectedValue, value)
				}
			} else {
				t.Errorf("Key %v not found in output configmap.", key)
			}
		}
	})

	t.Run("domain created", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		domain := _newDomain("qa", "origination", platform.Name)

		c, msgChan := _runController([]runtime.Object{platform, domain})
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

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			DomainCreatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("domain platformRef updated", func(t *testing.T) {
		// Arrange
		platformQa := _newPlatform("qa", "charismaonline.qa")
		platformUat := _newPlatform("uat", "charismaonline.uat")
		domain := _newDomain("qa", "origination", platformQa.Name)

		c, msgChan := _runController([]runtime.Object{platformQa, platformUat, domain})
		if c.workqueue.Len() != 2 {
			t.Error("queue should have 2 items, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		d, err := c.platformClientset.PlatformV1alpha1().Domains("qa").Get(context.TODO(), "origination", metav1.GetOptions{})
		if err != nil {
			t.Error("domain not found")
		}
		d = d.DeepCopy()
		d.Spec.PlatformRef = "charismaonline.uat"
		c.platformClientset.PlatformV1alpha1().Domains("qa").Update(context.TODO(), d, metav1.UpdateOptions{})
		time.Sleep(10 * time.Millisecond) //additional fix stale cache
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

		dUpdated, err := c.platformClientset.PlatformV1alpha1().Domains("qa").Get(context.TODO(), "origination", metav1.GetOptions{})
		if err != nil {
			t.Error("domain not found")
		}

		if dUpdated.Spec.PlatformRef != "charismaonline.uat" {
			t.Error("expected platformRef to be charismaonline.uat, got", dUpdated.Spec.PlatformRef)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			DomainUpdatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("domain deleted", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		domain := _newDomain("qa", "origination", platform.Name)

		c, msgChan := _runController([]runtime.Object{platform, domain})
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		err := c.platformClientset.PlatformV1alpha1().Domains("qa").Delete(context.TODO(), "origination", metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
		time.Sleep(10 * time.Millisecond) //fix stale cache

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:        false,
			DomainDeletedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("service created", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		service := _newService("qa", "service1", platform.Name)

		c, msgChan := _runController([]runtime.Object{platform, service})
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

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:         false,
			ServiceCreatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("service platformRef updated", func(t *testing.T) {
		// Arrange
		platformQa := _newPlatform("qa", "charismaonline.qa")
		platformUat := _newPlatform("uat", "charismaonline.uat")
		service := _newService("qa", "service1", platformQa.Name)

		c, msgChan := _runController([]runtime.Object{platformQa, platformUat, service})
		if c.workqueue.Len() != 2 {
			t.Error("queue should have 2 items, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}
		<-msgChan

		s, err := c.platformClientset.PlatformV1alpha1().Services("qa").Get(context.TODO(), "service1", metav1.GetOptions{})
		if err != nil {
			t.Error("domain not found")
		}
		s = s.DeepCopy()
		s.Spec.PlatformRef = "charismaonline.uat"
		c.platformClientset.PlatformV1alpha1().Services("qa").Update(context.TODO(), s, metav1.UpdateOptions{})
		time.Sleep(10 * time.Millisecond) //additional fix stale cache
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

		sUpdated, err := c.platformClientset.PlatformV1alpha1().Services("qa").Get(context.TODO(), "service1", metav1.GetOptions{})
		if err != nil {
			t.Error("service not found")
		}

		if sUpdated.Spec.PlatformRef != "charismaonline.uat" {
			t.Error("expected platformRef to be charismaonline.uat, got", sUpdated.Spec.PlatformRef)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:         false,
			ServiceUpdatedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})

	t.Run("service deleted", func(t *testing.T) {
		// Arrange
		platform := _newPlatform("qa", "charismaonline.qa")
		service := _newService("qa", "service1", platform.Name)

		c, msgChan := _runController([]runtime.Object{platform, service})
		if c.workqueue.Len() != 1 {
			t.Error("queue should have only 1 item, but it has", c.workqueue.Len())
		}

		// Act
		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		err := c.platformClientset.PlatformV1alpha1().Services("qa").Delete(context.TODO(), "service1", metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
		time.Sleep(10 * time.Millisecond) //fix stale cache

		if result := c.processNextWorkItem(); !result {
			t.Error("processing failed")
		}

		// Assert
		if c.workqueue.Len() != 0 {
			item, _ := c.workqueue.Get()
			t.Error("queue should be empty, but contains ", item)
		}

		// Collect messages with a timeout
		var receivedMsgs []messaging.RcvMsg
		timeout := time.After(1 * time.Second)
		done := false
		for !done {
			select {
			case msg := <-msgChan:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timeout:
				done = true
			}
		}

		// Expect messages to be published to the following topics
		expectedTopics := map[string]bool{
			SyncedSuccessfullyTopic:         false,
			ServiceDeletedSuccessfullyTopic: false,
		}

		// Mark topics as received
		for _, msg := range receivedMsgs {
			if _, exists := expectedTopics[msg.Topic]; exists {
				expectedTopics[msg.Topic] = true
			}
		}

		// Validate all expected topics were found
		for topic, found := range expectedTopics {
			if !found {
				t.Errorf("expected message with topic %s was not received", topic)
			}
		}
	})
}

func _newPlatform(ns, name string) *platformv1.Platform {
	return &platformv1.Platform{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: platformv1.PlatformSpec{
			TargetNamespace: ns,
		},
	}
}

func _newTenant(name, platform string, domains []string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Description: name + " description",
			Id:          uuid.New().String(),
			Enabled:     true,
			DomainRefs:  domains,
		},
	}
}

func _newDomain(ns, name, platform string) *platformv1.Domain {
	return &platformv1.Domain{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: platformv1.DomainSpec{
			PlatformRef: platform,
		},
	}
}

func _newService(ns, name, platform string /* requiredDomainRefs []string, optionalDomainRefs []string */) *platformv1.Service {
	return &platformv1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: platformv1.ServiceSpec{
			PlatformRef: platform,
			/* RequiredDomainRefs: requiredDomainRefs,
			OptionalDomainRefs: optionalDomainRefs, */
		},
	}
}

func _runController(objects []runtime.Object) (*PlatformController, chan messaging.RcvMsg) {
	kubeClient := kubeFakeClientSet.NewSimpleClientset()
	platformClient := fakeClientset.NewSimpleClientset(objects...)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	msgChan := make(chan messaging.RcvMsg)
	msgPublisher := messaging.MessagingPublisherMock(msgChan)

	c := NewPlatformController(kubeClient, platformClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		platformInformerFactory.Platform().V1alpha1().Platforms(),
		platformInformerFactory.Platform().V1alpha1().Tenants(),
		platformInformerFactory.Platform().V1alpha1().Domains(),
		platformInformerFactory.Platform().V1alpha1().Services(),
		nil, msgPublisher)
	kubeInformerFactory.Start(nil)
	platformInformerFactory.Start(nil)

	kubeInformerFactory.WaitForCacheSync(nil)
	platformInformerFactory.WaitForCacheSync(nil)

	return c, msgChan
}
