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

// FakeLocalScripts implements LocalScriptInterface
type FakeLocalScripts struct {
	Fake *FakeProvisioningV1alpha1
	ns   string
}

var localscriptsResource = v1alpha1.SchemeGroupVersion.WithResource("localscripts")

var localscriptsKind = v1alpha1.SchemeGroupVersion.WithKind("LocalScript")

// Get takes name of the localScript, and returns the corresponding localScript object, and an error if there is any.
func (c *FakeLocalScripts) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.LocalScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(localscriptsResource, c.ns, name), &v1alpha1.LocalScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.LocalScript), err
}

// List takes label and field selectors, and returns the list of LocalScripts that match those selectors.
func (c *FakeLocalScripts) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.LocalScriptList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(localscriptsResource, localscriptsKind, c.ns, opts), &v1alpha1.LocalScriptList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.LocalScriptList{ListMeta: obj.(*v1alpha1.LocalScriptList).ListMeta}
	for _, item := range obj.(*v1alpha1.LocalScriptList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested localScripts.
func (c *FakeLocalScripts) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(localscriptsResource, c.ns, opts))

}

// Create takes the representation of a localScript and creates it.  Returns the server's representation of the localScript, and an error, if there is any.
func (c *FakeLocalScripts) Create(ctx context.Context, localScript *v1alpha1.LocalScript, opts v1.CreateOptions) (result *v1alpha1.LocalScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(localscriptsResource, c.ns, localScript), &v1alpha1.LocalScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.LocalScript), err
}

// Update takes the representation of a localScript and updates it. Returns the server's representation of the localScript, and an error, if there is any.
func (c *FakeLocalScripts) Update(ctx context.Context, localScript *v1alpha1.LocalScript, opts v1.UpdateOptions) (result *v1alpha1.LocalScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(localscriptsResource, c.ns, localScript), &v1alpha1.LocalScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.LocalScript), err
}

// Delete takes name of the localScript and deletes it. Returns an error if one occurs.
func (c *FakeLocalScripts) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(localscriptsResource, c.ns, name, opts), &v1alpha1.LocalScript{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeLocalScripts) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(localscriptsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.LocalScriptList{})
	return err
}

// Patch applies the patch and returns the patched localScript.
func (c *FakeLocalScripts) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.LocalScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(localscriptsResource, c.ns, name, pt, data, subresources...), &v1alpha1.LocalScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.LocalScript), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied localScript.
func (c *FakeLocalScripts) Apply(ctx context.Context, localScript *provisioningv1alpha1.LocalScriptApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.LocalScript, err error) {
	if localScript == nil {
		return nil, fmt.Errorf("localScript provided to Apply must not be nil")
	}
	data, err := json.Marshal(localScript)
	if err != nil {
		return nil, err
	}
	name := localScript.Name
	if name == nil {
		return nil, fmt.Errorf("localScript.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(localscriptsResource, c.ns, *name, types.ApplyPatchType, data), &v1alpha1.LocalScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.LocalScript), err
}
