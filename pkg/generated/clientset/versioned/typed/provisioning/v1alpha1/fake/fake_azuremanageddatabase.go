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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// FakeAzureManagedDatabases implements AzureManagedDatabaseInterface
type FakeAzureManagedDatabases struct {
	Fake *FakeProvisioningV1alpha1
	ns   string
}

var azuremanageddatabasesResource = schema.GroupVersionResource{Group: "provisioning.totalsoft.ro", Version: "v1alpha1", Resource: "azuremanageddatabases"}

var azuremanageddatabasesKind = schema.GroupVersionKind{Group: "provisioning.totalsoft.ro", Version: "v1alpha1", Kind: "AzureManagedDatabase"}

// Get takes name of the azureManagedDatabase, and returns the corresponding azureManagedDatabase object, and an error if there is any.
func (c *FakeAzureManagedDatabases) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AzureManagedDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(azuremanageddatabasesResource, c.ns, name), &v1alpha1.AzureManagedDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureManagedDatabase), err
}

// List takes label and field selectors, and returns the list of AzureManagedDatabases that match those selectors.
func (c *FakeAzureManagedDatabases) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AzureManagedDatabaseList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(azuremanageddatabasesResource, azuremanageddatabasesKind, c.ns, opts), &v1alpha1.AzureManagedDatabaseList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AzureManagedDatabaseList{ListMeta: obj.(*v1alpha1.AzureManagedDatabaseList).ListMeta}
	for _, item := range obj.(*v1alpha1.AzureManagedDatabaseList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested azureManagedDatabases.
func (c *FakeAzureManagedDatabases) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(azuremanageddatabasesResource, c.ns, opts))

}

// Create takes the representation of a azureManagedDatabase and creates it.  Returns the server's representation of the azureManagedDatabase, and an error, if there is any.
func (c *FakeAzureManagedDatabases) Create(ctx context.Context, azureManagedDatabase *v1alpha1.AzureManagedDatabase, opts v1.CreateOptions) (result *v1alpha1.AzureManagedDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(azuremanageddatabasesResource, c.ns, azureManagedDatabase), &v1alpha1.AzureManagedDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureManagedDatabase), err
}

// Update takes the representation of a azureManagedDatabase and updates it. Returns the server's representation of the azureManagedDatabase, and an error, if there is any.
func (c *FakeAzureManagedDatabases) Update(ctx context.Context, azureManagedDatabase *v1alpha1.AzureManagedDatabase, opts v1.UpdateOptions) (result *v1alpha1.AzureManagedDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(azuremanageddatabasesResource, c.ns, azureManagedDatabase), &v1alpha1.AzureManagedDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureManagedDatabase), err
}

// Delete takes name of the azureManagedDatabase and deletes it. Returns an error if one occurs.
func (c *FakeAzureManagedDatabases) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(azuremanageddatabasesResource, c.ns, name, opts), &v1alpha1.AzureManagedDatabase{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAzureManagedDatabases) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(azuremanageddatabasesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.AzureManagedDatabaseList{})
	return err
}

// Patch applies the patch and returns the patched azureManagedDatabase.
func (c *FakeAzureManagedDatabases) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AzureManagedDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(azuremanageddatabasesResource, c.ns, name, pt, data, subresources...), &v1alpha1.AzureManagedDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureManagedDatabase), err
}
