package pulumi

import (
	"fmt"
	"math"
	"strings"

	compute "github.com/pulumi/pulumi-azure-native-sdk/compute/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v2"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func azureVirtualMachineDeployFunc[T provisioning.ProvisioningTarget](platform string, target T, resourceGroupName pulumi.StringOutput,
	azureVms []*provisioningv1.AzureVirtualMachine) pulumi.RunFunc {

	valueExporter := handleValueExport(platform, target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureVirtualMachine")
	return func(ctx *pulumi.Context) error {
		for _, azureVM := range azureVms {
			vmName := strings.ReplaceAll(fmt.Sprintf("%s-%s-%s", azureVM.Spec.VmName, platform, target.GetName()), ".", "-")

			computerNameOutput, err := random.NewRandomPet(ctx, fmt.Sprintf("%s-computer-name", vmName), &random.RandomPetArgs{
				Length:    pulumi.Int(2),
				Separator: pulumi.String("-"),
			})

			if err != nil {
				return err
			}

			computerName := computerNameOutput.ID().ToStringOutput().ApplyT(func(name string) (string, error) {
				return name[:int32(math.Min(float64(len(name)), 15))], nil
			}).(pulumi.StringOutput)

			password, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-admin-password", vmName), &random.RandomPasswordArgs{
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
				ResourceGroupName: resourceGroupName,
				DnsSettings: network.PublicIPAddressDnsSettingsArgs{
					DomainNameLabel: pulumi.String(vmName),
				},
			})

			if err != nil {
				return err
			}

			networkSecurityGroup, err := network.NewNetworkSecurityGroup(ctx, fmt.Sprintf("%s-nsg", vmName), &network.NetworkSecurityGroupArgs{
				ResourceGroupName: resourceGroupName,
				SecurityRules: network.SecurityRuleTypeArray{
					network.SecurityRuleTypeArgs{
						Name:                     pulumi.String("RDP"),
						Protocol:                 pulumi.String(network.SecurityRuleProtocolTcp),
						SourceAddressPrefix:      pulumi.String(azureVM.Spec.RdpSourceAddressPrefix),
						SourcePortRange:          pulumi.String("*"),
						DestinationAddressPrefix: pulumi.String("*"),
						DestinationPortRange:     pulumi.String("3389"),
						Access:                   pulumi.String(network.SecurityRuleAccessAllow),
						Priority:                 pulumi.Int(300),
						Direction:                pulumi.String(network.AccessRuleDirectionInbound),
					},
				},
			})

			if err != nil {
				return err
			}

			networkInterface, err := network.NewNetworkInterface(ctx, fmt.Sprintf("%s-net-if", vmName), &network.NetworkInterfaceArgs{
				ResourceGroupName: resourceGroupName,
				IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
					network.NetworkInterfaceIPConfigurationArgs{
						Name: pulumi.String(fmt.Sprintf("%s-net-if", vmName)),
						Subnet: network.SubnetTypeArgs{
							Id: pulumi.String(azureVM.Spec.SubnetId),
						},

						PublicIPAddress: network.PublicIPAddressTypeArgs{
							Id: publicIP.ID(),
						},
					},
				},
				NetworkSecurityGroup: network.NetworkSecurityGroupTypeArgs{
					Id: networkSecurityGroup.ID(),
				},
			})
			if err != nil {
				return err
			}

			args := compute.VirtualMachineArgs{
				VmName:            pulumi.String(vmName),
				ResourceGroupName: resourceGroupName,
				HardwareProfile: compute.HardwareProfileArgs{
					VmSize: pulumi.String(azureVM.Spec.VmSize),
				},
				OsProfile: compute.OSProfileArgs{
					ComputerName:  computerName,
					AdminUsername: pulumi.String(fmt.Sprintf("admin-%s", target.GetName())),
					AdminPassword: password.Result,
				},
				NetworkProfile: compute.NetworkProfileArgs{
					NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
						compute.NetworkInterfaceReferenceArgs{
							Id:      networkInterface.ID(),
							Primary: pulumi.Bool(true),
						},
					},
				},
				StorageProfile: compute.StorageProfileArgs{
					ImageReference: compute.ImageReferenceArgs{
						Id: pulumi.String(azureVM.Spec.SourceImageId),
					},
					OsDisk: compute.OSDiskArgs{
						Name:         pulumi.String(fmt.Sprintf("%s-os-disk", vmName)),
						CreateOption: pulumi.String(compute.DiskCreateOptionFromImage),
						DeleteOption: pulumi.String(compute.DeleteOptionsDetach),
						ManagedDisk: compute.ManagedDiskParametersArgs{
							StorageAccountType: pulumi.String(azureVM.Spec.OSDiskType),
						},
					},
				},
			}

			if azureVM.Spec.EnableTrustedLaunch {
				args.SecurityProfile = compute.SecurityProfileArgs{
					SecurityType: pulumi.String(compute.SecurityTypesTrustedLaunch),
					UefiSettings: compute.UefiSettingsArgs{
						SecureBootEnabled: pulumi.BoolPtr(true),
						VTpmEnabled:       pulumi.BoolPtr(true),
					},
				}
			}

			vm, err := compute.NewVirtualMachine(ctx, vmName, &args)
			if err != nil {
				return err
			}

			for _, exp := range azureVM.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, azureVM.Name, azureVM.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{
						"vmName":        {exp.VmName, vm.Name},
						"computerName":  {exp.ComputerName, computerName},
						"publicAddress": {exp.PublicAddress, publicIP.DnsSettings.Fqdn().Elem()},
						"adminUserName": {exp.AdminUserName, vm.OsProfile.AdminUsername().Elem()},
						"adminPassword": {exp.AdminPassword, password.Result}})
				if err != nil {
					return err
				}
			}

			ctx.Export(fmt.Sprintf("azureVirtualMachine:%s", azureVM.Spec.VmName), vm.Name)
		}
		return nil
	}
}
