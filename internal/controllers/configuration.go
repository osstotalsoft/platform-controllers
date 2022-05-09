package controllers

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformers "k8s.io/client-go/informers/core/v1"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/configuration/v1alpha1"
	listers "totalsoft.ro/platform-controllers/pkg/generated/listers/configuration/v1alpha1"
)

type ConfigurationController struct {
	kubeClientset           kubernetes.Interface
	configurationClientset  clientset.Interface
	configMapInformer       v1.ConfigMapInformer
	configAggregateInformer informers.ConfigurationAggregateInformer

	configMapsLister       coreListers.ConfigMapLister
	configMapsSynced       cache.InformerSynced
	configAggregatesLister listers.ConfigurationAggregateLister
	configAggregatesSynced cache.InformerSynced

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
) *ConfigurationController {
	controller := &ConfigurationController{
		kubeClientset:           kubeClientset,
		configurationClientset:  configurationClientset,
		configMapInformer:       configMapInformer,
		configAggregateInformer: configAggregateInformer,
		workqueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configuration"),
	}

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
	klog.Info("Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configMapsSynced, c.configAggregatesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Foo resources
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
	// for c.processNextWorkItem() {
	// }
}
