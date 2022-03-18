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

// FakeTenants implements TenantInterface
type FakeTenants struct {
	Fake *FakeProvisioningV1alpha1
	ns   string
}

var tenantsResource = schema.GroupVersionResource{Group: "provisioning.totalsoft.ro", Version: "v1alpha1", Resource: "tenants"}

var tenantsKind = schema.GroupVersionKind{Group: "provisioning.totalsoft.ro", Version: "v1alpha1", Kind: "Tenant"}

// Get takes name of the tenant, and returns the corresponding tenant object, and an error if there is any.
func (c *FakeTenants) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Tenant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(tenantsResource, c.ns, name), &v1alpha1.Tenant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Tenant), err
}

// List takes label and field selectors, and returns the list of Tenants that match those selectors.
func (c *FakeTenants) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.TenantList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(tenantsResource, tenantsKind, c.ns, opts), &v1alpha1.TenantList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.TenantList{ListMeta: obj.(*v1alpha1.TenantList).ListMeta}
	for _, item := range obj.(*v1alpha1.TenantList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested tenants.
func (c *FakeTenants) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(tenantsResource, c.ns, opts))

}

// Create takes the representation of a tenant and creates it.  Returns the server's representation of the tenant, and an error, if there is any.
func (c *FakeTenants) Create(ctx context.Context, tenant *v1alpha1.Tenant, opts v1.CreateOptions) (result *v1alpha1.Tenant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(tenantsResource, c.ns, tenant), &v1alpha1.Tenant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Tenant), err
}

// Update takes the representation of a tenant and updates it. Returns the server's representation of the tenant, and an error, if there is any.
func (c *FakeTenants) Update(ctx context.Context, tenant *v1alpha1.Tenant, opts v1.UpdateOptions) (result *v1alpha1.Tenant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(tenantsResource, c.ns, tenant), &v1alpha1.Tenant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Tenant), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeTenants) UpdateStatus(ctx context.Context, tenant *v1alpha1.Tenant, opts v1.UpdateOptions) (*v1alpha1.Tenant, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(tenantsResource, "status", c.ns, tenant), &v1alpha1.Tenant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Tenant), err
}

// Delete takes name of the tenant and deletes it. Returns an error if one occurs.
func (c *FakeTenants) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(tenantsResource, c.ns, name, opts), &v1alpha1.Tenant{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTenants) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(tenantsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.TenantList{})
	return err
}

// Patch applies the patch and returns the patched tenant.
func (c *FakeTenants) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Tenant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(tenantsResource, c.ns, name, pt, data, subresources...), &v1alpha1.Tenant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Tenant), err
}
