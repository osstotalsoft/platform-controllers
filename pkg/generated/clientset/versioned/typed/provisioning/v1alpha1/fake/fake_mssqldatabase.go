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
	json "encoding/json"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	provisioningv1alpha1 "totalsoft.ro/platform-controllers/pkg/generated/applyconfiguration/provisioning/v1alpha1"
)

// FakeMsSqlDatabases implements MsSqlDatabaseInterface
type FakeMsSqlDatabases struct {
	Fake *FakeProvisioningV1alpha1
	ns   string
}

var mssqldatabasesResource = v1alpha1.SchemeGroupVersion.WithResource("mssqldatabases")

var mssqldatabasesKind = v1alpha1.SchemeGroupVersion.WithKind("MsSqlDatabase")

// Get takes name of the msSqlDatabase, and returns the corresponding msSqlDatabase object, and an error if there is any.
func (c *FakeMsSqlDatabases) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.MsSqlDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(mssqldatabasesResource, c.ns, name), &v1alpha1.MsSqlDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MsSqlDatabase), err
}

// List takes label and field selectors, and returns the list of MsSqlDatabases that match those selectors.
func (c *FakeMsSqlDatabases) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MsSqlDatabaseList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(mssqldatabasesResource, mssqldatabasesKind, c.ns, opts), &v1alpha1.MsSqlDatabaseList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MsSqlDatabaseList{ListMeta: obj.(*v1alpha1.MsSqlDatabaseList).ListMeta}
	for _, item := range obj.(*v1alpha1.MsSqlDatabaseList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested msSqlDatabases.
func (c *FakeMsSqlDatabases) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(mssqldatabasesResource, c.ns, opts))

}

// Create takes the representation of a msSqlDatabase and creates it.  Returns the server's representation of the msSqlDatabase, and an error, if there is any.
func (c *FakeMsSqlDatabases) Create(ctx context.Context, msSqlDatabase *v1alpha1.MsSqlDatabase, opts v1.CreateOptions) (result *v1alpha1.MsSqlDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(mssqldatabasesResource, c.ns, msSqlDatabase), &v1alpha1.MsSqlDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MsSqlDatabase), err
}

// Update takes the representation of a msSqlDatabase and updates it. Returns the server's representation of the msSqlDatabase, and an error, if there is any.
func (c *FakeMsSqlDatabases) Update(ctx context.Context, msSqlDatabase *v1alpha1.MsSqlDatabase, opts v1.UpdateOptions) (result *v1alpha1.MsSqlDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(mssqldatabasesResource, c.ns, msSqlDatabase), &v1alpha1.MsSqlDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MsSqlDatabase), err
}

// Delete takes name of the msSqlDatabase and deletes it. Returns an error if one occurs.
func (c *FakeMsSqlDatabases) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(mssqldatabasesResource, c.ns, name, opts), &v1alpha1.MsSqlDatabase{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMsSqlDatabases) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(mssqldatabasesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MsSqlDatabaseList{})
	return err
}

// Patch applies the patch and returns the patched msSqlDatabase.
func (c *FakeMsSqlDatabases) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MsSqlDatabase, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mssqldatabasesResource, c.ns, name, pt, data, subresources...), &v1alpha1.MsSqlDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MsSqlDatabase), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied msSqlDatabase.
func (c *FakeMsSqlDatabases) Apply(ctx context.Context, msSqlDatabase *provisioningv1alpha1.MsSqlDatabaseApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.MsSqlDatabase, err error) {
	if msSqlDatabase == nil {
		return nil, fmt.Errorf("msSqlDatabase provided to Apply must not be nil")
	}
	data, err := json.Marshal(msSqlDatabase)
	if err != nil {
		return nil, err
	}
	name := msSqlDatabase.Name
	if name == nil {
		return nil, fmt.Errorf("msSqlDatabase.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mssqldatabasesResource, c.ns, *name, types.ApplyPatchType, data), &v1alpha1.MsSqlDatabase{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MsSqlDatabase), err
}