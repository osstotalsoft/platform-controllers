package configuration

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/configuration/v1alpha1"
	platformInformers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/platform/v1alpha1"
	listers "totalsoft.ro/platform-controllers/pkg/generated/listers/configuration/v1alpha1"
	platformListers "totalsoft.ro/platform-controllers/pkg/generated/listers/platform/v1alpha1"

	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	csiClientset "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	csiInformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions/apis/v1"
	csiListers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/listers/apis/v1"
	messaging "totalsoft.ro/platform-controllers/internal/messaging"
)

const (
	configControllerAgentName = "configuration-controller"

	// SuccessSynced is used as part of the Event 'reason' when a ConfigurationDomain is synced
	SuccessConfigurationDomainSynced = "Synced successfully"

	// ErrResourceExists is used as part of the Event 'reason' when a ConfigurationDomain fails
	// to sync due to a ConfigMap of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a ConfigMap already existing
	MessageResourceExists = "Resource %q already exists and is not managed by ConfigurationDomain"

	// MessageResourceSynced is the message used for an Event fired when a ConfigurationDomain
	// is synced successfully
	MessageResourceSynced = "Synced successfully"

	// ReadyCondition indicates the resource is ready and fully reconciled.
	// If the Condition is False, the resource SHOULD be considered to be in the process of reconciling and not a
	// representation of actual state.
	ReadyCondition = "Ready"

	syncedSuccessfullyTopic = "PlatformControllers.ConfigurationDomainController.SyncedSuccessfully"
)

var requeueInterval time.Duration = 2 * time.Minute

type ConfigurationDomainController struct {
	kubeClientset        kubernetes.Interface
	csiClientset         csiClientset.Interface
	platformClientset    clientset.Interface
	configMapInformer    coreInformers.ConfigMapInformer
	kubeSecretInformer   coreInformers.SecretInformer
	spcInformer          csiInformers.SecretProviderClassInformer
	configDomainInformer informers.ConfigurationDomainInformer

	configMapsLister    coreListers.ConfigMapLister
	configMapsSynced    cache.InformerSynced
	kubeSecretsLister   coreListers.SecretLister
	kubeSecretsSynced   cache.InformerSynced
	spcLister           csiListers.SecretProviderClassLister
	spcSynced           cache.InformerSynced
	configDomainsLister listers.ConfigurationDomainLister
	configDomainsSynced cache.InformerSynced
	platformsLister     platformListers.PlatformLister
	platformsSynced     cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.

	recorder record.EventRecorder
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	configurationHandler *configurationHandler
	kubeSecretsHandler   *kubeSecretsHandler
	vaultSecretsHandler  *secretsHandler
	messagingPublisher   messaging.MessagingPublisher
}

