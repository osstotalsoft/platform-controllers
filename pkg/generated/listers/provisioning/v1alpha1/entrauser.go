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
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// EntraUserLister helps list EntraUsers.
// All objects returned here must be treated as read-only.
type EntraUserLister interface {
	// List lists all EntraUsers in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.EntraUser, err error)
	// EntraUsers returns an object that can list and get EntraUsers.
	EntraUsers(namespace string) EntraUserNamespaceLister
	EntraUserListerExpansion
}

// entraUserLister implements the EntraUserLister interface.
type entraUserLister struct {
	indexer cache.Indexer
}

// NewEntraUserLister returns a new EntraUserLister.
func NewEntraUserLister(indexer cache.Indexer) EntraUserLister {
	return &entraUserLister{indexer: indexer}
}

// List lists all EntraUsers in the indexer.
func (s *entraUserLister) List(selector labels.Selector) (ret []*v1alpha1.EntraUser, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.EntraUser))
	})
	return ret, err
}

// EntraUsers returns an object that can list and get EntraUsers.
func (s *entraUserLister) EntraUsers(namespace string) EntraUserNamespaceLister {
	return entraUserNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// EntraUserNamespaceLister helps list and get EntraUsers.
// All objects returned here must be treated as read-only.
type EntraUserNamespaceLister interface {
	// List lists all EntraUsers in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.EntraUser, err error)
	// Get retrieves the EntraUser from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.EntraUser, error)
	EntraUserNamespaceListerExpansion
}

// entraUserNamespaceLister implements the EntraUserNamespaceLister
// interface.
type entraUserNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all EntraUsers in the indexer for a given namespace.
func (s entraUserNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.EntraUser, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.EntraUser))
	})
	return ret, err
}

// Get retrieves the EntraUser from the indexer for a given namespace and name.
func (s entraUserNamespaceLister) Get(name string) (*v1alpha1.EntraUser, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("entrauser"), name)
	}
	return obj.(*v1alpha1.EntraUser), nil
}
