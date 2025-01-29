package platform

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	controllers "totalsoft.ro/platform-controllers/internal/controllers"
	messaging "totalsoft.ro/platform-controllers/internal/messaging"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/platform/v1alpha1"
	listers "totalsoft.ro/platform-controllers/pkg/generated/listers/platform/v1alpha1"
)

const (
	platformControllerAgentName = "platform-controller"
	ErrResourceExists           = "ErrResourceExists"

	// ReadyCondition indicates the resource is ready and fully reconciled.
	// If the Condition is False, the resource SHOULD be considered to be in the process of reconciling and not a
	// representation of actual state.
	ReadyCondition = "Ready"
)

type PlatformController struct {
	kubeClientset     kubernetes.Interface
	platformClientset clientset.Interface
	configMapsLister  coreListers.ConfigMapLister
	configMapsSynced  cache.InformerSynced
	platformInformer  informers.PlatformInformer
	platformsLister   listers.PlatformLister
	platformsSynced   cache.InformerSynced
	tenantInformer    informers.TenantInformer
	tenantsLister     listers.TenantLister
	tenantsSynced     cache.InformerSynced
	domainInformer    informers.DomainInformer
	domainsLister     listers.DomainLister
	domainsSynced     cache.InformerSynced
	serviceInformer   informers.ServiceInformer
	servicesLister    listers.ServiceLister
	servicesSynced    cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.

	recorder record.EventRecorder
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	messagingPublisher messaging.MessagingPublisher
}