func NewConfigurationDomainController(
	platformClientset clientset.Interface,
	kubeClientset kubernetes.Interface,
	csiClientset csiClientset.Interface,

	platformInformer platformInformers.PlatformInformer,
	configDomainInformer informers.ConfigurationDomainInformer,
	configMapInformer coreInformers.ConfigMapInformer,
	kubeSecretInformer coreInformers.SecretInformer,
	spcInformer csiInformers.SecretProviderClassInformer,

	eventBroadcaster record.EventBroadcaster,
	messagingPublisher messaging.MessagingPublisher,
) *ConfigurationDomainController {
	controller := &ConfigurationDomainController{
		platformClientset: platformClientset,
		kubeClientset:     kubeClientset,
		csiClientset:      csiClientset,

		platformsLister: platformInformer.Lister(),
		platformsSynced: platformInformer.Informer().HasSynced,

		configDomainInformer: configDomainInformer,
		configDomainsLister:  configDomainInformer.Lister(),
		configDomainsSynced:  configDomainInformer.Informer().HasSynced,

		configMapInformer: configMapInformer,
		configMapsLister:  configMapInformer.Lister(),
		configMapsSynced:  configMapInformer.Informer().HasSynced,

		kubeSecretInformer: kubeSecretInformer,
		kubeSecretsLister:  kubeSecretInformer.Lister(),
		kubeSecretsSynced:  kubeSecretInformer.Informer().HasSynced,

		spcInformer: spcInformer,
		spcLister:   spcInformer.Lister(),
		spcSynced:   spcInformer.Informer().HasSynced,

		recorder:           &record.FakeRecorder{},
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configuration-domain"),
		messagingPublisher: messagingPublisher,
	}

	utilruntime.Must(clientsetScheme.AddToScheme(scheme.Scheme))
	if eventBroadcaster != nil {
		controller.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: configControllerAgentName})
	}

	controller.configurationHandler = newConfigurationHandler(kubeClientset, configMapInformer.Lister(), controller.recorder)
	controller.kubeSecretsHandler = newKubeSecretsHandler(kubeClientset, kubeSecretInformer.Lister(), controller.recorder)
	controller.vaultSecretsHandler = newVaultSecretsHandler(csiClientset, spcInformer.Lister(), controller.recorder)

	klog.Info("Setting up event handlers")

	// Set up an event handler for when ConfigAggregate resources change
	addPlatformHandlers(platformInformer)
	addConfigurationDomainHandlers(configDomainInformer, controller.enqueueConfigurationDomain)
	addConfigMapHandlers(configMapInformer, controller.handleKubeResource)
	addKubeSecretsHandlers(kubeSecretInformer, controller.handleKubeResource)
	addSPCHandlers(spcInformer, controller.enqueueConfigurationDomain)

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *ConfigurationDomainController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting configuration domain controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.platformsSynced, c.configDomainsSynced, c.configMapsSynced, c.kubeSecretsSynced, c.spcSynced); !ok {
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
func (c *ConfigurationDomainController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ConfigurationDomainController) processNextWorkItem() bool {
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
func (c *ConfigurationDomainController) syncHandler(key string) error {
	// Convert the namespace::domain string into a distinct namespace and domain
	namespace, domain, err := decodeDomainKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the ConfigurationDomain resource
	configDomain, err := c.configDomainsLister.ConfigurationDomains(namespace).Get(domain)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	platformObj, err := c.platformsLister.Get(configDomain.Spec.PlatformRef)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	platformNotOk := platformObj == nil
	cleanupConfigMap := platformNotOk || !configDomain.Spec.AggregateConfigMaps
	if cleanupConfigMap {
		err = c.configurationHandler.Cleanup(namespace, domain)
		if err != nil {
			return err
		}
	}

	cleanupSecrets := platformNotOk || !configDomain.Spec.AggregateSecrets
	if cleanupSecrets {
		err = c.vaultSecretsHandler.Cleanup(namespace, domain)
		if err != nil {
			return err
		}

		err = c.kubeSecretsHandler.Cleanup(namespace, domain)
		if err != nil {
			return err
		}
	}

	if cleanupConfigMap && cleanupSecrets {
		return nil
	}

	configDomain, err = c.resetStatus(configDomain)
	if err != nil {
		return err
	}

	if !cleanupConfigMap {
		err = c.configurationHandler.Sync(platformObj, configDomain)
		if err != nil {
			c.updateStatus(configDomain, false, "Aggregation failed"+err.Error())
			return err
		}
	}

	if !cleanupSecrets {
		err = c.vaultSecretsHandler.Sync(platformObj, configDomain)
		if err != nil {
			c.updateStatus(configDomain, false, "Aggregation failed"+err.Error())
			return err
		}

		err = c.kubeSecretsHandler.Sync(platformObj, configDomain)
		if err != nil {
			c.updateStatus(configDomain, false, "Aggregation failed"+err.Error())
			return err
		}

		//Requeue the processing after the configured interval
		c.workqueue.AddAfter(key, requeueInterval)
	}

	// Finally, we update the status block of the ConfigurationDomain resource to reflect the
	// current state of the world
	c.updateStatus(configDomain, true, SuccessConfigurationDomainSynced)
	c.recorder.Event(configDomain, corev1.EventTypeNormal, SuccessConfigurationDomainSynced, MessageResourceSynced)
	var ev = struct {
		Domain    string
		Namespace string
	}{
		Domain:    domain,
		Namespace: namespace,
	}
	err = c.messagingPublisher(context.TODO(), syncedSuccessfullyTopic, ev, platformObj.Name)
	if err != nil {
		klog.ErrorS(err, "message publisher error")
	}
	return nil
}

// resetStatus resets the conditions of the ConfigurationAggregate to meta.Condition
// of type meta.ReadyCondition with status 'Unknown' and meta.ProgressingReason
// reason and message. It returns the modified ConfigurationAggregate.
func (c *ConfigurationDomainController) resetStatus(configurationDomain *v1alpha1.ConfigurationDomain) (*v1alpha1.ConfigurationDomain, error) {
	configurationDomain = configurationDomain.DeepCopy()
	configurationDomain.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  controllers.ProgressingReason,
		Message: "aggregation in progress",
	}
	apimeta.SetStatusCondition(&configurationDomain.Status.Conditions, newCondition)

	return c.platformClientset.ConfigurationV1alpha1().ConfigurationDomains(configurationDomain.Namespace).UpdateStatus(context.TODO(), configurationDomain, metav1.UpdateOptions{})
}

func (c *ConfigurationDomainController) updateStatus(configurationDomain *v1alpha1.ConfigurationDomain, isReady bool, message string) {
	configurationDomain = configurationDomain.DeepCopy()

	var conditionStatus metav1.ConditionStatus
	var reason string
	if isReady {
		conditionStatus = metav1.ConditionTrue
		reason = controllers.SucceededReason
	} else {
		conditionStatus = metav1.ConditionFalse
		reason = controllers.FailedReason
	}

	configurationDomain.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(&configurationDomain.Status.Conditions, newCondition)

	_, err := c.platformClientset.ConfigurationV1alpha1().ConfigurationDomains(configurationDomain.DeepCopy().Namespace).UpdateStatus(context.TODO(), configurationDomain, metav1.UpdateOptions{})

	if err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *ConfigurationDomainController) handleKubeResource(platform, namespace, domain string) {
	platformObj, err := c.platformsLister.Get(platform)
	if err != nil {
		return
	}

	platformNamespace := namespace == platformObj.Spec.TargetNamespace
	globalDomain := domain == controllers.GlobalDomainLabelValue

	if platformNamespace || globalDomain {
		if platformNamespace {
			namespace = ""
		}
		configDomains, err := c.configDomainsLister.ConfigurationDomains(namespace).List(labels.Everything())
		if err != nil {
			return
		}
		for _, configurationDomain := range configDomains {
			if configurationDomain.Spec.PlatformRef == platform {
				c.enqueueConfigurationDomain(configurationDomain.Namespace, configurationDomain.Name)
			}
		}
	} else {
		c.enqueueConfigurationDomain(namespace, domain)
	}
}

func (c *ConfigurationDomainController) enqueueConfigurationDomain(namespace, domain string) {
	key := encodeDomainKey(namespace, domain)
	klog.InfoS("enqueue ConfigurationDomain", "namespace", namespace, "domain", domain)
	c.workqueue.Add(key)
}

func encodeDomainKey(namespace, domain string) (key string) {
	return fmt.Sprintf("%s::%s", namespace, domain)
}

func decodeDomainKey(key string) (namespace, domain string, err error) {
	res := strings.Split(key, "::")
	if len(res) == 2 {
		return res[0], res[1], nil
	}
	return "", "", fmt.Errorf("cannot decode key: %v", key)
}

func addConfigurationDomainHandlers(informer informers.ConfigurationDomainInformer, handler func(namespace, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cd := obj.(*v1alpha1.ConfigurationDomain)
			klog.V(4).InfoS("ConfigurationDomain added", "name", cd.Name, "namespace", cd.Namespace)
			handler(cd.Namespace, cd.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*v1alpha1.ConfigurationDomain)
			newComp := newObj.(*v1alpha1.ConfigurationDomain)
			domain := newComp.Name
			namespace := newComp.Namespace
			specChanged := oldComp.Spec != newComp.Spec

			if specChanged {
				klog.V(4).InfoS("ConfigurationDomain updated", "name", newComp.Name, "namespace", newComp.Namespace)
				handler(namespace, domain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*v1alpha1.ConfigurationDomain)
			klog.V(4).InfoS("ConfigurationDomain deleted", "name", comp.Name, "namespace", comp.Namespace)
			// Output configmap and spc automatically deleted because it is controlled
		},
	})
}

