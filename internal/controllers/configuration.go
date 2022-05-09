package controllers

import (
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

type ConfigurationController struct {
	clientset         kubernetes.Interface
	configMapInformer v1.ConfigMapInformer

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

func NewController(
	clientset kubernetes.Interface,
	configMapInformer v1.ConfigMapInformer,
) *ConfigurationController {
	controller := &ConfigurationController{
		clientset:         clientset,
		configMapInformer: configMapInformer,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configuration"),
	}

	return controller
}