func NewPlatformController(
	kubeClientset kubernetes.Interface,
	platformClientset clientset.Interface,
	configMapInformer coreInformers.ConfigMapInformer,
	platformInformer informers.PlatformInformer,
	tenantInformer informers.TenantInformer,
	domainInformer informers.DomainInformer,
	serviceInformer informers.ServiceInformer,
	eventBroadcaster record.EventBroadcaster,
	messagingPublisher messaging.MessagingPublisher,
) *PlatformController {
	controller := &PlatformController{
		kubeClientset:     kubeClientset,
		platformClientset: platformClientset,
		configMapsLister:  configMapInformer.Lister(),
		configMapsSynced:  configMapInformer.Informer().HasSynced,
		platformInformer:  platformInformer,
		platformsLister:   platformInformer.Lister(),
		platformsSynced:   platformInformer.Informer().HasSynced,
		tenantInformer:    tenantInformer,
		tenantsLister:     tenantInformer.Lister(),
		tenantsSynced:     tenantInformer.Informer().HasSynced,
		domainInformer:    domainInformer,
		domainsLister:     domainInformer.Lister(),
		domainsSynced:     domainInformer.Informer().HasSynced,
		serviceInformer:   serviceInformer,
		servicesLister:    serviceInformer.Lister(),
		servicesSynced:    serviceInformer.Informer().HasSynced,

		recorder:           &record.FakeRecorder{},
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "platform"),
		messagingPublisher: messagingPublisher,
	}

	utilruntime.Must(clientsetScheme.AddToScheme(scheme.Scheme))
	if eventBroadcaster != nil {
		controller.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: platformControllerAgentName})
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when Tenant resources change
	tenantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			tenant := obj.(*platformv1.Tenant)
			klog.V(4).InfoS("tenant added", "name", tenant.Name, "namespace", tenant.Namespace)
			controller.enqueuePlatformByTenant(tenant)

			controller.recorder.Event(tenant, corev1.EventTypeNormal, "Tenant created successfully", "Tenant created successfully")
			event := TenantCreated{
				TenantId:   tenant.Spec.Id,
				TenantName: tenant.Name,
			}
			err := controller.messagingPublisher(context.TODO(), TenantCreatedSuccessfullyTopic, event, tenant.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.TenantCreatedSuccessfully event")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldT := oldObj.(*platformv1.Tenant)
			newT := newObj.(*platformv1.Tenant)
			specChanged := !reflect.DeepEqual(oldT.Spec, newT.Spec)
			if specChanged {
				klog.V(4).InfoS("tenant updated", "name", newT.Name, "namespace", newT.Namespace)
				controller.enqueuePlatformByTenant(newT)

				if platformChanged := oldT.Spec.PlatformRef != newT.Spec.PlatformRef; platformChanged {
					controller.enqueuePlatformByTenant(oldT)
				}

				controller.recorder.Event(newT, corev1.EventTypeNormal, "Tenant updated successfully", "Tenant updated successfully")
				event := TenantUpdated{
					TenantId:     newT.Spec.Id,
					TenantName:   newT.Name,
					PlatformRef:  newT.Spec.PlatformRef,
					Enabled:      newT.Spec.Enabled,
					DomainRefs:   newT.Spec.DomainRefs,
					AdminEmail:   newT.Spec.AdminEmail,
					DeletePolicy: newT.Spec.DeletePolicy,
					Configs:      newT.Spec.Configs,
				}
				err := controller.messagingPublisher(context.TODO(), TenantUpdatedSuccessfullyTopic, event, newT.Spec.PlatformRef)
				if err != nil {
					klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.TenantUpdatedSuccessfully event")
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			tenant := obj.(*platformv1.Tenant)
			klog.V(4).InfoS("tenant deleted", "name", tenant.Name, "namespace", tenant.Namespace)
			controller.enqueuePlatformByTenant(tenant)

			controller.recorder.Event(tenant, corev1.EventTypeNormal, "Tenant deleted successfully", "Tenant deleted successfully")
			event := TenantDeleted{
				TenantId: tenant.Spec.Id,
			}
			err := controller.messagingPublisher(context.TODO(), TenantDeletedSuccessfullyTopic, event, tenant.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.TenantDeletedSuccessfully event")
			}
		},
	})

	// Set up an event handler for when Domain resources change
	domainInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			domain := obj.(*platformv1.Domain)
			klog.V(4).InfoS("domain added", "name", domain.Name, "namespace", domain.Namespace)
			controller.enqueuePlatformByDomain(domain)

			controller.recorder.Event(domain, corev1.EventTypeNormal, "Domain created successfully", "Domain created successfully")
			event := DomainCreated{
				DomainName:  domain.Name,
				Namespace:   domain.Namespace,
				PlatformRef: domain.Spec.PlatformRef,
			}
			err := controller.messagingPublisher(context.TODO(), DomainCreatedSuccessfullyTopic, event, domain.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.DomainCreatedSuccessfully event")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldD := oldObj.(*platformv1.Domain)
			newD := newObj.(*platformv1.Domain)
			specChanged := !reflect.DeepEqual(oldD.Spec, newD.Spec)
			if specChanged {
				klog.V(4).InfoS("domain updated", "name", newD.Name, "namespace", newD.Namespace)
				controller.enqueuePlatformByDomain(newD)

				if platformChanged := oldD.Spec.PlatformRef != newD.Spec.PlatformRef; platformChanged {
					controller.enqueuePlatformByDomain(oldD)
				}

				controller.recorder.Event(newD, corev1.EventTypeNormal, "Domain updated successfully", "Domain updated successfully")
				event := DomainUpdated{
					oldValue: Domain{
						DomainName:          oldD.Name,
						Namespace:           oldD.Namespace,
						PlatformRef:         oldD.Spec.PlatformRef,
						ExportActiveDomains: oldD.Spec.ExportActiveDomains,
					},
					newValue: Domain{
						DomainName:          newD.Name,
						Namespace:           newD.Namespace,
						PlatformRef:         newD.Spec.PlatformRef,
						ExportActiveDomains: newD.Spec.ExportActiveDomains,
					},
				}
				err := controller.messagingPublisher(context.TODO(), DomainUpdatedSuccessfullyTopic, event, newD.Spec.PlatformRef)
				if err != nil {
					klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.DomainUpdatedSuccessfully event")
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			domain := obj.(*platformv1.Domain)
			klog.V(4).InfoS("domain deleted", "name", domain.Name, "namespace", domain.Namespace)
			controller.enqueuePlatformByDomain(domain)

			controller.recorder.Event(domain, corev1.EventTypeNormal, "Domain deleted successfully", "Domain deleted successfully")
			event := DomainDeleted{
				DomainName: domain.Name,
				Namespace:  domain.Namespace,
			}
			err := controller.messagingPublisher(context.TODO(), DomainDeletedSuccessfullyTopic, event, domain.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.DomainDeletedSuccessfully event")
			}
		},
	})

	// Set up an event handler for when Service resources change
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*platformv1.Service)
			klog.V(4).InfoS("service added", "name", service.Name, "namespace", service.Namespace)
			controller.enqueuePlatformByService(service)

			controller.recorder.Event(service, corev1.EventTypeNormal, "Service created successfully", "Service created successfully")
			event := ServiceCreated{
				ServiceName: service.Name,
				Namespace:   service.Namespace,
				PlatformRef: service.Spec.PlatformRef,
			}
			err := controller.messagingPublisher(context.TODO(), ServiceCreatedSuccessfullyTopic, event, service.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.ServiceCreatedSuccessfully event")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldS := oldObj.(*platformv1.Service)
			newS := newObj.(*platformv1.Service)
			specChanged := !reflect.DeepEqual(oldS.Spec, newS.Spec)
			if specChanged {
				klog.V(4).InfoS("service updated", "name", newS.Name, "namespace", newS.Namespace)
				controller.enqueuePlatformByService(newS)

				if platformChanged := oldS.Spec.PlatformRef != newS.Spec.PlatformRef; platformChanged {
					controller.enqueuePlatformByService(oldS)
				}

				controller.recorder.Event(newS, corev1.EventTypeNormal, "Service updated successfully", "Service updated successfully")
				event := ServiceUpdated{
					ServiceName:        newS.Name,
					Namespace:          newS.Namespace,
					PlatformRef:        newS.Spec.PlatformRef,
					RequiredDomainRefs: newS.Spec.RequiredDomainRefs,
					OptionalDomainRefs: newS.Spec.OptionalDomainRefs,
				}
				err := controller.messagingPublisher(context.TODO(), ServiceUpdatedSuccessfullyTopic, event, newS.Spec.PlatformRef)
				if err != nil {
					klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.ServiceUpdatedSuccessfully event")
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*platformv1.Service)
			klog.V(4).InfoS("service deleted", "name", service.Name, "namespace", service.Namespace)
			controller.enqueuePlatformByService(service)

			controller.recorder.Event(service, corev1.EventTypeNormal, "Service deleted successfully", "Service deleted successfully")
			event := ServiceDeleted{
				ServiceName: service.Name,
				Namespace:   service.Namespace,
			}
			err := controller.messagingPublisher(context.TODO(), ServiceDeletedSuccessfullyTopic, event, service.Spec.PlatformRef)
			if err != nil {
				klog.ErrorS(err, "Failed to publish PlatformControllers.PlatformController.ServiceDeletedSuccessfully event")
			}
		},
	})

	// Set up an event handler for when Platform resources change
	platformInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Platform)
			klog.V(4).InfoS("platform added", "name", comp.Name)
			controller.enqueuePlatform(comp)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldP := oldObj.(*platformv1.Platform)
			newP := newObj.(*platformv1.Platform)
			klog.V(4).InfoS("platform updated", "name", newP.Name)
			if oldP.Spec != newP.Spec {
				controller.enqueuePlatform(newP)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*platformv1.Platform)
			klog.V(4).InfoS("platform deleted", "name", comp.Name)
			// Output configmap automatically deleted because it is controlled
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *PlatformController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Tenant controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configMapsSynced, c.tenantsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Platform resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *PlatformController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *PlatformController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Platform resource to be synced.
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
// converge the two. It then updates the Status block of the ConfigMapAggregate resource
// with the current status of the resource.
func (c *PlatformController) syncHandler(key string) error {
	// Get the Platform resource with this name
	platform, err := c.platformsLister.Get(key)
	if err != nil {
		// The Platform resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("platform '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	platform, err = c.resetStatus(platform)
	if err != nil {
		return err
	}

	tenants, err := c.tenantInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}

	n := 0
	for _, db := range tenants {
		if db.Spec.PlatformRef == platform.Name {
			tenants[n] = db
			n++
		}
	}
	tenants = tenants[:n]

	domains, err := c.domainInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}

	n = 0
	for _, d := range domains {
		if d.Spec.PlatformRef == platform.Name {
			domains[n] = d
			n++
		}
	}
	domains = domains[:n]

	platformCfgMap := c.genPlatformTenantsCfgMap(platform, tenants)
	err = c.syncConfigMap(platformCfgMap, platform)

	// If an error occurs we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.updateStatus(platform, false, "Sync failed: "+err.Error())
		c.recorder.Event(platform, corev1.EventTypeWarning, "Synced failed", err.Error())
		return err
	}

	for _, d := range domains {
		domainCfgMap := c.genDomainTenantsCfgMap(platform, tenants, d)
		err = c.syncConfigMap(domainCfgMap, platform)
		if err != nil {
			c.recorder.Event(d, corev1.EventTypeWarning, "Synced failed", err.Error())
		}
	}

	// Finally, we update the status block of the Platform resource to reflect the
	// current state of the world
	c.updateStatus(platform, true, "Synced successfully")
	c.recorder.Event(platform, corev1.EventTypeNormal, "Synced successfully", "Synced successfully")
	var ev = struct {
		Platform string
	}{
		Platform: platform.Name,
	}
	err = c.messagingPublisher(context.TODO(), SyncedSuccessfullyTopic, ev, platform.Name)
	if err != nil {
		klog.ErrorS(err, "message publisher error")
	}
	return nil

}

