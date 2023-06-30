package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestAzureVirtualDesktopDeployFunc(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)
	vm := newVirtualDesktop("my-virtual-Desktop", platform)
	rg := pulumi.String("my-rg").ToStringOutput()

	t.Run("maximal virtual Desktop spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			err := azureVirtualDesktopDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualDesktop{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("virtual Desktop with nil exports spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := vm.DeepCopy()
			hr.Spec.Exports = nil
			err := azureVirtualDesktopDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualDesktop{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("virtual Desktop with trusted launch security", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := vm.DeepCopy()
			hr.Spec.EnableTrustedLaunch = true
			err := azureVirtualDesktopDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualDesktop{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func newVirtualDesktop(name, platform string) *provisioningv1.AzureVirtualDesktop {
	false := false

	hr := provisioningv1.AzureVirtualDesktop{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureVirtualDesktopSpec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
			},
			HostPoolName:        "test-vm",
			VmSize:              "Standard_B1s",
			OSDiskType:          "Standard_LRS",
			SourceImageId:       "/subscriptions/.../my-image/versions/1.0.0",
			SubnetId:            "/subscriptions/.../my-vnet/subnets/default",
			EnableTrustedLaunch: false,
			Exports: []provisioningv1.AzureVirtualDesktopExportsSpec{
				{
					Domain: "myDomain",
					HostPoolName: provisioningv1.ValueExport{
						ToConfigMap: provisioningv1.ConfigMapTemplate{
							KeyTemplate: "MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_HostPool_Name",
						},
					},
				},
			},
		},
	}
	return &hr
}
