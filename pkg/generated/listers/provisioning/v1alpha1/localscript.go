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

// LocalScriptLister helps list LocalScripts.
// All objects returned here must be treated as read-only.
type LocalScriptLister interface {
	// List lists all LocalScripts in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.LocalScript, err error)
	// LocalScripts returns an object that can list and get LocalScripts.
	LocalScripts(namespace string) LocalScriptNamespaceLister
	LocalScriptListerExpansion
}

// localScriptLister implements the LocalScriptLister interface.
type localScriptLister struct {
	indexer cache.Indexer
}

// NewLocalScriptLister returns a new LocalScriptLister.
func NewLocalScriptLister(indexer cache.Indexer) LocalScriptLister {
	return &localScriptLister{indexer: indexer}
}

// List lists all LocalScripts in the indexer.
func (s *localScriptLister) List(selector labels.Selector) (ret []*v1alpha1.LocalScript, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.LocalScript))
	})
	return ret, err
}

// LocalScripts returns an object that can list and get LocalScripts.
func (s *localScriptLister) LocalScripts(namespace string) LocalScriptNamespaceLister {
	return localScriptNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// LocalScriptNamespaceLister helps list and get LocalScripts.
// All objects returned here must be treated as read-only.
type LocalScriptNamespaceLister interface {
	// List lists all LocalScripts in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.LocalScript, err error)
	// Get retrieves the LocalScript from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.LocalScript, error)
	LocalScriptNamespaceListerExpansion
}

// localScriptNamespaceLister implements the LocalScriptNamespaceLister
// interface.
type localScriptNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all LocalScripts in the indexer for a given namespace.
func (s localScriptNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.LocalScript, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.LocalScript))
	})
	return ret, err
}

// Get retrieves the LocalScript from the indexer for a given namespace and name.
func (s localScriptNamespaceLister) Get(name string) (*v1alpha1.LocalScript, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("localscript"), name)
	}
	return obj.(*v1alpha1.LocalScript), nil
}