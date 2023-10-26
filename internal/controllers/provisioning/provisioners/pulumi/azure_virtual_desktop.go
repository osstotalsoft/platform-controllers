package pulumi

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/compute/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/desktopvirtualization/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type AzureVirtualDesktop struct {
	pulumi.ResourceState

	HostPool                     *desktopvirtualization.HostPool
	Workspace                    *desktopvirtualization.Workspace
	DesktopAppGroup              *desktopvirtualization.ApplicationGroup
	RemoteAppGroup               *desktopvirtualization.ApplicationGroup
	RemoteAppGroupAssignment     *authorization.RoleAssignment
	RemoteDesktopGroupAssignment *authorization.RoleAssignment
}

type AzureVirtualDesktopVM struct {
	pulumi.ResourceState

	VirtualMachine   *compute.VirtualMachine
	NetworkInterface *network.NetworkInterface
	ComputerName     pulumi.StringOutput
	AdminPassword    *random.RandomPassword
}

type AzureVirtualDesktopVMArgs struct {
	// A required username for the VM login.
	TenantName pulumi.StringInput

	HostPoolName pulumi.StringInput

	RegistrationToken pulumi.StringInput

	// An optional VM size; if unspecified, Standard_A0 (micro) will be used.
	VMSize pulumi.StringInput

	// A required Resource Group in which to create the VM
	ResourceGroupName pulumi.StringInput

	// Applications UserGroup ID
	LoginUserGroupId pulumi.StringInput

	// Admin UserGroup ID
	LoginAdminGroupId pulumi.StringInput

	// A required Subnet in which to deploy the VM
	SubnetID pulumi.StringInput

	ProfileFileServer pulumi.StringInput

	ProfileShare pulumi.StringInput

	ProfileUser pulumi.StringInput

	ProfileSecret pulumi.StringInput

	DiskDeleteOptions compute.DeleteOptions

	Spec provisioningv1.AzureVirtualDesktopSpec
}