func (c *PlatformController) syncConfigMap(desiredCfgMap *corev1.ConfigMap, platform *platformv1.Platform) error {
	existingCfgMap, err := c.configMapsLister.ConfigMaps(desiredCfgMap.Namespace).Get(desiredCfgMap.Name)
	if errors.IsNotFound(err) {
		existingCfgMap, err = c.kubeClientset.CoreV1().ConfigMaps(desiredCfgMap.Namespace).Create(context.TODO(), desiredCfgMap, metav1.CreateOptions{})
	}

	if err != nil {
		return err
	}

	// If the ConfigMap is not controlled by this Platform resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(existingCfgMap, platform) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by Platform.", existingCfgMap.Name)
		c.recorder.Event(platform, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil
	}

	// If the existing ConfigMap data differs from the aggregation result we
	// should update the ConfigMap resource.
	if !reflect.DeepEqual(desiredCfgMap.Data, existingCfgMap.Data) {
		klog.V(4).Infof("Config map %s changed.", desiredCfgMap.Name)
		err = c.kubeClientset.CoreV1().ConfigMaps(desiredCfgMap.Namespace).Delete(context.TODO(), desiredCfgMap.Name, metav1.DeleteOptions{})
		if err == nil {
			_, err = c.kubeClientset.CoreV1().ConfigMaps(desiredCfgMap.Namespace).Create(context.TODO(), desiredCfgMap, metav1.CreateOptions{})
		}
	}

	return err
}

