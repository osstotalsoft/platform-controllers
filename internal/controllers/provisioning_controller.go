package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
	platformInformersv1 "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/platform/v1alpha1"
	provisioningInformersv1 "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/provisioning/v1alpha1"
)

const (
	controllerAgentName = "provisioning-controller"
	// SuccessSynced is used as part of the Event 'reason' when a Resource is synced
	SuccessSynced = "Synced successfully"
	ErrorSynced   = "Error"

	SkipTenantLabelFormat = "provisioning.totalsoft.ro/skip-tenant-%s"
	SkipProvisioningLabel = "provisioning.totalsoft.ro/skip-provisioning"
)

// ProvisioningController is the controller implementation for Tenant resources
type ProvisioningController struct {
	factory   informers.SharedInformerFactory
	clientset clientset.Interface
	migrator  func(platform string, tenant *platformv1.Tenant) error

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	provisioner provisioners.CreateInfrastructureFunc

	platformInformer       platformInformersv1.PlatformInformer
	tenantInformer         platformInformersv1.TenantInformer
	azureDbInformer        provisioningInformersv1.AzureDatabaseInformer
	azureManagedDbInformer provisioningInformersv1.AzureManagedDatabaseInformer
}

func NewProvisioningController(clientSet clientset.Interface,
	provisioner provisioners.CreateInfrastructureFunc,
	migrator func(platform string, tenant *platformv1.Tenant) error,
	eventBroadcaster record.EventBroadcaster) *ProvisioningController {

	factory := informers.NewSharedInformerFactory(clientSet, 0)

	// Create event broadcaster
	// Add provisioning-controller types to the default Kubernetes Scheme so Events can be
	// logged for provisioning-controller types.
	utilruntime.Must(clientsetScheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")

	c := &ProvisioningController{
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "provisioning"),
		recorder:  &record.FakeRecorder{},
		factory:   factory,

		platformInformer:       factory.Platform().V1alpha1().Platforms(),
		tenantInformer:         factory.Platform().V1alpha1().Tenants(),
		azureDbInformer:        factory.Provisioning().V1alpha1().AzureDatabases(),
		azureManagedDbInformer: factory.Provisioning().V1alpha1().AzureManagedDatabases(),

		provisioner: provisioner,
		clientset:   clientSet,
		migrator:    migrator,
	}

	if eventBroadcaster != nil {
		c.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	}

	addTenantHandlers(c.tenantInformer, c.enqueueTenant)
	addPlatformHandlers(c.platformInformer)
	addAzureDbHandlers(c.azureDbInformer, c.enqueueAllTenant)
	addAzureManagedDbHandlers(c.azureManagedDbInformer, c.enqueueAllTenant)

	return c
}

func (c *ProvisioningController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.factory.Start(stopCh)

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Provisioner controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	c.factory.WaitForCacheSync(stopCh)

	klog.Info("Starting workers")

	// Launch two workers to process resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker(i), time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *ProvisioningController) runWorker(i int) func() {
	return func() {
		for c.processNextWorkItem(i) {
		}
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ProvisioningController) processNextWorkItem(i int) bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		klog.V(4).Info("shutdown requested closing worker")
		return false
	}

	klog.V(4).InfoS("dequeue item", "worker", i, "key", obj)

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		key, _ := obj.(string)
		// Run the syncHandler, passing it the namespace/name string of the resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Tenant resource
