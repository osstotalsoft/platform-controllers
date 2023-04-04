package pulumi

import (
	"fmt"
	"strings"

	compute "github.com/pulumi/pulumi-azure-native/sdk/go/azure/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureVirtualMachineSnapshotDeployFunc(platform string, tenant *platformv1.Tenant, resourceGroupName pulumi.StringOutput,
	azureVms []*provisioningv1.AzureVirtualMachine) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, tenant)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureVirtualMachine")
	return func(ctx *pulumi.Context) error {
		for _, vmSpec := range azureVms {
			vmName := strings.ReplaceAll(fmt.Sprintf("%s-%s-%s", vmSpec.Spec.VmName, platform, tenant.Name), ".", "-")
			osDiskName := fmt.Sprintf("%s-os-disk", vmName)

			ignoreChanges := []string{}
			if PulumiRetainOnDelete {
				// TODO: analyze for vm
				ignoreChanges = []string{"resourceGroupName", "creationData.sourceResourceId"}
			}

			disk, err := compute.NewDisk(ctx, osDiskName, &compute.DiskArgs{
				ResourceGroupName: resourceGroupName,
				CreationData: compute.CreationDataArgs{
					CreateOption:     pulumi.String("Copy"),
					SourceResourceId: pulumi.String(vmSpec.Spec.SourceImageId),
				},
			},
				pulumi.RetainOnDelete(PulumiRetainOnDelete),
				pulumi.IgnoreChanges(ignoreChanges),
			)

			if err != nil {
				return err
			}

			args := compute.VirtualMachineArgs{
				ResourceGroupName: resourceGroupName,
				HardwareProfile: compute.HardwareProfileArgs{
					VmSize: pulumi.String(vmSpec.Spec.VmSize),
				},
				// OsProfile: compute.OSProfileArgs{
				// 	ComputerName: pulumi.String(vmName),
				// },
				NetworkProfile: compute.NetworkProfileArgs{
					// NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
					// 	compute.NetworkInterfaceReferenceArgs{
					// 		//TODO: extract to config
					// 		Id: pulumi.String("/subscriptions/05a50a12-6628-4627-bd30-19932dac39f8/resourceGroups/Provisioning_Test/providers/Microsoft.Network/networkInterfaces/myvm2200"),
					// 	},
					// },
					NetworkApiVersion: pulumi.String(compute.NetworkApiVersion_2020_11_01),
					NetworkInterfaceConfigurations: compute.VirtualMachineNetworkInterfaceConfigurationArray{
						compute.VirtualMachineNetworkInterfaceConfigurationArgs{
							Name: pulumi.String(fmt.Sprintf("%s-net-if", vmName)),
							NetworkSecurityGroup: compute.SubResourceArgs{
								Id: pulumi.String(vmSpec.Spec.NetworkSecurityGroupId),
							},
							Primary: pulumi.BoolPtr(true),
							IpConfigurations: compute.VirtualMachineNetworkInterfaceIPConfigurationArray{
								compute.VirtualMachineNetworkInterfaceIPConfigurationArgs{
									Name:    pulumi.String(fmt.Sprintf("%s-ip", vmName)),
									Primary: pulumi.BoolPtr(true),
									Subnet: compute.SubResourceArgs{
										Id: pulumi.String(vmSpec.Spec.SubnetId),
									},
									PublicIPAddressConfiguration: compute.VirtualMachinePublicIPAddressConfigurationArgs{
										DnsSettings: compute.VirtualMachinePublicIPAddressDnsSettingsConfigurationArgs{
											DomainNameLabel: pulumi.String(vmName),
										},
										Name: pulumi.String(fmt.Sprintf("%s-public-ip", vmName)),
									},
								},
							},
						},
					},
				},
				SecurityProfile: compute.SecurityProfileArgs{
					SecurityType: pulumi.String(compute.SecurityTypesTrustedLaunch),
					UefiSettings: compute.UefiSettingsArgs{
						SecureBootEnabled: pulumi.BoolPtr(true),
						VTpmEnabled:       pulumi.BoolPtr(true),
					},
				},
				StorageProfile: compute.StorageProfileArgs{
					OsDisk: compute.OSDiskArgs{
						Name:         disk.Name,
						CreateOption: pulumi.String(compute.DiskCreateOptionAttach),
						ManagedDisk: compute.ManagedDiskParametersArgs{
							Id: disk.ID(),
						},
						//OsType: compute.OperatingSystemTypesPtr(vmSpec.Spec.OsType),
					},
				},
			}

			vm, err := compute.NewVirtualMachine(ctx, vmName, &args)
			if err != nil {
				return err
			}

			for _, exp := range vmSpec.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, vmSpec.Name, vmSpec.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{"vmName": {exp.VmName, vm.Name}})
				if err != nil {
					return err
				}
			}
			ctx.Export(fmt.Sprintf("azureVirtualmachine:%s", vmSpec.Spec.VmName), vm.Name)
		}
		return nil
	}
	return nil
}
