package pulumi

import (
	"fmt"
	"math"
	"strings"

	compute "github.com/pulumi/pulumi-azure-native/sdk/go/azure/compute"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/network"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureVirtualMachineDeployFunc(platform string, tenant *platformv1.Tenant,
	azureVms []*provisioningv1.AzureVirtualMachine) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, tenant)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureVirtualMachine")
	return func(ctx *pulumi.Context) error {
		for _, vmSpec := range azureVms {
			vmName := strings.ReplaceAll(fmt.Sprintf("%s-%s-%s", vmSpec.Spec.VmName, platform, tenant.Name), ".", "-")
			// computerNameOutput, err := random.NewRandomId(ctx, "computerNameRandomId", &random.RandomIdArgs{
			// 	ByteLength: pulumi.Int(8),
			// })
			// computerNameOutput, err := random.NewRandomString(ctx, "computerNameRandom", &random.RandomStringArgs{
			// 	Length:  pulumi.Int(15),
			// 	Special: pulumi.Bool(false),
			// 	Numeric: pulumi.Bool(false),
			// 	Lower:   pulumi.Bool(true),
			// 	Upper:   pulumi.Bool(false),
			// })
			computerNameOutput, err := random.NewRandomPet(ctx, "computerName", &random.RandomPetArgs{
				Length:    pulumi.Int(2),
				Separator: pulumi.String("-"),
			})

			if err != nil {
				return err
			}

			computerName := computerNameOutput.ID().ToStringOutput().ApplyT(func(name string) (string, error) {
				return name[:int32(math.Min(float64(len(name)), 15))], nil
			}).(pulumi.StringOutput)

			password, err := random.NewRandomPassword(ctx, "password", &random.RandomPasswordArgs{
				Length:     pulumi.Int(10),
				Upper:      pulumi.Bool(true),
				MinUpper:   pulumi.Int(1),
				Lower:      pulumi.Bool(true),
				MinLower:   pulumi.Int(1),
				Numeric:    pulumi.Bool(true),
				MinNumeric: pulumi.Int(1),
				Special:    pulumi.Bool(true),
				MinSpecial: pulumi.Int(1),
			})

			if err != nil {
				return err
			}

			publicIP, err := network.NewPublicIPAddress(ctx, fmt.Sprintf("%s-public-ip", vmName), &network.PublicIPAddressArgs{
				ResourceGroupName: pulumi.String(vmSpec.Spec.ResourceGroup),
				DnsSettings: network.PublicIPAddressDnsSettingsArgs{
					DomainNameLabel: pulumi.String(vmName),
				},
			})
			if err != nil {
				return err
			}

			networkInterface, err := network.NewNetworkInterface(ctx, fmt.Sprintf("%s-net-if", vmName), &network.NetworkInterfaceArgs{
				ResourceGroupName: pulumi.String(vmSpec.Spec.ResourceGroup),
				IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
					network.NetworkInterfaceIPConfigurationArgs{
						Name: pulumi.String(fmt.Sprintf("%s-net-if", vmName)),
						Subnet: network.SubnetTypeArgs{
							Id: pulumi.String(vmSpec.Spec.SubnetId),
						},

						PublicIPAddress: network.PublicIPAddressTypeArgs{
							Id: publicIP.ID(),
						},
					},
				},
				NetworkSecurityGroup: network.NetworkSecurityGroupTypeArgs{
					Id: pulumi.String(vmSpec.Spec.NetworkSecurityGroupId),
				},
			})
			if err != nil {
				return err
			}

			args := compute.VirtualMachineArgs{
				VmName:            pulumi.String(vmName),
				ResourceGroupName: pulumi.String(vmSpec.Spec.ResourceGroup),
				HardwareProfile: compute.HardwareProfileArgs{
					VmSize: pulumi.String(vmSpec.Spec.VmSize),
				},
				OsProfile: compute.OSProfileArgs{
					ComputerName:  computerName,
					AdminUsername: pulumi.String("lfra"),
					AdminPassword: password.Result,
				},
				NetworkProfile: compute.NetworkProfileArgs{
					NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
						compute.NetworkInterfaceReferenceArgs{
							Id:      networkInterface.ID(),
							Primary: pulumi.Bool(true),
							// NetworkSecurityGroup: compute.SubResourceArgs{
							// 	Id: pulumi.String(vmSpec.Spec.NetworkSecurityGroupId),
							// },
						},
					},
				},
				// NetworkProfile: compute.NetworkProfileArgs{
				// 	NetworkApiVersion: pulumi.String(compute.NetworkApiVersion_2020_11_01),
				// 	NetworkInterfaceConfigurations: compute.VirtualMachineNetworkInterfaceConfigurationArray{
				// 		compute.VirtualMachineNetworkInterfaceConfigurationArgs{
				// 			Name: pulumi.String(fmt.Sprintf("%s-net-if", vmName)),
				// 			NetworkSecurityGroup: compute.SubResourceArgs{
				// 				Id: pulumi.String(vmSpec.Spec.NetworkSecurityGroupId),
				// 			},
				// 			Primary: pulumi.BoolPtr(true),
				// 			IpConfigurations: compute.VirtualMachineNetworkInterfaceIPConfigurationArray{
				// 				compute.VirtualMachineNetworkInterfaceIPConfigurationArgs{
				// 					Name:    pulumi.String(fmt.Sprintf("%s-ip", vmName)),
				// 					Primary: pulumi.BoolPtr(true),
				// 					Subnet: compute.SubResourceArgs{
				// 						Id: pulumi.String(vmSpec.Spec.SubnetId),
				// 					},
				// 					PublicIPAddressConfiguration: compute.VirtualMachinePublicIPAddressConfigurationArgs{
				// 						DnsSettings: compute.VirtualMachinePublicIPAddressDnsSettingsConfigurationArgs{
				// 							DomainNameLabel: pulumi.String(vmName),
				// 						},
				// 						Name: pulumi.String(fmt.Sprintf("%s-public-ip", vmName)),
				// 					},
				// 				},
				// 			},
				// 		},
				// 	},
				// },
				// SecurityProfile: compute.SecurityProfileArgs{
				// 	SecurityType: pulumi.String(compute.SecurityTypesTrustedLaunch),
				// 	UefiSettings: compute.UefiSettingsArgs{
				// 		SecureBootEnabled: pulumi.BoolPtr(true),
				// 		VTpmEnabled:       pulumi.BoolPtr(true),
				// 	},
				// },
				StorageProfile: compute.StorageProfileArgs{
					ImageReference: compute.ImageReferenceArgs{
						Id: pulumi.String(vmSpec.Spec.SourceSnapshotId),
					},
					OsDisk: compute.OSDiskArgs{
						Name:         pulumi.String(fmt.Sprintf("%s-os-disk", vmName)),
						CreateOption: pulumi.String(compute.DiskCreateOptionFromImage),
						//OsType:       compute.OperatingSystemTypesPtr(vmSpec.Spec.OsType),
					},
				},
			}

			vm, err := compute.NewVirtualMachine(ctx, vmName, &args)
			if err != nil {
				return err
			}

			for _, exp := range vmSpec.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, vmSpec.Name, vmSpec.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{
						"vmName":        {exp.VmName, vm.Name},
						"computerName":  {exp.ComputerName, computerName},
						"publicAddress": {exp.PublicAddress, publicIP.DnsSettings.Fqdn().Elem()}})
				if err != nil {
					return err
				}
			}
			ctx.Export(fmt.Sprintf("azureVirtualmachine:%s", vmSpec.Spec.VmName), vm.Name)
			ctx.Export(fmt.Sprintf("azureVirtualmachineAdminUserName:%s", vmSpec.Spec.VmName), vm.OsProfile.AdminUsername())
			ctx.Export(fmt.Sprintf("azureVirtualmachineAdminPassword:%s", vmSpec.Spec.VmName), password.Result)
		}
		return nil
	}
}
