package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
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
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/configuration/v1alpha1"
	listers "totalsoft.ro/platform-controllers/pkg/generated/listers/configuration/v1alpha1"
)

const (
	domainLabelName           = "platform.totalsoft.ro/domain"
	platformLabelName         = "platform.totalsoft.ro/platform"
	configControllerAgentName = "configuration-controller"
	allDomainsLabelValue      = "all"

	// SuccessSynced is used as part of the Event 'reason' when a ConfigurationAggregate is synced
	SuccessConfigAggregateSynced = "Synced successfully"

	// ErrResourceExists is used as part of the Event 'reason' when a ConfigurationAggregate fails
	// to sync due to a ConfigMap of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a ConfigMap already existing
	MessageResourceExists = "Resource %q already exists and is not managed by ConfigurationAggregate"

	// MessageResourceSynced is the message used for an Event fired when a ConfigurationAggregate
	// is synced successfully
	MessageResourceSynced = "Synced successfully"

	// ReadyCondition indicates the resource is ready and fully reconciled.
	// If the Condition is False, the resource SHOULD be considered to be in the process of reconciling and not a
	// representation of actual state.
	ReadyCondition string = "Ready"
)

type ConfigurationController struct {
	kubeClientset           kubernetes.Interface
	configurationClientset  clientset.Interface
	configMapInformer       coreInformers.ConfigMapInformer
	configAggregateInformer informers.ConfigurationAggregateInformer

	configMapsLister       coreListers.ConfigMapLister
	configMapsSynced       cache.InformerSynced
	configAggregatesLister listers.ConfigurationAggregateLister
	configAggregatesSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.

	recorder record.EventRecorder
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

func NewConfigurationController(
	kubeClientset kubernetes.Interface,
	configurationClientset clientset.Interface,
	configMapInformer coreInformers.ConfigMapInformer,
	configAggregateInformer informers.ConfigurationAggregateInformer,
	eventBroadcaster record.EventBroadcaster,
) *ConfigurationController {
	controller := &ConfigurationController{
		kubeClientset:           kubeClientset,
		configurationClientset:  configurationClientset,
		configMapInformer:       configMapInformer,
		configMapsLister:        configMapInformer.Lister(),
		configMapsSynced:        configMapInformer.Informer().HasSynced,
		configAggregateInformer: configAggregateInformer,
		configAggregatesLister:  configAggregateInformer.Lister(),
		configAggregatesSynced:  configAggregateInformer.Informer().HasSynced,

		recorder:  &record.FakeRecorder{},
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configuration"),
	}

	utilruntime.Must(clientsetScheme.AddToScheme(scheme.Scheme))
	if eventBroadcaster != nil {
		controller.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: configControllerAgentName})
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when ConfigAggregate resources change
	addConfigAggregateHandlers(configAggregateInformer, controller.enqueueDomain)
	addConfigMapHandlers(configMapInformer, controller.enqueueDomain)

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *ConfigurationController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting configuration controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configMapsSynced, c.configAggregatesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process ConfigurationAggregate and ConfigMap resources
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
func (c *ConfigurationController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ConfigurationController) processNextWorkItem() bool {
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
		// Foo resource to be synced.
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
func (c *ConfigurationController) syncHandler(key string) error {
	// Convert the namespace::domain string into a distinct namespace and domain
	platform, domain, err := decodeDomainKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	outputConfigMapName := fmt.Sprintf("%s-%s-aggregate", platform, domain)

	domainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			domainLabelName:   domain,
			platformLabelName: platform,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	allDomainsAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			domainLabelName:   allDomainsLabelValue,
			platformLabelName: platform,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	// Get the ConfigAggregates resource with this namespace::domain
	configAggregates, err := c.configAggregatesLister.ConfigurationAggregates("").List(labels.Everything())
	if err != nil {
		return err
	}

	n := 0
	for _, db := range configAggregates {
		if db.Spec.PlatformRef == platform && db.Spec.Domain == domain {
			configAggregates[n] = db
			n++
		}
	}
	configAggregates = configAggregates[:n]

	if len(configAggregates) != 1 {
		// Cleanup if configAggregate was invalidated
		if len(configAggregates) == 0 {
			domainConfigMaps, err := c.configMapsLister.ConfigMaps("").List(domainAndPlatformLabelSelector)
			if err == nil {
				for _, domainConfigMap := range domainConfigMaps {
					if domainConfigMap.Name == outputConfigMapName {
						c.kubeClientset.CoreV1().ConfigMaps(domainConfigMap.Namespace).Delete(context.TODO(), domainConfigMap.Name, metav1.DeleteOptions{})
					}
				}
			}
		}
		msg := fmt.Sprintf("there should be exactly one ConfigMapAggregate for a platform %s and domain %s. Found: %d", platform, domain, len(configAggregates))

		for _, configAggregate := range configAggregates {
			c.recorder.Event(configAggregate, corev1.EventTypeWarning, ErrorSynced, msg)
			c.updateStatus(configAggregate, false, "Aggregation failed: "+msg)
		}

		utilruntime.HandleError(fmt.Errorf(msg))
		return nil
	}

	configAggregate := configAggregates[0]
	configAggregate, err = c.resetStatus(configAggregate)
	if err != nil {
		return err
	}

	allDomainsConfigMaps, err := c.configMapsLister.ConfigMaps("").List(allDomainsAndPlatformLabelSelector)
	if err != nil {
		c.updateStatus(configAggregate, false, err.Error())
		return err
	}

	configMaps, err := c.configMapsLister.ConfigMaps("").List(domainAndPlatformLabelSelector)
	if err != nil {
		c.updateStatus(configAggregate, false, err.Error())
		return err
	}

	configMaps = append(allDomainsConfigMaps, configMaps...)

	aggregatedConfigMap := c.aggregateConfigMaps(configAggregate, configMaps, outputConfigMapName)

	// Get the output config map for this namespace::domain
	outputConfigMap, err := c.configMapsLister.ConfigMaps(configAggregate.Namespace).Get(outputConfigMapName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		outputConfigMap, err = c.kubeClientset.CoreV1().ConfigMaps(configAggregate.Namespace).Create(context.TODO(), aggregatedConfigMap, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(configAggregate, corev1.EventTypeWarning, ErrorSynced, err.Error())
		c.updateStatus(configAggregate, false, "Aggregation failed"+err.Error())
		return err
	}

	// If the ConfigMap is not controlled by this ConfigMapAggregate resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputConfigMap, configAggregate) {
		msg := fmt.Sprintf(MessageResourceExists, outputConfigMap.Name)
		c.recorder.Event(configAggregate, corev1.EventTypeWarning, ErrResourceExists, msg)
		c.updateStatus(configAggregate, false, "Aggregation failed"+err.Error())
		return nil
	}

	// If the existing ConfigMap data differs from the aggregation result we
	// should update the ConfigMap resource.
	if !reflect.DeepEqual(aggregatedConfigMap.Data, outputConfigMap.Data) {
		klog.V(4).Infof("Configuration values changed")
		err = c.kubeClientset.CoreV1().ConfigMaps(configAggregate.Namespace).Delete(context.TODO(), aggregatedConfigMap.Name, metav1.DeleteOptions{})
		if err == nil {
			_, err = c.kubeClientset.CoreV1().ConfigMaps(configAggregate.Namespace).Create(context.TODO(), aggregatedConfigMap, metav1.CreateOptions{})
		}
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(configAggregate, corev1.EventTypeWarning, ErrorSynced, err.Error())
		c.updateStatus(configAggregate, false, "Aggregation failed: "+err.Error())
		return err
	}

	// Finally, we update the status block of the ConfigurationAggregate resource to reflect the
	// current state of the world
	c.updateStatus(configAggregate, true, SuccessConfigAggregateSynced)
	c.recorder.Event(configAggregate, corev1.EventTypeNormal, SuccessConfigAggregateSynced, MessageResourceSynced)
	return nil
}

// resetStatus resets the conditions of the ConfigurationAggregate to meta.Condition
// of type meta.ReadyCondition with status 'Unknown' and meta.ProgressingReason
// reason and message. It returns the modified ConfigurationAggregate.
func (c *ConfigurationController) resetStatus(configAggregate *v1alpha1.ConfigurationAggregate) (*v1alpha1.ConfigurationAggregate, error) {
	configAggregate = configAggregate.DeepCopy()
	configAggregate.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  ProgressingReason,
		Message: "aggregation in progress",
	}
	apimeta.SetStatusCondition(&configAggregate.Status.Conditions, newCondition)

	return c.configurationClientset.ConfigurationV1alpha1().ConfigurationAggregates(configAggregate.Namespace).UpdateStatus(context.TODO(), configAggregate, metav1.UpdateOptions{})
}

func (c *ConfigurationController) updateStatus(configAggregate *v1alpha1.ConfigurationAggregate, isReady bool, message string) {
	configAggregate = configAggregate.DeepCopy()

	var conditionStatus metav1.ConditionStatus
	var reason string
	if isReady {
		conditionStatus = metav1.ConditionTrue
		reason = SucceededReason
	} else {
		conditionStatus = metav1.ConditionFalse
		reason = FailedReason
	}

	configAggregate.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(&configAggregate.Status.Conditions, newCondition)

	_, err := c.configurationClientset.ConfigurationV1alpha1().ConfigurationAggregates(configAggregate.DeepCopy().Namespace).UpdateStatus(context.TODO(), configAggregate, metav1.UpdateOptions{})

	if err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *ConfigurationController) enqueueDomain(platform string, domain string) {
	if domain == allDomainsLabelValue {
		configAggregates, err := c.configAggregatesLister.ConfigurationAggregates("").List(labels.Everything())
		if err != nil {
			return
		}
		for _, configAggregate := range configAggregates {
			if configAggregate.Spec.PlatformRef == platform {
				key := encodeDomainKey(platform, configAggregate.Spec.Domain)
				c.workqueue.Add(key)
			}
		}
	} else {
		key := encodeDomainKey(platform, domain)
		c.workqueue.Add(key)
	}

}

func (c *ConfigurationController) aggregateConfigMaps(configMapAggregate *v1alpha1.ConfigurationAggregate, configMaps []*corev1.ConfigMap, outputName string) *corev1.ConfigMap {
	mergedData := map[string]string{}
	for _, configMap := range configMaps {
		if configMap.Name == outputName {
			continue
		}

		for k, v := range configMap.Data {
			if existingValue, ok := mergedData[k]; ok {
				msg := fmt.Sprintf("Key %s already exists with value %s. It will be replaced by config map %s with value %s", k, existingValue, configMap.Name, v)
				c.recorder.Event(configMapAggregate, corev1.EventTypeWarning, ErrResourceExists, msg)
			}
			mergedData[k] = v
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: outputName,
			Labels: map[string]string{
				domainLabelName:   configMapAggregate.Spec.Domain,
				platformLabelName: configMapAggregate.Spec.PlatformRef,
			},
			Namespace: configMapAggregate.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(configMapAggregate, v1alpha1.SchemeGroupVersion.WithKind("ConfigurationAggregate")),
			},
		},
		Data:      mergedData,
		Immutable: func(b bool) *bool { return &b }(true),
	}
}

