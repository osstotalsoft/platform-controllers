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

// FakeAzurePowerShellScripts implements AzurePowerShellScriptInterface
type FakeAzurePowerShellScripts struct {
	Fake *FakeProvisioningV1alpha1
	ns   string
}

var azurepowershellscriptsResource = v1alpha1.SchemeGroupVersion.WithResource("azurepowershellscripts")

var azurepowershellscriptsKind = v1alpha1.SchemeGroupVersion.WithKind("AzurePowerShellScript")

// Get takes name of the azurePowerShellScript, and returns the corresponding azurePowerShellScript object, and an error if there is any.
func (c *FakeAzurePowerShellScripts) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AzurePowerShellScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(azurepowershellscriptsResource, c.ns, name), &v1alpha1.AzurePowerShellScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzurePowerShellScript), err
}

// List takes label and field selectors, and returns the list of AzurePowerShellScripts that match those selectors.
func (c *FakeAzurePowerShellScripts) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AzurePowerShellScriptList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(azurepowershellscriptsResource, azurepowershellscriptsKind, c.ns, opts), &v1alpha1.AzurePowerShellScriptList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AzurePowerShellScriptList{ListMeta: obj.(*v1alpha1.AzurePowerShellScriptList).ListMeta}
	for _, item := range obj.(*v1alpha1.AzurePowerShellScriptList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested azurePowerShellScripts.
func (c *FakeAzurePowerShellScripts) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(azurepowershellscriptsResource, c.ns, opts))

}

// Create takes the representation of a azurePowerShellScript and creates it.  Returns the server's representation of the azurePowerShellScript, and an error, if there is any.
func (c *FakeAzurePowerShellScripts) Create(ctx context.Context, azurePowerShellScript *v1alpha1.AzurePowerShellScript, opts v1.CreateOptions) (result *v1alpha1.AzurePowerShellScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(azurepowershellscriptsResource, c.ns, azurePowerShellScript), &v1alpha1.AzurePowerShellScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzurePowerShellScript), err
}

// Update takes the representation of a azurePowerShellScript and updates it. Returns the server's representation of the azurePowerShellScript, and an error, if there is any.
func (c *FakeAzurePowerShellScripts) Update(ctx context.Context, azurePowerShellScript *v1alpha1.AzurePowerShellScript, opts v1.UpdateOptions) (result *v1alpha1.AzurePowerShellScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(azurepowershellscriptsResource, c.ns, azurePowerShellScript), &v1alpha1.AzurePowerShellScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzurePowerShellScript), err
}

// Delete takes name of the azurePowerShellScript and deletes it. Returns an error if one occurs.
func (c *FakeAzurePowerShellScripts) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(azurepowershellscriptsResource, c.ns, name, opts), &v1alpha1.AzurePowerShellScript{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAzurePowerShellScripts) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(azurepowershellscriptsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.AzurePowerShellScriptList{})
	return err
}

// Patch applies the patch and returns the patched azurePowerShellScript.
func (c *FakeAzurePowerShellScripts) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AzurePowerShellScript, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(azurepowershellscriptsResource, c.ns, name, pt, data, subresources...), &v1alpha1.AzurePowerShellScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzurePowerShellScript), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied azurePowerShellScript.
func (c *FakeAzurePowerShellScripts) Apply(ctx context.Context, azurePowerShellScript *provisioningv1alpha1.AzurePowerShellScriptApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.AzurePowerShellScript, err error) {
	if azurePowerShellScript == nil {
		return nil, fmt.Errorf("azurePowerShellScript provided to Apply must not be nil")
	}
	data, err := json.Marshal(azurePowerShellScript)
	if err != nil {
		return nil, err
	}
	name := azurePowerShellScript.Name
	if name == nil {
		return nil, fmt.Errorf("azurePowerShellScript.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(azurepowershellscriptsResource, c.ns, *name, types.ApplyPatchType, data), &v1alpha1.AzurePowerShellScript{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzurePowerShellScript), err
}