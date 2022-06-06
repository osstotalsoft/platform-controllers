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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
)

// SecretsAggregateLister helps list SecretsAggregates.
// All objects returned here must be treated as read-only.
type SecretsAggregateLister interface {
	// List lists all SecretsAggregates in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.SecretsAggregate, err error)
	// SecretsAggregates returns an object that can list and get SecretsAggregates.
	SecretsAggregates(namespace string) SecretsAggregateNamespaceLister
	SecretsAggregateListerExpansion
}

// secretsAggregateLister implements the SecretsAggregateLister interface.
type secretsAggregateLister struct {
	indexer cache.Indexer
}

// NewSecretsAggregateLister returns a new SecretsAggregateLister.
func NewSecretsAggregateLister(indexer cache.Indexer) SecretsAggregateLister {
	return &secretsAggregateLister{indexer: indexer}
}

// List lists all SecretsAggregates in the indexer.
func (s *secretsAggregateLister) List(selector labels.Selector) (ret []*v1alpha1.SecretsAggregate, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.SecretsAggregate))
	})
	return ret, err
}

// SecretsAggregates returns an object that can list and get SecretsAggregates.
func (s *secretsAggregateLister) SecretsAggregates(namespace string) SecretsAggregateNamespaceLister {
	return secretsAggregateNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SecretsAggregateNamespaceLister helps list and get SecretsAggregates.
// All objects returned here must be treated as read-only.
type SecretsAggregateNamespaceLister interface {
	// List lists all SecretsAggregates in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.SecretsAggregate, err error)
	// Get retrieves the SecretsAggregate from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.SecretsAggregate, error)
	SecretsAggregateNamespaceListerExpansion
}

// secretsAggregateNamespaceLister implements the SecretsAggregateNamespaceLister
// interface.
type secretsAggregateNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SecretsAggregates in the indexer for a given namespace.
func (s secretsAggregateNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.SecretsAggregate, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.SecretsAggregate))
	})
	return ret, err
}

// Get retrieves the SecretsAggregate from the indexer for a given namespace and name.
func (s secretsAggregateNamespaceLister) Get(name string) (*v1alpha1.SecretsAggregate, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("secretsaggregate"), name)
	}
	return obj.(*v1alpha1.SecretsAggregate), nil
}