func encodeDomainKey(platform string, domain string) (key string) {
	return fmt.Sprintf("%s::%s", platform, domain)
}

func decodeDomainKey(key string) (platform string, domain string, err error) {
	res := strings.Split(key, "::")
	if len(res) == 2 {
		return res[0], res[1], nil
	}
	return "", "", fmt.Errorf("cannot decode key: %v", key)
}

func addConfigAggregateHandlers(informer informers.ConfigurationAggregateInformer, handler func(platform string, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*v1alpha1.ConfigurationAggregate)
			if platform, domain, ok := getAggregateConfigPlatformAndDomain(comp); ok {
				klog.V(4).InfoS("ConfigMapAggregate added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				handler(platform, domain)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*v1alpha1.ConfigurationAggregate)
			newComp := newObj.(*v1alpha1.ConfigurationAggregate)
			oldPlatform, oldDomain, oldOk := getAggregateConfigPlatformAndDomain(oldComp)
			newPlatform, newDomain, newOk := getAggregateConfigPlatformAndDomain(newComp)
			targetChanged := oldPlatform != newPlatform || oldDomain != newDomain

			if !oldOk && !newOk {
				return
			}

			if oldOk && (targetChanged || !newOk) {
				klog.V(4).InfoS("ConfigMapAggregate invalidated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", oldPlatform, "domain", oldDomain)
				handler(oldPlatform, oldDomain)
				return
			}

			if oldOk && newOk {
				if oldComp.Spec == newComp.Spec && reflect.DeepEqual(oldComp.Labels, newComp.Labels) {
					return
				}

				klog.V(4).InfoS("ConfigMapAggregate updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", oldPlatform, "domain", newDomain)
				handler(newPlatform, newDomain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*v1alpha1.ConfigurationAggregate)
			if platform, domain, ok := getAggregateConfigPlatformAndDomain(comp); ok {
				klog.V(4).InfoS("ConfigMapAggregate deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				// Output configmap automatically deleted because it is controlled
			}
		},
	})
}

func addConfigMapHandlers(informer coreInformers.ConfigMapInformer, handler func(platform string, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*corev1.ConfigMap)

			if platform, domain, ok := getConfigMapPlatformAndDomain(comp); ok {
				if isControlledByConfigAggregate(comp) {
					return
				}

				klog.V(4).InfoS("Config map added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				handler(platform, domain)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*corev1.ConfigMap)
			newComp := newObj.(*corev1.ConfigMap)

			oldPlatform, oldDomain, oldOk := getConfigMapPlatformAndDomain(oldComp)
			newPlatform, newDomain, newOk := getConfigMapPlatformAndDomain(newComp)
			targetChanged := oldPlatform != newPlatform || oldDomain != newDomain

			if !oldOk && !newOk {
				return
			}

			if oldOk && (!newOk || targetChanged) {
				klog.V(4).InfoS("Config map updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", oldPlatform, "domain", oldDomain)
				handler(oldPlatform, oldDomain)
			}

			if newOk {
				if reflect.DeepEqual(oldComp.Data, newComp.Data) && reflect.DeepEqual(oldComp.Labels, newComp.Labels) {
					return
				}

				klog.V(4).InfoS("Config map updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", newPlatform, "domain", newDomain)
				handler(newPlatform, newDomain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*corev1.ConfigMap)

			if platform, domain, ok := getConfigMapPlatformAndDomain(comp); ok {
				if isControlledByConfigAggregate(comp) {
					return
				}

				klog.V(4).InfoS("Config map deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				handler(platform, domain)
			}
		},
	})
}

func getAggregateConfigPlatformAndDomain(configAggregate *v1alpha1.ConfigurationAggregate) (platform string, domain string, ok bool) {
	domain = configAggregate.Spec.Domain
	if len(domain) == 0 {
		return "", domain, false
	}

	platform = configAggregate.Spec.PlatformRef
	if len(platform) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}

func getConfigMapPlatformAndDomain(configMap *corev1.ConfigMap) (platform string, domain string, ok bool) {
	domain, domainLabelExists := configMap.Labels[domainLabelName]
	if !domainLabelExists || len(domain) == 0 {
		return "", domain, false
	}

	platform, platformLabelExists := configMap.Labels[platformLabelName]
	if !platformLabelExists || len(platform) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}

func isControlledByConfigAggregate(configMap *corev1.ConfigMap) bool {
	owner := metav1.GetControllerOf(configMap)
	return (owner != nil &&
		owner.Kind == "ConfigurationAggregate" &&
		owner.APIVersion == "configuration.totalsoft.ro/v1alpha1")
}