// with the current status of the resource.
func (c *ProvisioningController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	platform, tenantKey, _ := decodeKey(key)
	namespace, name, err := cache.SplitMetaNamespaceKey(tenantKey)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid tenant key: %s", tenantKey))
		return nil
	}

	// Get the Tenant resource with this namespace/name
	// use the live query API, to get the latest version instead of listers which are cached
	tenant, err := c.clientset.PlatformV1alpha1().Tenants(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if shouldCleanupTenantResources := (err != nil && errors.IsNotFound(err)) || (err == nil && tenant.Spec.PlatformRef != platform); shouldCleanupTenantResources {
		cleanupResult := c.provisioner(platform, &platformv1.Tenant{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       platformv1.TenantSpec{PlatformRef: platform}},
			&provisioners.InfrastructureManifests{
				AzureDbs:        []*provisioningv1.AzureDatabase{},
				AzureManagedDbs: []*provisioningv1.AzureManagedDatabase{},
			})
		if cleanupResult.Error != nil {
			utilruntime.HandleError(cleanupResult.Error)
		}

		return nil
	}
	if err != nil {
		return err
	}

	skipTenantLabel := fmt.Sprintf(SkipTenantLabelFormat, tenant.Name)
	skipTenantLabelSelector, err := labels.Parse(fmt.Sprintf("%s!=true", skipTenantLabel))
	if err != nil {
		return err
	}

	azureDbs, err := c.azureDbInformer.Lister().List(skipTenantLabelSelector)
	if err != nil {
		return err
	}

	n := 0
	for _, db := range azureDbs {
		if db.Spec.PlatformRef == platform {
			azureDbs[n] = db
			n++
		}
	}
	azureDbs = azureDbs[:n]

	azureManagedDbs, err := c.azureManagedDbInformer.Lister().List(skipTenantLabelSelector)
	if err != nil {
		return err
	}

	n = 0
	for _, db := range azureManagedDbs {
		if db.Spec.PlatformRef == platform {
			azureManagedDbs[n] = db
			n++
		}
	}
	azureManagedDbs = azureManagedDbs[:n]

	result := c.provisioner(platform, tenant, &provisioners.InfrastructureManifests{
		AzureDbs:        azureDbs,
		AzureManagedDbs: azureManagedDbs,
	})

	if result.Error == nil {
		if c.migrator != nil && (result.HasAzureDbChanges || result.HasAzureManagedDbChanges) {
			p, err := c.clientset.PlatformV1alpha1().Platforms().Get(context.TODO(), platform, metav1.GetOptions{})
			if err == nil {
				result.Error = c.migrator(p.Spec.TargetNamespace, tenant)
			} else {
				klog.ErrorS(err, "platform not found")
			}
		}
	}

	if result.Error == nil {
		c.recorder.Event(tenant, corev1.EventTypeNormal, SuccessSynced, SuccessSynced)
	} else {
		c.recorder.Event(tenant, corev1.EventTypeWarning, ErrorSynced, result.Error.Error())
	}
	_, e := c.updateTenantStatus(tenant, result.Error)
	if e != nil {
		//just log this error, don't propagate
		utilruntime.HandleError(e)
	}

	return result.Error
}

func (c *ProvisioningController) updateTenantStatus(tenant *platformv1.Tenant, err error) (*platformv1.Tenant, error) {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	tenantCopy := tenant.DeepCopy()
	tenantCopy.Status.LastResyncTime = metav1.Now()

	if err != nil {
		apimeta.SetStatusCondition(&tenantCopy.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  FailedReason,
			Message: err.Error(),
		})
	} else {
		apimeta.SetStatusCondition(&tenantCopy.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  SucceededReason,
			Message: SuccessSynced,
		})
	}

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	return c.clientset.PlatformV1alpha1().Tenants(tenant.Namespace).UpdateStatus(context.TODO(), tenantCopy, metav1.UpdateOptions{})
}

func (c *ProvisioningController) enqueueAllTenant(platform string) {
	tenants, err := c.tenantInformer.Lister().List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, tenant := range tenants {
		if tenant.Spec.PlatformRef == platform {
			c.enqueueTenant(tenant)
		}
	}
}

func (c *ProvisioningController) enqueueTenant(tenant *platformv1.Tenant) {
	var tenantKey string
	var err error

	if v, ok := tenant.Labels[SkipProvisioningLabel]; ok && v == "true" {
		return
	}

	if tenantKey, err = cache.MetaNamespaceKeyFunc(tenant); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(encodeKey(tenant.Spec.PlatformRef, tenantKey))
}

func encodeKey(platformKey, tenantKey string) (key string) {
	return fmt.Sprintf("%s::%s", platformKey, tenantKey)
}

func decodeKey(key string) (platformKey, tenantKey string, err error) {
	res := strings.Split(key, "::")
	if len(res) == 2 {
		return res[0], res[1], nil
	}
	return "", "", fmt.Errorf("cannot decode key: %v", key)
}

func addPlatformHandlers(informer platformInformersv1.PlatformInformer) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Platform)
			klog.V(4).InfoS("Platform added", "name", comp.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newT := newObj.(*platformv1.Platform)
			klog.V(4).InfoS("Platform updated", "name", newT.Name)
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Platform)
			klog.V(4).InfoS("Platform deleted", "name", comp.Name)
		},
	})
}