// resetStatus resets the conditions of the Platform to meta.Condition
// of type meta.ReadyCondition with status 'Unknown' and meta.ProgressingReason
// reason and message. It returns the modified Platform.
func (c *PlatformController) resetStatus(platform *platformv1.Platform) (*platformv1.Platform, error) {
	platform = platform.DeepCopy()
	platform.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  controllers.ProgressingReason,
		Message: "config generation in progress",
	}
	apimeta.SetStatusCondition(&platform.Status.Conditions, newCondition)

	return c.platformClientset.PlatformV1alpha1().Platforms().UpdateStatus(context.TODO(), platform, metav1.UpdateOptions{})
}

func (c *PlatformController) updateStatus(platform *platformv1.Platform, isReady bool, message string) {
	platform = platform.DeepCopy()

	var conditionStatus metav1.ConditionStatus
	var reason string
	if isReady {
		conditionStatus = metav1.ConditionTrue
		reason = controllers.SucceededReason
	} else {
		conditionStatus = metav1.ConditionFalse
		reason = controllers.FailedReason
	}

	platform.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(&platform.Status.Conditions, newCondition)

	_, err := c.platformClientset.PlatformV1alpha1().Platforms().UpdateStatus(context.TODO(), platform, metav1.UpdateOptions{})

	if err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *PlatformController) enqueuePlatformByTenant(tenant *platformv1.Tenant) {
	platformRef := tenant.Spec.PlatformRef
	c.workqueue.Add(platformRef)
}