func NewAzureVirtualDesktopVM(ctx *pulumi.Context, name string, args *AzureVirtualDesktopVMArgs, opts ...pulumi.ResourceOption) (*AzureVirtualDesktopVM, error) {

	avdVM := &AzureVirtualDesktopVM{}

	err := ctx.RegisterComponentResource("ts-azure-comp:azureVirtualDesktop:AzureVirtualDesktopVM", name, avdVM, opts...)
	if err != nil {
		return nil, err
	}

	computerName, err := random.NewRandomPet(ctx, fmt.Sprintf("%s-computer-name", name), &random.RandomPetArgs{
		Length:    pulumi.Int(2),
		Separator: pulumi.String("-"),
	}, pulumi.Parent(avdVM))

	if err != nil {
		return nil, err
	}

	avdVM.ComputerName = computerName.ID().ToStringOutput().ApplyT(func(name string) (string, error) {
		return name[:int32(math.Min(float64(len(name)), 15))], nil
	}).(pulumi.StringOutput)

	avdVM.AdminPassword, err = random.NewRandomPassword(ctx, fmt.Sprintf("%s-admin-password", name), &random.RandomPasswordArgs{
		Length:     pulumi.Int(10),
		Upper:      pulumi.Bool(true),
		MinUpper:   pulumi.Int(1),
		Lower:      pulumi.Bool(true),
		MinLower:   pulumi.Int(1),
		Numeric:    pulumi.Bool(true),
		MinNumeric: pulumi.Int(1),
		Special:    pulumi.Bool(true),
		MinSpecial: pulumi.Int(1),
	}, pulumi.Parent(avdVM))

	if err != nil {
		return nil, err
	}

	avdVM.NetworkInterface, err = network.NewNetworkInterface(ctx, fmt.Sprintf("%s-net-if", name), &network.NetworkInterfaceArgs{
		ResourceGroupName: args.ResourceGroupName,
		IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
			network.NetworkInterfaceIPConfigurationArgs{
				Name: pulumi.String(fmt.Sprintf("%s-net-if", name)),
				Subnet: network.SubnetTypeArgs{
					Id: args.SubnetID,
				},
			},
		},
	}, pulumi.Parent(avdVM))
	if err != nil {
		return nil, err
	}

	vmApplications := compute.VMGalleryApplicationArray{}
	for _, vmApplication := range args.Spec.VmApplications {
		vmApplications = append(vmApplications, compute.VMGalleryApplicationArgs{
			PackageReferenceId: pulumi.String(vmApplication.PackageId),
			Order:              pulumi.Int(vmApplication.InstallOrderIndex),
		})
	}

	vmArgs := compute.VirtualMachineArgs{
		//VmName:            pulumi.String(name),
		ResourceGroupName: args.ResourceGroupName,
		HardwareProfile: compute.HardwareProfileArgs{
			VmSize: args.VMSize,
		},
		Identity: compute.VirtualMachineIdentityArgs{
			Type: compute.ResourceIdentityTypeSystemAssigned,
		},

		OsProfile: compute.OSProfileArgs{
			ComputerName:  avdVM.ComputerName,
			AdminUsername: pulumi.String(fmt.Sprintf("admin-%s", args.TenantName)),
			AdminPassword: avdVM.AdminPassword.Result,
			WindowsConfiguration: compute.WindowsConfigurationArgs{
				EnableAutomaticUpdates: pulumi.Bool(false),
				PatchSettings: compute.PatchSettingsArgs{
					PatchMode: pulumi.String(compute.WindowsVMGuestPatchModeManual),
				},
			},
		},
		NetworkProfile: compute.NetworkProfileArgs{
			NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
				compute.NetworkInterfaceReferenceArgs{
					Id:      avdVM.NetworkInterface.ID(),
					Primary: pulumi.Bool(true),
				},
			},
		},
		StorageProfile: compute.StorageProfileArgs{
			ImageReference: compute.ImageReferenceArgs{
				Id: pulumi.String(args.Spec.SourceImageId),
			},
			OsDisk: compute.OSDiskArgs{
				//Name:         pulumi.String(fmt.Sprintf("%s-os-disk", name)),
				CreateOption: pulumi.String(compute.DiskCreateOptionFromImage),
				DeleteOption: pulumi.String(args.DiskDeleteOptions),
				ManagedDisk: compute.ManagedDiskParametersArgs{
					StorageAccountType: pulumi.String(args.Spec.OSDiskType),
				},
			},
		},
		ApplicationProfile: compute.ApplicationProfileArgs{
			GalleryApplications: vmApplications,
		},
	}

	if args.Spec.EnableTrustedLaunch {
		vmArgs.SecurityProfile = compute.SecurityProfileArgs{
			SecurityType: pulumi.String(compute.SecurityTypesTrustedLaunch),
			UefiSettings: compute.UefiSettingsArgs{
				SecureBootEnabled: pulumi.BoolPtr(true),
				VTpmEnabled:       pulumi.BoolPtr(true),
			},
		}
	}

	avdVM.VirtualMachine, err = compute.NewVirtualMachine(ctx, name, &vmArgs, pulumi.Parent(avdVM))
	if err != nil {
		return nil, err
	}

	vmUserLoginRole, err := authorization.LookupRoleDefinition(ctx, &authorization.LookupRoleDefinitionArgs{
		RoleDefinitionId: "fb879df8-f326-4884-b1cf-06f3ad86be52", // Virtual Machine User Login
		Scope:            "/",
	})

	if err != nil {
		return nil, err
	}

	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-user-assignment", name), &authorization.RoleAssignmentArgs{
		PrincipalId:      args.LoginUserGroupId,
		PrincipalType:    pulumi.String(authorization.PrincipalTypeGroup),
		RoleDefinitionId: pulumi.String(vmUserLoginRole.Id),
		Scope:            avdVM.VirtualMachine.ID(),
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	vmAdminLoginRole, err := authorization.LookupRoleDefinition(ctx, &authorization.LookupRoleDefinitionArgs{
		RoleDefinitionId: "1c0163c0-47e6-4577-8991-ea5c82e286e4", // Virtual Machine Administrator Login
		Scope:            "/",
	})

	if err != nil {
		return nil, err
	}

	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-admin-assignment", name), &authorization.RoleAssignmentArgs{
		PrincipalId:      args.LoginAdminGroupId,
		PrincipalType:    pulumi.String(authorization.PrincipalTypeGroup),
		RoleDefinitionId: pulumi.String(vmAdminLoginRole.Id),
		Scope:            avdVM.VirtualMachine.ID(),
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	_, err = compute.NewVirtualMachineExtension(ctx, fmt.Sprintf("%s-aad", name), &compute.VirtualMachineExtensionArgs{
		ResourceGroupName:       args.ResourceGroupName,
		VmExtensionName:         pulumi.String("AADLoginForWindows"),
		VmName:                  avdVM.VirtualMachine.Name,
		AutoUpgradeMinorVersion: pulumi.Bool(true),
		Type:                    pulumi.String("AADLoginForWindows"),
		TypeHandlerVersion:      pulumi.String("2.0"),
		Publisher:               pulumi.String("Microsoft.Azure.ActiveDirectory"),
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	_, err = compute.NewVirtualMachineExtension(ctx, fmt.Sprintf("%s-dsc", name), &compute.VirtualMachineExtensionArgs{
		ResourceGroupName:       args.ResourceGroupName,
		VmExtensionName:         pulumi.String("Microsoft.PowerShell.DSC"),
		VmName:                  avdVM.VirtualMachine.Name,
		AutoUpgradeMinorVersion: pulumi.Bool(true),
		Type:                    pulumi.String("DSC"),
		TypeHandlerVersion:      pulumi.String("2.73"),
		Publisher:               pulumi.String("Microsoft.Powershell"),
		Settings: pulumi.Map{
			"modulesUrl":            pulumi.String("https://wvdportalstorageblob.blob.core.windows.net/galleryartifacts/Configuration_01-19-2023.zip"),
			"configurationFunction": pulumi.String("Configuration.ps1\\AddSessionHost"),
			"properties": pulumi.Map{
				"hostPoolName":             args.HostPoolName,
				"registrationInfoToken":    args.RegistrationToken,
				"aadJoin":                  pulumi.Bool(true),
				"useAgentDownloadEndpoint": pulumi.Bool(true),
			},
		},
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	params := compute.RunCommandInputParameterArray{}
	for _, arg := range args.Spec.InitScriptArguments {
		params = append(params, compute.RunCommandInputParameterArgs{
			Name:  pulumi.String(arg.Name),
			Value: pulumi.String(arg.Value),
		})
	}

	_, err = compute.NewVirtualMachineRunCommandByVirtualMachine(ctx, fmt.Sprintf("%s-init-cmd", name), &compute.VirtualMachineRunCommandByVirtualMachineArgs{
		ResourceGroupName: args.ResourceGroupName,
		VmName:            avdVM.VirtualMachine.Name,
		AsyncExecution:    pulumi.Bool(false),
		RunCommandName:    pulumi.String("InitVM"),
		Source: compute.VirtualMachineRunCommandScriptSourceArgs{
			Script: pulumi.String(args.Spec.InitScript),
		},
		Parameters:       params,
		TimeoutInSeconds: pulumi.Int(60),
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	fsLogixSetupScript := `
	param
	(    
		[Parameter(Mandatory = $true)]
		[String]$fileServer,
	
		[Parameter(Mandatory = $true)]
		[String]$profileShare,
	
		[Parameter(Mandatory = $true)]
		[String]$user,
	
		[Parameter(Mandatory = $true)]
		[String]$secret
	)
	
	write-host "Configuring FSLogix"

	$fileServer = ([System.Uri]$fileServer).Host
	$profileShare = "\\$($fileServer)\$($profileShare)"
	$user = "localhost\$($user)"
	
	New-Item -Path "HKLM:\SOFTWARE" -Name "FSLogix" -ErrorAction Ignore
	New-Item -Path "HKLM:\SOFTWARE\FSLogix" -Name "Profiles" -ErrorAction Ignore
	New-ItemProperty -Path "HKLM:\SOFTWARE\FSLogix\Profiles" -Name "Enabled" -Value 1 -force
	New-ItemProperty -Path "HKLM:\SOFTWARE\FSLogix\Profiles" -Name "VHDLocations" -Value $($profileShare) -force
	New-ItemProperty -Path "HKLM:\SOFTWARE\FSLogix\Profiles" -Name "AccessNetworkAsComputerObject" -Value 1 -force

	New-ItemProperty -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services" -Name "RemoteAppLogoffTimeLimit" -PropertyType 'DWord' -Value '300000' -Force # 5min
	
	# Store credentials to access the storage account
	cmdkey.exe /add:$($fileServer) /user:$($user) /pass:$($secret)
	# Disable Windows Defender Credential Guard (only needed for Windows 11 22H2)
	New-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa" -Name "LsaCfgFlags" -Value 0 -force
		
	write-host "The script has finished."
`

	_, err = compute.NewVirtualMachineRunCommandByVirtualMachine(ctx, fmt.Sprintf("%s-fslogix-setup", name), &compute.VirtualMachineRunCommandByVirtualMachineArgs{
		ResourceGroupName: args.ResourceGroupName,
		VmName:            avdVM.VirtualMachine.Name,
		AsyncExecution:    pulumi.Bool(false),
		RunCommandName:    pulumi.String("SetupFSLogix"),
		Source: compute.VirtualMachineRunCommandScriptSourceArgs{
			Script: pulumi.String(fsLogixSetupScript),
		},
		Parameters: compute.RunCommandInputParameterArray{
			compute.RunCommandInputParameterArgs{
				Name:  pulumi.String("fileServer"),
				Value: args.ProfileFileServer,
			},
			compute.RunCommandInputParameterArgs{
				Name:  pulumi.String("profileShare"),
				Value: args.ProfileShare,
			},
			compute.RunCommandInputParameterArgs{
				Name:  pulumi.String("user"),
				Value: args.ProfileUser,
			},
			compute.RunCommandInputParameterArgs{
				Name:  pulumi.String("secret"),
				Value: args.ProfileSecret,
			},
		},
		TimeoutInSeconds: pulumi.Int(60),
	}, pulumi.Parent(avdVM.VirtualMachine))

	if err != nil {
		return nil, err
	}

	return avdVM, nil
}

func azureVirtualDesktopDeployFunc[T provisioning.ProvisioningTarget](target T, resourceGroupName pulumi.StringOutput,
	avds []*provisioningv1.AzureVirtualDesktop) pulumi.RunFunc {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("AzureVirtualDesktop")
	return func(ctx *pulumi.Context) error {
		for _, avd := range avds {
			hostPoolName := avd.Spec.HostPoolName

			globalQalifier := provisioning.Match(target,
				func(tenant *platformv1.Tenant) string {
					return fmt.Sprintf("%s-%s", tenant.Spec.PlatformRef, tenant.GetName())
				},
				func(platform *platformv1.Platform) string {
					return fmt.Sprintf("%s", platform.GetName())
				},
			)

			pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources

			avdComponent := &AzureVirtualDesktop{}
			err := ctx.RegisterComponentResource("ts-azure-comp:azureVirtualDesktop:AzureVirtualDesktop", hostPoolName, avdComponent)
			if err != nil {
				return err
			}

			current, err := azuread.GetClientConfig(ctx, nil, nil)
			if err != nil {
				return err
			}
			appsUserGroup, err := azuread.NewGroup(ctx, fmt.Sprintf("%s-apps", hostPoolName), &azuread.GroupArgs{
				DisplayName: pulumi.String(fmt.Sprintf("%s-%s-apps", globalQalifier, hostPoolName)),
				Owners: pulumi.StringArray{
					pulumi.String(current.ObjectId),
				},
				SecurityEnabled: pulumi.Bool(true),
			}, pulumi.Parent(avdComponent), pulumi.RetainOnDelete(pulumiRetainOnDelete))
			if err != nil {
				return err
			}

			for _, appUser := range avd.Spec.Users.ApplicationUsers {
				user, err := azuread.LookupUser(ctx, &azuread.LookupUserArgs{
					UserPrincipalName: pulumi.StringRef(appUser),
				}, nil)
				if err != nil {
					return err
				}

				_, err = azuread.NewGroupMember(ctx, fmt.Sprintf("%s-app-user-%s", appUser, avd.Spec.HostPoolName), &azuread.GroupMemberArgs{
					GroupObjectId:  appsUserGroup.ID(),
					MemberObjectId: pulumi.String(user.Id),
				}, pulumi.Parent(appsUserGroup))
				if err != nil {
					return err
				}
			}

			adminUserGroup, err := azuread.NewGroup(ctx, fmt.Sprintf("%s-admin", hostPoolName), &azuread.GroupArgs{
				DisplayName: pulumi.String(fmt.Sprintf("%s-%s-admin", globalQalifier, hostPoolName)),
				Owners: pulumi.StringArray{
					pulumi.String(current.ObjectId),
				},
				SecurityEnabled: pulumi.Bool(true),
			}, pulumi.Parent(avdComponent), pulumi.RetainOnDelete(pulumiRetainOnDelete))
			if err != nil {
				return err
			}

			for _, admin := range avd.Spec.Users.Admins {
				user, err := azuread.LookupUser(ctx, &azuread.LookupUserArgs{
					UserPrincipalName: pulumi.StringRef(admin),
				}, nil)
				if err != nil {
					return err
				}

				_, err = azuread.NewGroupMember(ctx, fmt.Sprintf("%s-admin-%s", admin, avd.Spec.HostPoolName), &azuread.GroupMemberArgs{
					GroupObjectId:  adminUserGroup.ID(),
					MemberObjectId: pulumi.String(user.Id),
				}, pulumi.Parent(adminUserGroup))
				if err != nil {
					return err
				}
			}

			avdComponent.HostPool, err = desktopvirtualization.NewHostPool(ctx, hostPoolName, &desktopvirtualization.HostPoolArgs{
				//HostPoolName:                  pulumi.String(hostPoolName),
				HostPoolType:                  pulumi.String(desktopvirtualization.HostPoolTypePooled),
				LoadBalancerType:              pulumi.String(desktopvirtualization.LoadBalancerTypeBreadthFirst),
				CustomRdpProperty:             pulumi.String("targetisaadjoined:i:1;drivestoredirect:s:*;audiomode:i:0;videoplaybackmode:i:1;redirectclipboard:i:1;redirectprinters:i:1;devicestoredirect:s:*;redirectcomports:i:1;redirectsmartcards:i:1;usbdevicestoredirect:s:*;enablecredsspsupport:i:1;redirectwebauthn:i:1;use multimon:i:1"),
				PreferredAppGroupType:         pulumi.String(desktopvirtualization.PreferredAppGroupTypeDesktop),
				PersonalDesktopAssignmentType: pulumi.String(desktopvirtualization.PersonalDesktopAssignmentTypeAutomatic),
				//MaxSessionLimit:               pulumi.Int(999999),
				ValidationEnvironment: pulumi.Bool(false),

				ResourceGroupName: resourceGroupName,
				StartVMOnConnect:  pulumi.Bool(false),

				RegistrationInfo: &desktopvirtualization.RegistrationInfoArgs{
					ExpirationTime:             pulumi.String(time.Now().AddDate(0, 0, 14).Format(time.RFC3339)),
					RegistrationTokenOperation: pulumi.String(desktopvirtualization.RegistrationTokenOperationUpdate),
				},

				VmTemplate: pulumi.String(fmt.Sprintf(`
				{
					"domain":"",
					"galleryImageOffer":null,
					"galleryImagePublisher":null,
					"galleryImageSKU":null,
					"imageType":"CustomImage",
					"customImageId":"%s",
					"namePrefix":"%s",
					"osDiskType":"StandardSSD_LRS",
					"vmSize":{"id":"%s","cores":1,"ram":1,"rdmaEnabled":false,"supportsMemoryPreservingMaintenance":true},
					"galleryItemId":null,
					"hibernate":false,
					"diskSizeGB":0,
					"securityType":"Standard",
					"secureBoot":false,
					"vTPM":false
				}`, avd.Spec.SourceImageId, avd.Spec.VmNamePrefix, avd.Spec.VmSize)),
			}, pulumi.Parent(avdComponent))
			if err != nil {
				return err
			}

			avdComponent.DesktopAppGroup, err = desktopvirtualization.NewApplicationGroup(ctx, fmt.Sprintf("%s-desktop", hostPoolName), &desktopvirtualization.ApplicationGroupArgs{
				//ApplicationGroupName: pulumi.String(fmt.Sprintf("%s-desktop", hostPoolName)),
				ApplicationGroupType: pulumi.String(desktopvirtualization.ApplicationGroupTypeDesktop),
				FriendlyName:         pulumi.String("Desktop"),
				HostPoolArmPath:      avdComponent.HostPool.ID(),
				ResourceGroupName:    resourceGroupName,
			}, pulumi.Parent(avdComponent))

			if err != nil {
				return err
			}

			avdComponent.RemoteAppGroup, err = desktopvirtualization.NewApplicationGroup(ctx, fmt.Sprintf("%s-apps", hostPoolName), &desktopvirtualization.ApplicationGroupArgs{
				//ApplicationGroupName: pulumi.String(fmt.Sprintf("%s-apps", hostPoolName)),
				ApplicationGroupType: pulumi.String(desktopvirtualization.ApplicationGroupTypeRemoteApp),
				FriendlyName:         pulumi.String("Applications"),
				HostPoolArmPath:      avdComponent.HostPool.ID(),
				ResourceGroupName:    resourceGroupName,
			}, pulumi.Parent(avdComponent))

			if err != nil {
				return err
			}

			avdUserRole, err := authorization.LookupRoleDefinition(ctx, &authorization.LookupRoleDefinitionArgs{
				RoleDefinitionId: "1d18fff3-a72a-46b5-b4a9-0b38a3cd7e63", //Desktop Virtualization User
				Scope:            "/",
			})

			if err != nil {
				return err
			}

			_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-apps-user-assignment", hostPoolName), &authorization.RoleAssignmentArgs{
				PrincipalId:      appsUserGroup.ObjectId,
				PrincipalType:    pulumi.String(authorization.PrincipalTypeGroup),
				RoleDefinitionId: pulumi.String(avdUserRole.Id),
				Scope:            avdComponent.RemoteAppGroup.ID(),
			}, pulumi.Parent(avdComponent.RemoteAppGroup))

			if err != nil {
				return err
			}

			_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-apps-admin-assignment", hostPoolName), &authorization.RoleAssignmentArgs{
				PrincipalId:      adminUserGroup.ObjectId,
				PrincipalType:    pulumi.String(authorization.PrincipalTypeGroup),
				RoleDefinitionId: pulumi.String(avdUserRole.Id),
				Scope:            avdComponent.RemoteAppGroup.ID(),
			}, pulumi.Parent(avdComponent.RemoteAppGroup))

			if err != nil {
				return err
			}

			_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("%s-desktop-admin-assignment", hostPoolName), &authorization.RoleAssignmentArgs{
				PrincipalId:      adminUserGroup.ObjectId,
				PrincipalType:    pulumi.String(authorization.PrincipalTypeGroup),
				RoleDefinitionId: pulumi.String(avdUserRole.Id),
				Scope:            avdComponent.DesktopAppGroup.ID(),
			}, pulumi.Parent(avdComponent.DesktopAppGroup))

			if err != nil {
				return err
			}

			for _, app := range avd.Spec.Applications {
				_, err = desktopvirtualization.NewApplication(ctx, fmt.Sprintf("%s-apps-%s", hostPoolName, app.Name), &desktopvirtualization.ApplicationArgs{
					ApplicationGroupName: avdComponent.RemoteAppGroup.Name,
					ApplicationName:      pulumi.String(app.Name),
					FriendlyName:         pulumi.String(app.FriendlyName),
					FilePath:             pulumi.String(app.Path),
					IconPath:             pulumi.String(app.Path),
					IconIndex:            pulumi.Int(0),
					CommandLineSetting:   pulumi.String(desktopvirtualization.CommandLineSettingDoNotAllow),
					ShowInPortal:         pulumi.Bool(true),
					ResourceGroupName:    resourceGroupName,
				}, pulumi.Parent(avdComponent.RemoteAppGroup))

				if err != nil {
					return err
				}
			}

			_, err = desktopvirtualization.NewWorkspace(ctx, fmt.Sprintf("%s-ws", hostPoolName), &desktopvirtualization.WorkspaceArgs{
				//WorkspaceName:     pulumi.String(fmt.Sprintf("%s-ws", hostPoolName)),
				FriendlyName:      pulumi.String(fmt.Sprintf("%s - %s", avd.Spec.WorkspaceFriendlyName, target.GetDescription())),
				ResourceGroupName: resourceGroupName,
				ApplicationGroupReferences: pulumi.StringArray{
					avdComponent.DesktopAppGroup.ID(),
					avdComponent.RemoteAppGroup.ID(),
				},
			}, pulumi.Parent(avdComponent))

			if err != nil {
				return err
			}

			storageAccountName := strings.ToLower(hostPoolName)
			storageAccountName = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(storageAccountName, "")
			storageAccountName = storageAccountName[:int32(math.Min(float64(len(storageAccountName)), 16))] // extra 8 chars for UID, total max 24 chars

			storageAccount, err := storage.NewStorageAccount(ctx, storageAccountName, &storage.StorageAccountArgs{
				Kind:              pulumi.String(storage.KindStorage),
				ResourceGroupName: resourceGroupName,
				Sku: &storage.SkuArgs{
					Name: pulumi.String(storage.SkuName_Standard_LRS),
				},
			}, pulumi.Parent(avdComponent))
			if err != nil {
				return err
			}

			profileShare, err := storage.NewFileShare(ctx, fmt.Sprintf("%s-profile-share", hostPoolName), &storage.FileShareArgs{
				AccountName:       storageAccount.Name,
				ResourceGroupName: resourceGroupName,
				ShareName:         pulumi.String("profiles"),
				ShareQuota:        pulumi.Int(100),
			}, pulumi.Parent(storageAccount), pulumi.RetainOnDelete(pulumiRetainOnDelete))
			if err != nil {
				return err
			}

			storageAccountKeys := storage.ListStorageAccountKeysOutput(ctx, storage.ListStorageAccountKeysOutputArgs{
				AccountName:       storageAccount.Name,
				ResourceGroupName: resourceGroupName,
			})

			if err != nil {
				return err
			}

			var diskDeleteOptions compute.DeleteOptions
			if pulumiRetainOnDelete {
				diskDeleteOptions = compute.DeleteOptionsDetach
			} else {
				diskDeleteOptions = compute.DeleteOptionsDelete
			}

			vms := make([]*AzureVirtualDesktopVM, avd.Spec.VmNumberOfInstances)

			for i := 0; i < avd.Spec.VmNumberOfInstances; i++ {
				vmName := fmt.Sprintf("%s-%d", avd.Spec.HostPoolName, i)
				avdVM, err := NewAzureVirtualDesktopVM(ctx, vmName, &AzureVirtualDesktopVMArgs{
					ResourceGroupName: resourceGroupName,
					TenantName:        pulumi.String(target.GetName()),
					HostPoolName:      avdComponent.HostPool.Name,
					RegistrationToken: avdComponent.HostPool.RegistrationInfo.Token().Elem(),
					VMSize:            pulumi.String(avd.Spec.VmSize),
					SubnetID:          pulumi.String(avd.Spec.SubnetId),
					LoginUserGroupId:  appsUserGroup.ID(),
					LoginAdminGroupId: adminUserGroup.ID(),
					DiskDeleteOptions: diskDeleteOptions,

					ProfileFileServer: storageAccount.PrimaryEndpoints.File(),
					ProfileShare:      profileShare.Name,
					ProfileUser:       storageAccount.Name,
					ProfileSecret:     storageAccountKeys.Keys().Index(pulumi.Int(0)).Value(),

					Spec: avd.Spec,
				}, pulumi.Parent(avdComponent))

				if err != nil {
					return err
				}

				vms[i] = avdVM
			}

			for _, exp := range avd.Spec.Exports {
				err = valueExporter(newExportContext(ctx, exp.Domain, avd.Name, avd.ObjectMeta, gvk),
					map[string]exportTemplateWithValue{
						"hostPoolName": {exp.HostPoolName, avdComponent.HostPool.Name},
						"computerName": {exp.ComputerName, joinProp(vms, func(vm *AzureVirtualDesktopVM) pulumi.StringOutput { return vm.ComputerName })},
						"adminUserName": {exp.AdminUserName, joinProp(vms, func(vm *AzureVirtualDesktopVM) pulumi.StringOutput {
							return vm.VirtualMachine.OsProfile.AdminUsername().Elem()
						})},
						"adminPassword": {exp.AdminPassword, joinProp(vms, func(vm *AzureVirtualDesktopVM) pulumi.StringOutput { return vm.AdminPassword.Result })},
					}, pulumi.Parent(avdComponent))
				if err != nil {
					return err
				}
			}

			ctx.Export(fmt.Sprintf("azureVirtualDesktop:%s", avd.Spec.HostPoolName), avdComponent.HostPool.Name)
		}

		return nil
	}
}

func joinProp(vms []*AzureVirtualDesktopVM, selector func(vm *AzureVirtualDesktopVM) pulumi.StringOutput) pulumi.StringOutput {
	selectedProps := make([]interface{}, len(vms))
	for i := range vms {
		selectedProps[i] = selector(vms[i])
	}

	return pulumi.All(selectedProps...).ApplyT(func(args []interface{}) string {
		stringArgs := make([]string, len(args))
		for i := range vms {
			stringArgs[i] = args[i].(string)
		}

		return strings.Join(stringArgs, " ; ")
	}).(pulumi.StringOutput)
}
