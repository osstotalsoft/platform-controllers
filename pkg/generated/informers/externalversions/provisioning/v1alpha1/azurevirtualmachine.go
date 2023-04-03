/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	provisioningv1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	versioned "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	internalinterfaces "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/listers/provisioning/v1alpha1"
)

// AzureVirtualMachineInformer provides access to a shared informer and lister for
// AzureVirtualMachines.
type AzureVirtualMachineInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.AzureVirtualMachineLister
}

type azureVirtualMachineInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewAzureVirtualMachineInformer constructs a new informer for AzureVirtualMachine type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewAzureVirtualMachineInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredAzureVirtualMachineInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredAzureVirtualMachineInformer constructs a new informer for AzureVirtualMachine type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredAzureVirtualMachineInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ProvisioningV1alpha1().AzureVirtualMachines(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ProvisioningV1alpha1().AzureVirtualMachines(namespace).Watch(context.TODO(), options)
			},
		},
		&provisioningv1alpha1.AzureVirtualMachine{},
		resyncPeriod,
		indexers,
	)
}

func (f *azureVirtualMachineInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredAzureVirtualMachineInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *azureVirtualMachineInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&provisioningv1alpha1.AzureVirtualMachine{}, f.defaultInformer)
}

func (f *azureVirtualMachineInformer) Lister() v1alpha1.AzureVirtualMachineLister {
	return v1alpha1.NewAzureVirtualMachineLister(f.Informer().GetIndexer())
}
