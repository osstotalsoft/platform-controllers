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
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
)

// FakeConfigurationAggregates implements ConfigurationAggregateInterface
type FakeConfigurationAggregates struct {
	Fake *FakeConfigurationV1alpha1
	ns   string
}

var configurationaggregatesResource = schema.GroupVersionResource{Group: "configuration.totalsoft.ro", Version: "v1alpha1", Resource: "configurationaggregates"}

var configurationaggregatesKind = schema.GroupVersionKind{Group: "configuration.totalsoft.ro", Version: "v1alpha1", Kind: "ConfigurationAggregate"}

// Get takes name of the configurationAggregate, and returns the corresponding configurationAggregate object, and an error if there is any.
func (c *FakeConfigurationAggregates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ConfigurationAggregate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(configurationaggregatesResource, c.ns, name), &v1alpha1.ConfigurationAggregate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ConfigurationAggregate), err
}

// List takes label and field selectors, and returns the list of ConfigurationAggregates that match those selectors.
func (c *FakeConfigurationAggregates) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ConfigurationAggregateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(configurationaggregatesResource, configurationaggregatesKind, c.ns, opts), &v1alpha1.ConfigurationAggregateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ConfigurationAggregateList{ListMeta: obj.(*v1alpha1.ConfigurationAggregateList).ListMeta}
	for _, item := range obj.(*v1alpha1.ConfigurationAggregateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested configurationAggregates.
func (c *FakeConfigurationAggregates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(configurationaggregatesResource, c.ns, opts))

}

// Create takes the representation of a configurationAggregate and creates it.  Returns the server's representation of the configurationAggregate, and an error, if there is any.
func (c *FakeConfigurationAggregates) Create(ctx context.Context, configurationAggregate *v1alpha1.ConfigurationAggregate, opts v1.CreateOptions) (result *v1alpha1.ConfigurationAggregate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(configurationaggregatesResource, c.ns, configurationAggregate), &v1alpha1.ConfigurationAggregate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ConfigurationAggregate), err
}

// Update takes the representation of a configurationAggregate and updates it. Returns the server's representation of the configurationAggregate, and an error, if there is any.
func (c *FakeConfigurationAggregates) Update(ctx context.Context, configurationAggregate *v1alpha1.ConfigurationAggregate, opts v1.UpdateOptions) (result *v1alpha1.ConfigurationAggregate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(configurationaggregatesResource, c.ns, configurationAggregate), &v1alpha1.ConfigurationAggregate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ConfigurationAggregate), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeConfigurationAggregates) UpdateStatus(ctx context.Context, configurationAggregate *v1alpha1.ConfigurationAggregate, opts v1.UpdateOptions) (*v1alpha1.ConfigurationAggregate, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(configurationaggregatesResource, "status", c.ns, configurationAggregate), &v1alpha1.ConfigurationAggregate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ConfigurationAggregate), err
}

// Delete takes name of the configurationAggregate and deletes it. Returns an error if one occurs.
func (c *FakeConfigurationAggregates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(configurationaggregatesResource, c.ns, name, opts), &v1alpha1.ConfigurationAggregate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeConfigurationAggregates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(configurationaggregatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ConfigurationAggregateList{})
	return err
}

// Patch applies the patch and returns the patched configurationAggregate.
func (c *FakeConfigurationAggregates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ConfigurationAggregate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(configurationaggregatesResource, c.ns, name, pt, data, subresources...), &v1alpha1.ConfigurationAggregate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ConfigurationAggregate), err
}
