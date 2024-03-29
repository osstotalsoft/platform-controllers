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

// AzureVirtualMachineLister helps list AzureVirtualMachines.
// All objects returned here must be treated as read-only.
type AzureVirtualMachineLister interface {
	// List lists all AzureVirtualMachines in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.AzureVirtualMachine, err error)
	// AzureVirtualMachines returns an object that can list and get AzureVirtualMachines.
	AzureVirtualMachines(namespace string) AzureVirtualMachineNamespaceLister
	AzureVirtualMachineListerExpansion
}

// azureVirtualMachineLister implements the AzureVirtualMachineLister interface.
type azureVirtualMachineLister struct {
	indexer cache.Indexer
}

// NewAzureVirtualMachineLister returns a new AzureVirtualMachineLister.
func NewAzureVirtualMachineLister(indexer cache.Indexer) AzureVirtualMachineLister {
	return &azureVirtualMachineLister{indexer: indexer}
}

// List lists all AzureVirtualMachines in the indexer.
func (s *azureVirtualMachineLister) List(selector labels.Selector) (ret []*v1alpha1.AzureVirtualMachine, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.AzureVirtualMachine))
	})
	return ret, err
}

// AzureVirtualMachines returns an object that can list and get AzureVirtualMachines.
func (s *azureVirtualMachineLister) AzureVirtualMachines(namespace string) AzureVirtualMachineNamespaceLister {
	return azureVirtualMachineNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// AzureVirtualMachineNamespaceLister helps list and get AzureVirtualMachines.
// All objects returned here must be treated as read-only.
type AzureVirtualMachineNamespaceLister interface {
	// List lists all AzureVirtualMachines in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.AzureVirtualMachine, err error)
	// Get retrieves the AzureVirtualMachine from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.AzureVirtualMachine, error)
	AzureVirtualMachineNamespaceListerExpansion
}

// azureVirtualMachineNamespaceLister implements the AzureVirtualMachineNamespaceLister
// interface.
type azureVirtualMachineNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all AzureVirtualMachines in the indexer for a given namespace.
func (s azureVirtualMachineNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.AzureVirtualMachine, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.AzureVirtualMachine))
	})
	return ret, err
}

// Get retrieves the AzureVirtualMachine from the indexer for a given namespace and name.
func (s azureVirtualMachineNamespaceLister) Get(name string) (*v1alpha1.AzureVirtualMachine, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("azurevirtualmachine"), name)
	}
	return obj.(*v1alpha1.AzureVirtualMachine), nil
}
