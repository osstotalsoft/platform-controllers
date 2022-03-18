package controllers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"strings"
	"time"
	"totalsoft.ro/platform-controllers/internal/provisioners"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
	informersv1 "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/provisioning/v1alpha1"
)

const (
	controllerAgentName = "provisioning-controller"
	// SuccessSynced is used as part of the Event 'reason' when a Resource is synced
	SuccessSynced = "Synced successfully"
	ErrorSynced   = "Error"
)

// ProvisioningController is the controller implementation for Tenant resources
type ProvisioningController struct {
	factory   informers.SharedInformerFactory
	clientset clientset.Interface
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	infraCreator provisioners.CreateInfrastructureFunc

	platformInformer informersv1.PlatformInformer
	infraInformer    informersv1.AzureDatabaseInformer
	tenantInformer   informersv1.TenantInformer
}

func NewProvisioningController(clientSet clientset.Interface,
	infraCreator provisioners.CreateInfrastructureFunc,
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

		platformInformer: factory.Provisioning().V1alpha1().Platforms(),
		infraInformer:    factory.Provisioning().V1alpha1().AzureDatabases(),
		tenantInformer:   factory.Provisioning().V1alpha1().Tenants(),

		infraCreator: infraCreator,
		clientset:    clientSet,
	}

	if eventBroadcaster != nil {
		c.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	}

	addTenantHandlers(c.tenantInformer, c.enqueueTenant)
	//addPlatformHandlers(c.platformInformer)
	addInfraHandlers(c.infraInformer, c.enqueueAllTenant)

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
	tenant, err := c.tenantInformer.Lister().Tenants(namespace).Get(name)
	if err != nil {
		// The tenant resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("tenant '%s' from work queue no longer exists", key))
			return nil
		}
		return err
	}

	azureDbs, err := c.infraInformer.Lister().List(labels.Everything())
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

	err = c.infraCreator(platform, tenant, azureDbs)
	if err == nil {
		c.recorder.Event(tenant, corev1.EventTypeNormal, SuccessSynced, SuccessSynced)
	} else {
		c.recorder.Event(tenant, corev1.EventTypeWarning, ErrorSynced, err.Error())
	}
	c.updateTenantStatus(tenant, err)

	return err
}

func (c *ProvisioningController) updateTenantStatus(tenant *provisioningv1.Tenant, err error) {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	tenantCopy := tenant.DeepCopy()
	tenantCopy.Status.LastResyncTime = metav1.Now()
	tenantCopy.Status.State = SuccessSynced
	if err != nil {
		tenantCopy.Status.State = ErrorSynced
	}

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err = c.clientset.ProvisioningV1alpha1().Tenants(tenant.Namespace).UpdateStatus(context.TODO(), tenantCopy, metav1.UpdateOptions{})
	if err != nil {
		utilruntime.HandleError(err)
	}
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

func (c *ProvisioningController) enqueueTenant(tenant *provisioningv1.Tenant) {
	var tenantKey string
	var err error

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

func addTenantHandlers(informer informersv1.TenantInformer, handler func(*provisioningv1.Tenant)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.Tenant)
			klog.V(4).InfoS("tenant added", "name", comp.Name, "namespace", comp.Namespace)
			handler(comp)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldT := oldObj.(*provisioningv1.Tenant)
			newT := newObj.(*provisioningv1.Tenant)
			klog.V(4).InfoS("tenant updated", "name", newT.Name, "namespace", newT.Namespace)
			if oldT.Spec != newT.Spec {
				handler(newT)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.Tenant)
			klog.V(4).InfoS("tenant deleted", "name", comp.Name, "namespace", comp.Namespace)
		},
	})
}

func addPlatformHandlers(informer informersv1.PlatformInformer) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.Platform)
			klog.V(4).InfoS("platform added", "name", comp.Name, "namespace", comp.Namespace)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			comp := newObj.(*provisioningv1.Platform)
			klog.V(4).InfoS("platform updated - do nothing", "name", comp.Name, "namespace", comp.Namespace)
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.Platform)
			klog.V(4).InfoS("platform deleted  - do nothing", "name", comp.Name, "namespace", comp.Namespace)
		},
	})
}

func addInfraHandlers(informer informersv1.AzureDatabaseInformer, handler func(platform string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureDatabase)
			klog.V(4).InfoS("Azure database added", "name", comp.Name, "namespace", comp.Namespace)
			handler(comp.Spec.PlatformRef)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			comp := newObj.(*provisioningv1.AzureDatabase)
			klog.V(4).InfoS("Azure database updated", "name", comp.Name, "namespace", comp.Namespace)
			handler(comp.Spec.PlatformRef)
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*provisioningv1.AzureDatabase)
			klog.V(4).InfoS("Azure database deleted", "name", comp.Name, "namespace", comp.Namespace)
			handler(comp.Spec.PlatformRef)
		},
	})
}