func addTenantHandlers(informer platformInformersv1.TenantInformer, handler func(*platformv1.Tenant)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Tenant)
			if platform, ok := getTenantPlatform(comp); ok {
				klog.V(4).InfoS("tenant added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(comp)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldT := oldObj.(*platformv1.Tenant)
			newT := newObj.(*platformv1.Tenant)
			oldPlatform, oldOk := getTenantPlatform(oldT)
			newPlatform, newOk := getTenantPlatform(newT)
			platformChanged := oldPlatform != newPlatform
			specChanged := oldT.Spec != newT.Spec
			if oldOk && platformChanged {
				klog.V(4).InfoS("Tenant invalidated", "name", oldT.Name, "namespace", oldT.Namespace, "platform", oldPlatform)
				handler(oldT)
			}

			if newOk && specChanged {
				klog.V(4).InfoS("Tenant updated", "name", newT.Name, "namespace", newT.Namespace, "platform", newPlatform)
				handler(newT)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Tenant)
			if platform, ok := getTenantPlatform(comp); ok {
				klog.V(4).InfoS("tenant deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(comp)
			}
		},
	})
}

func addAzureDbHandlers(informer provisioningInformersv1.AzureDatabaseInformer, handler func(platform string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureDatabase)
			if platform, ok := getAzureDbPlatform(comp); ok {
				klog.V(4).InfoS("Azure database added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(platform)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*provisioningv1.AzureDatabase)
			newComp := newObj.(*provisioningv1.AzureDatabase)
			oldPlatform, oldOk := getAzureDbPlatform(oldComp)
			newPlatform, newOk := getAzureDbPlatform(newComp)
			platformChanged := oldPlatform != newPlatform

			if oldOk && platformChanged {
				klog.V(4).InfoS("Azure database invalidated", "name", oldComp.Name, "namespace", oldComp.Namespace, "platform", oldPlatform)
				handler(oldPlatform)
			}

			if newOk {
				klog.V(4).InfoS("Azure database updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", newPlatform)
				handler(newPlatform)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureDatabase)
			if platform, ok := getAzureDbPlatform(comp); ok {
				klog.V(4).InfoS("Azure database deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(platform)
			}
		},
	})
}

func addAzureManagedDbHandlers(informer provisioningInformersv1.AzureManagedDatabaseInformer, handler func(platform string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureManagedDatabase)
			if platform, ok := getAzureManagedDbPlatform(comp); ok {
				klog.V(4).InfoS("Azure managed database added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(platform)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*provisioningv1.AzureManagedDatabase)
			newComp := newObj.(*provisioningv1.AzureManagedDatabase)
			oldPlatform, oldOk := getAzureManagedDbPlatform(oldComp)
			newPlatform, newOk := getAzureManagedDbPlatform(newComp)
			platformChanged := oldPlatform != newPlatform

			if oldOk && platformChanged {
				klog.V(4).InfoS("Azure managed database invalidated", "name", oldComp.Name, "namespace", oldComp.Namespace, "platform", oldPlatform)
				handler(oldPlatform)
			}

			if newOk {
				klog.V(4).InfoS("Azure managed database updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", newPlatform)
				handler(newPlatform)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureManagedDatabase)
			if platform, ok := getAzureManagedDbPlatform(comp); ok {
				klog.V(4).InfoS("Azure managed database deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform)
				handler(platform)
			}
		},
	})
}

func getTenantPlatform(tenant *platformv1.Tenant) (platform string, ok bool) {
	platform = tenant.Spec.PlatformRef
	if len(platform) == 0 {
		return platform, false
	}

	return platform, true
}

func getAzureDbPlatform(azureDb *provisioningv1.AzureDatabase) (platform string, ok bool) {
	platform = azureDb.Spec.PlatformRef
	if len(platform) == 0 {
		return platform, false
	}

	return platform, true
}

func getAzureManagedDbPlatform(azureManagedDb *provisioningv1.AzureManagedDatabase) (platform string, ok bool) {
	platform = azureManagedDb.Spec.PlatformRef
	if len(platform) == 0 {
		return platform, false
	}

	return platform, true
}