func (c *PlatformController) enqueuePlatformByDomain(domain *platformv1.Domain) {
	platformRef := domain.Spec.PlatformRef
	c.workqueue.Add(platformRef)
}

func (c *PlatformController) enqueuePlatformByService(service *platformv1.Service) {
	platformRef := service.Spec.PlatformRef
	c.workqueue.Add(platformRef)
}

func (c *PlatformController) enqueuePlatform(platform *platformv1.Platform) {
	c.workqueue.Add(platform.Name)
}

func (c *PlatformController) genPlatformTenantsCfgMap(platform *platformv1.Platform, tenants []*platformv1.Tenant) *corev1.ConfigMap {
	cfgMapName := fmt.Sprintf("%s-tenants", platform.Name)
	tenantData := map[string]string{}
	for _, tenant := range tenants {
		if tenant.Spec.Configs != nil {
			for cfgKey, cfgValue := range tenant.Spec.Configs {
				tenantData[fmt.Sprintf("MultiTenancy__Tenants__%s__%s", tenant.Name, cfgKey)] = cfgValue
			}
		}
		tenantData[fmt.Sprintf("MultiTenancy__Tenants__%s__TenantId", tenant.Name)] = tenant.Spec.Id
		tenantData[fmt.Sprintf("MultiTenancy__Tenants__%s__Enabled", tenant.Name)] = strconv.FormatBool(tenant.Spec.Enabled)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cfgMapName,
			Labels: map[string]string{
				controllers.PlatformLabelName: platform.Name,
				controllers.DomainLabelName:   controllers.GlobalDomainLabelValue,
			},
			Namespace: platform.Spec.TargetNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(platform, platformv1.SchemeGroupVersion.WithKind("Platform")),
			},
		},
		Data:      tenantData,
		Immutable: func(b bool) *bool { return &b }(true),
	}
}

func (c *PlatformController) genDomainTenantsCfgMap(platform *platformv1.Platform, tenants []*platformv1.Tenant, domain *platformv1.Domain) *corev1.ConfigMap {
	cfgMapName := fmt.Sprintf("%s-tenants", domain.Name)
	tenantData := map[string]string{}
	for _, tenant := range tenants {
		tenantEnabled := tenant.Spec.Enabled && tenantHasAccessToDomain(tenant, domain.Name)
		tenantData[fmt.Sprintf("MultiTenancy__Tenants__%s__Enabled", tenant.Name)] = strconv.FormatBool(tenant.Spec.Enabled && tenantEnabled)
		if domain.Spec.ExportActiveDomains && tenantEnabled {
			for _, domain := range tenant.Spec.DomainRefs {
				tenantData[fmt.Sprintf("MultiTenancy__Tenants__%s__Domains__%s__Enabled", tenant.Name, domain)] = strconv.FormatBool(true)
			}
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cfgMapName,
			Labels: map[string]string{
				controllers.PlatformLabelName: platform.Name,
				controllers.DomainLabelName:   domain.Name,
			},
			Namespace: domain.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(platform, platformv1.SchemeGroupVersion.WithKind("Platform")),
			},
		},
		Data:      tenantData,
		Immutable: func(b bool) *bool { return &b }(true),
	}
}

func tenantHasAccessToDomain(tenant *platformv1.Tenant, domainName string) bool {
	for _, d := range tenant.Spec.DomainRefs {
		if d == domainName {
			return true
		}
	}
	return false
}
