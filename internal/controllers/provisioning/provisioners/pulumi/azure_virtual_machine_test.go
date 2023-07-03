package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestAzureVMDeployFunc(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)
	vm := newVm("my-virtual-machine", platform)
	rg := pulumi.String("my-rg").ToStringOutput()

	t.Run("maximal virtual machine spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			err := azureVirtualMachineDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualMachine{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("virtual machine with nil exports spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := vm.DeepCopy()
			hr.Spec.Exports = nil
			err := azureVirtualMachineDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualMachine{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("virtual machine with trusted launch security", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := vm.DeepCopy()
			hr.Spec.EnableTrustedLaunch = true
			err := azureVirtualMachineDeployFunc(platform, tenant, rg, []*provisioningv1.AzureVirtualMachine{vm})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func newVm(name, platform string) *provisioningv1.AzureVirtualMachine {
	false := false

	hr := provisioningv1.AzureVirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.AzureVirtualMachineSpec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
			},
			VmName:                 "test-vm",
			VmSize:                 "Standard_B1s",
			OSDiskType:             "Standard_LRS",
			SourceImageId:          "/subscriptions/.../my-image/versions/1.0.0",
			SubnetId:               "/subscriptions/.../my-vnet/subnets/default",
			RdpSourceAddressPrefix: "128.12.2.11/24",
			EnableTrustedLaunch:    false,
			Exports: []provisioningv1.AzureVirtualMachineExportsSpec{
				{
					Domain: "myDomain",
					VmName: provisioningv1.ValueExport{
						ToConfigMap: provisioningv1.ConfigMapTemplate{
							KeyTemplate: "MultiTenancy__Tenants__{{ .Tenant.Code }}__CharismaClient_VM_Name",
						},
					},
				},
			},
		},
	}
	return &hr
}