func addConfigMapHandlers(informer coreInformers.ConfigMapInformer, handler func(platform, namespace, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*corev1.ConfigMap)

			if platform, domainNs, domain, ok := controllers.GetPlatformNsAndDomain(comp); ok {
				if isOutputConfigMap(comp) {
					return
				}

				klog.V(4).InfoS("Config map added", "name", comp.Name, "namespace", domainNs, "platform", platform, "domain", domain)
				handler(platform, domainNs, domain)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*corev1.ConfigMap)
			newComp := newObj.(*corev1.ConfigMap)

			// if isControlledByConfigAggregate(newComp) {
			// 	return
			// }

			oldPlatform, oldDomainNs, oldDomain, oldOk := controllers.GetPlatformNsAndDomain(oldComp)
			newPlatform, newDomainNs, newDomain, newOk := controllers.GetPlatformNsAndDomain(newComp)
			targetChanged := oldPlatform != newPlatform || oldDomain != newDomain

			if !oldOk && !newOk {
				return
			}

			if oldOk && targetChanged {
				klog.V(4).InfoS("Config map updated", "name", newComp.Name, "namespace", oldDomainNs, "platform", oldPlatform, "domain", oldDomain)
				handler(oldPlatform, oldDomainNs, oldDomain)
			}
			dataChanged := !reflect.DeepEqual(oldComp.Data, newComp.Data) || !reflect.DeepEqual(oldComp.Labels, newComp.Labels)
			if newOk && dataChanged {
				klog.V(4).InfoS("Config map updated", "name", newComp.Name, "namespace", newDomainNs, "platform", newPlatform, "domain", newDomain)
				handler(newPlatform, newDomainNs, newDomain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*corev1.ConfigMap)

			// if isControlledByConfigAggregate(newComp) {
			// 	return
			// }

			if platform, domainNs, domain, ok := controllers.GetPlatformNsAndDomain(comp); ok {

				klog.V(4).InfoS("Config map deleted", "name", comp.Name, "namespace", domainNs, "platform", platform, "domain", domain)
				handler(platform, domainNs, domain)
			}
		},
	})
}

