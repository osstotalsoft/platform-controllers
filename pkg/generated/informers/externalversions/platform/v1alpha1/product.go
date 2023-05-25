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
	platformv1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	versioned "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	internalinterfaces "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/listers/platform/v1alpha1"
)

// ProductInformer provides access to a shared informer and lister for
// Products.
type ProductInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ProductLister
}

type productInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewProductInformer constructs a new informer for Product type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewProductInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredProductInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredProductInformer constructs a new informer for Product type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredProductInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PlatformV1alpha1().Products().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PlatformV1alpha1().Products().Watch(context.TODO(), options)
			},
		},
		&platformv1alpha1.Product{},
		resyncPeriod,
		indexers,
	)
}

func (f *productInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredProductInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *productInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&platformv1alpha1.Product{}, f.defaultInformer)
}

func (f *productInformer) Lister() v1alpha1.ProductLister {
	return v1alpha1.NewProductLister(f.Informer().GetIndexer())
}