func addKubeSecretsHandlers(informer coreInformers.SecretInformer, handler func(platform, namespace, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*corev1.Secret)

			if platform, domainNs, domain, ok := controllers.GetPlatformNsAndDomain(&comp.ObjectMeta); ok {
				if isOutputSecret(comp) {
					return
				}

				klog.V(4).InfoS("Config map added", "name", comp.Name, "namespace", domainNs, "platform", platform, "domain", domain)
				handler(platform, domainNs, domain)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*corev1.Secret)
			newComp := newObj.(*corev1.Secret)

			// if isControlledByConfigAggregate(newComp) {
			// 	return
			// }

			oldPlatform, oldDomainNs, oldDomain, oldOk := controllers.GetPlatformNsAndDomain(oldComp)
			newPlatform, newDomainNs, newDomain, newOk := controllers.GetPlatformNsAndDomain(newComp)
			targetChanged := oldPlatform != newPlatform || oldDomain != newDomain

			if !oldOk && !newOk {
				return
			}

			if oldOk && targetChanged {
				klog.V(4).InfoS("Secret updated", "name", newComp.Name, "namespace", oldDomainNs, "platform", oldPlatform, "domain", oldDomain)
				handler(oldPlatform, oldDomainNs, oldDomain)
			}
			dataChanged := !reflect.DeepEqual(oldComp.Data, newComp.Data) || !reflect.DeepEqual(oldComp.Labels, newComp.Labels)
			if newOk && dataChanged {
				klog.V(4).InfoS("Secret updated", "name", newComp.Name, "namespace", newDomainNs, "platform", newPlatform, "domain", newDomain)
				handler(newPlatform, newDomainNs, newDomain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*corev1.Secret)

			// if isControlledByConfigAggregate(newComp) {
			// 	return
			// }

			if platform, domainNs, domain, ok := controllers.GetPlatformNsAndDomain(comp); ok {

				klog.V(4).InfoS("Secret deleted", "name", comp.Name, "namespace", domainNs, "platform", platform, "domain", domain)
				handler(platform, domainNs, domain)
			}
		},
	})
}

func addPlatformHandlers(informer platformInformers.PlatformInformer) {
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

func addSPCHandlers(informer csiInformers.SecretProviderClassInformer, handler func(namespace, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
		},
		DeleteFunc: func(obj interface{}) {
			spc := obj.(*csiv1.SecretProviderClass)

			if _, domain, ok := getSPCPlatformAndDomain(spc); isOutputSPC(spc) && ok {
				klog.V(4).InfoS("Secret provider class deleted", "name", spc.Name, "namespace", spc.Namespace, "domain", domain)
				handler(spc.Namespace, domain)
			}
		},
	})
}
