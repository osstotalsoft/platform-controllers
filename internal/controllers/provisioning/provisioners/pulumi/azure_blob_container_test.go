package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

const (
	testStorageAccountId = "/subscriptions/sub-1/resourceGroups/storage-rg/providers/Microsoft.Storage/storageAccounts/mystorage"
	blobContainerType    = "azure-native:storage:BlobContainer"
	roleAssignmentType   = "azure-native:authorization:RoleAssignment"
)

func newBlobContainer(name string) *provisioningv1.AzureBlobContainer {
	return &provisioningv1.AzureBlobContainer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "qa"},
		Spec: provisioningv1.AzureBlobContainerSpec{
			ContainerName: "files",
			StorageAccount: provisioningv1.StorageAccountSpec{
				ResourceGroupName: "storage-rg",
				AccountName:       "mystorage",
			},
			ProvisioningMeta: provisioningv1.ProvisioningMeta{DomainRef: "example-domain"},
		},
	}
}

func newBlobContainerMocks() *resourceCaptureMocks {
	capture := newResourceCaptureMocks()
	capture.stubCall("azure-native:storage:getStorageAccount", resource.PropertyMap{
		"id":   resource.NewStringProperty(testStorageAccountId),
		"name": resource.NewStringProperty("mystorage"),
		"primaryEndpoints": resource.NewObjectProperty(resource.PropertyMap{
			"blob": resource.NewStringProperty("https://mystorage.blob.core.windows.net/"),
		}),
	})
	capture.stubCall("azure-native:storage:listStorageAccountServiceSAS", resource.PropertyMap{
		"serviceSasToken": resource.NewStringProperty("sv=2022-11-02&sr=c&sp=racwdl&sig=abc"),
	})
	return capture
}

func deployTestBlobContainer(t *testing.T, capture *resourceCaptureMocks, container *provisioningv1.AzureBlobContainer, imports *importOptions) error {
	t.Helper()
	tenant := newTenant("tenant1", "dev")
	return pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := deployAzureBlobContainer(tenant, container, []pulumi.Resource{}, imports, ctx)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
}

// exportedConfigMapData returns the data of the only exported ConfigMap, secrets unwrapped.
func exportedConfigMapData(t *testing.T, capture *resourceCaptureMocks) map[string]string {
	t.Helper()
	configMaps := capture.byType["kubernetes:core/v1:ConfigMap"]
	if !assert.Len(t, configMaps, 1) {
		return nil
	}
	data := map[string]string{}
	inputs := configMaps[0].Inputs["data"]
	if inputs.IsSecret() {
		inputs = inputs.SecretValue().Element
	}
	for k, v := range inputs.ObjectValue() {
		if v.IsSecret() {
			v = v.SecretValue().Element
		}
		data[string(k)] = v.StringValue()
	}
	return data
}

func toConfigMap(key string) provisioningv1.ValueExport {
	return provisioningv1.ValueExport{ToConfigMap: provisioningv1.ConfigMapTemplate{KeyTemplate: key}}
}

func TestDeployAzureBlobContainer(t *testing.T) {
	t.Run("creates a tenant scoped container in the existing storage account", func(t *testing.T) {
		capture := newBlobContainerMocks()
		err := deployTestBlobContainer(t, capture, newBlobContainer("my-container"), nil)
		assert.NoError(t, err)

		container := capture.byName["my-container"]
		assert.Equal(t, blobContainerType, container.TypeToken)
		assert.Equal(t, "files-dev-tenant1", container.Inputs["containerName"].StringValue())
		assert.Equal(t, "mystorage", container.Inputs["accountName"].StringValue())
		assert.Equal(t, "storage-rg", container.Inputs["resourceGroupName"].StringValue())
		assert.Empty(t, capture.importId(t, "my-container"))
		assert.Empty(t, capture.byType["azure-native:managedidentity:UserAssignedIdentity"])
		assert.Empty(t, capture.byType[roleAssignmentType])
	})

	t.Run("exports the container's coordinates and its SAS URL", func(t *testing.T) {
		capture := newBlobContainerMocks()
		container := newBlobContainer("my-container")
		container.Spec.Exports = []provisioningv1.AzureBlobContainerExportsSpec{{
			ContainerName:   toConfigMap("Storage__ContainerName"),
			AccountName:     toConfigMap("Storage__AccountName"),
			BlobEndpoint:    toConfigMap("Storage__BlobEndpoint"),
			ContainerUrl:    toConfigMap("Storage__ContainerUrl"),
			ContainerSasUrl: toConfigMap("ObjectStorage__BlobContainerSasUrl"),
		}}

		err := deployTestBlobContainer(t, capture, container, nil)
		assert.NoError(t, err)

		data := exportedConfigMapData(t, capture)
		assert.Equal(t, map[string]string{
			"Storage__ContainerName":             "files-dev-tenant1",
			"Storage__AccountName":               "mystorage",
			"Storage__BlobEndpoint":              "https://mystorage.blob.core.windows.net/",
			"Storage__ContainerUrl":              "https://mystorage.blob.core.windows.net/files-dev-tenant1",
			"ObjectStorage__BlobContainerSasUrl": "https://mystorage.blob.core.windows.net/files-dev-tenant1?sv=2022-11-02&sr=c&sp=racwdl&sig=abc",
		}, data)
		assert.Equal(t, "example-domain-tenant1-my-container",
			capture.byType["kubernetes:core/v1:ConfigMap"][0].Inputs["metadata"].ObjectValue()["name"].StringValue(),
			"an export without a domain goes to domainRef")
	})

	t.Run("grants each managed identity its role on the container", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		capture := newBlobContainerMocks()
		capture.stubIdentityIds("my-container-reader-blob-identity", "reader-client", "reader-principal")
		container := newBlobContainer("my-container")
		container.Spec.ManagedIdentities = []provisioningv1.BlobContainerIdentitySpec{
			{
				Name:              "app",
				ResourceGroupName: "identity-rg",
				Location:          "westeurope",
				ServiceAccounts:   []provisioningv1.FederatedServiceAccount{{Name: "app-{{ .Tenant.Code }}"}},
			},
			{
				Name:              "reader",
				ResourceGroupName: "identity-rg",
				Location:          "westeurope",
				Role:              provisioningv1.BlobContainerRoleReader,
			},
		}
		container.Spec.Exports = []provisioningv1.AzureBlobContainerExportsSpec{{
			Identities: []provisioningv1.ExportedIdentitySpec{
				{IdentityRef: "app", IdentityClientId: toConfigMap("App__ClientId")},
				{IdentityRef: "reader", IdentityClientId: toConfigMap("Reader__ClientId")},
			},
		}}

		err := deployTestBlobContainer(t, capture, container, nil)
		assert.NoError(t, err)

		identity := capture.byName["my-container-app-blob-identity"]
		assert.Equal(t, "app_files-dev-tenant1", identity.Inputs["resourceName"].StringValue())
		assert.Equal(t, "identity-rg", identity.Inputs["resourceGroupName"].StringValue())

		appAssignment := capture.byName["my-container-app-blob-role-assignment"]
		assert.Equal(t, "my-container_id", appAssignment.Inputs["scope"].StringValue())
		assert.Equal(t, "22222222-2222-2222-2222-222222222222", appAssignment.Inputs["principalId"].StringValue())
		assert.Equal(t, "ServicePrincipal", appAssignment.Inputs["principalType"].StringValue())
		assert.Equal(t, "/subscriptions/sub-1/providers/Microsoft.Authorization/roleDefinitions/ba92f5b4-2d11-453d-a403-e96b0029c9fe",
			appAssignment.Inputs["roleDefinitionId"].StringValue(), "the role defaults to Storage Blob Data Contributor")
		assert.False(t, capture.retainOnDelete("my-container-app-blob-role-assignment"))

		readerAssignment := capture.byName["my-container-reader-blob-role-assignment"]
		assert.Equal(t, "reader-principal", readerAssignment.Inputs["principalId"].StringValue())
		assert.Equal(t, "/subscriptions/sub-1/providers/Microsoft.Authorization/roleDefinitions/2a2b9908-6ea1-4ae2-8e65-a410df84e7d1",
			readerAssignment.Inputs["roleDefinitionId"].StringValue())

		credentials := capture.byType[federatedCredentialType]
		if assert.Len(t, credentials, 1, "only the identity with serviceAccounts gets a federated credential") {
			assert.Equal(t, "system:serviceaccount:qa:app-tenant1", credentials[0].Inputs["subject"].StringValue())
		}

		assert.Equal(t, map[string]string{
			"App__ClientId":    "11111111-1111-1111-1111-111111111111",
			"Reader__ClientId": "reader-client",
		}, exportedConfigMapData(t, capture))
	})

	t.Run("rejects invalid specs before deploying anything", func(t *testing.T) {
		tests := map[string]func(*provisioningv1.AzureBlobContainer){
			"container name too long": func(c *provisioningv1.AzureBlobContainer) {
				c.Spec.ContainerName = "a-very-long-container-name-prefix-that-azure-will-not-accept"
			},
			"container name with consecutive hyphens": func(c *provisioningv1.AzureBlobContainer) {
				c.Spec.ContainerName = "files-"
			},
			"sas validity within the rotation period": func(c *provisioningv1.AzureBlobContainer) {
				c.Spec.SasValidityDays = 30
			},
			"unknown role": func(c *provisioningv1.AzureBlobContainer) {
				c.Spec.ManagedIdentities = []provisioningv1.BlobContainerIdentitySpec{{Name: "app", Role: "Owner"}}
			},
			"duplicate export domain": func(c *provisioningv1.AzureBlobContainer) {
				c.Spec.Exports = []provisioningv1.AzureBlobContainerExportsSpec{{}, {Domain: "example-domain"}}
			},
		}
		for name, mutate := range tests {
			t.Run(name, func(t *testing.T) {
				capture := newBlobContainerMocks()
				container := newBlobContainer("my-container")
				mutate(container)
				err := deployTestBlobContainer(t, capture, container, nil)
				assert.Error(t, err)
				assert.Empty(t, capture.byType[blobContainerType])
			})
		}
	})
}

func TestImportAzureBlobContainer(t *testing.T) {
	t.Run("an explicit importContainerName is adopted as is", func(t *testing.T) {
		capture := newBlobContainerMocks()
		container := newBlobContainer("my-container")
		container.Spec.ImportContainerName = "Legacy_container"
		assert.NoError(t, deployTestBlobContainer(t, capture, container, nil))

		assert.Equal(t, testStorageAccountId+"/blobServices/default/containers/Legacy_container", capture.importId(t, "my-container"))
		assert.Equal(t, "Legacy_container", capture.byName["my-container"].Inputs["containerName"].StringValue())
	})

	t.Run("import all imports the container by its generated name", func(t *testing.T) {
		capture := newBlobContainerMocks()
		assert.NoError(t, deployTestBlobContainer(t, capture, newBlobContainer("my-container"), importAll()))

		assert.Equal(t, testStorageAccountId+"/blobServices/default/containers/files-dev-tenant1", capture.importId(t, "my-container"))
	})

	t.Run("a container the stack already manages is not imported again", func(t *testing.T) {
		capture := newBlobContainerMocks()
		imports := importAll(managedResourceKey(azureBlobContainerType, "my-container"))
		assert.NoError(t, deployTestBlobContainer(t, capture, newBlobContainer("my-container"), imports))

		assert.Empty(t, capture.importId(t, "my-container"))
	})
}

func TestBlobContainerName(t *testing.T) {
	assert.Equal(t, "files-dev-tenant-1", blobContainerName(newTenant("Tenant.1", "dev"), "Files"))
	assert.Equal(t, "my-files-dev-tenant1", blobContainerName(newTenant("tenant1", "dev"), "my_files"))
}

func TestBlobContainerSasRotation(t *testing.T) {
	deploy := func(t *testing.T, sasValidityDays int) *resourceCaptureMocks {
		capture := newBlobContainerMocks()
		container := newBlobContainer("my-container")
		container.Spec.SasValidityDays = sasValidityDays
		container.Spec.Exports = []provisioningv1.AzureBlobContainerExportsSpec{{
			ContainerSasUrl: toConfigMap("ObjectStorage__BlobContainerSasUrl"),
		}}
		assert.NoError(t, deployTestBlobContainer(t, capture, container, nil))
		return capture
	}

	t.Run("signs the SAS from the last rotation, which is renewed every 30 days", func(t *testing.T) {
		capture := deploy(t, 0)

		rotation := capture.byName["my-container-sas-rotation"]
		assert.Equal(t, "time:index/rotating:Rotating", rotation.TypeToken)
		assert.Equal(t, float64(30), rotation.Inputs["rotationDays"].NumberValue())

		args := capture.callArgs("azure-native:storage:listStorageAccountServiceSAS")
		if assert.NotNil(t, args) {
			// Without a start time Azure would sign from the current time, giving a new SAS on every update.
			assert.Equal(t, testRotationTime, args["sharedAccessStartTime"].StringValue())
			assert.Equal(t, "2027-11-10T13:18:52Z", args["sharedAccessExpiryTime"].StringValue(), "valid 365 days by default")
			assert.Equal(t, "/blob/mystorage/files-dev-tenant1", args["canonicalizedResource"].StringValue())
			assert.Equal(t, "c", args["resource"].StringValue())
			assert.Equal(t, "racwdl", args["permissions"].StringValue())
		}
	})

	t.Run("sasValidityDays sets the expiry", func(t *testing.T) {
		capture := deploy(t, 90)
		assert.Equal(t, "2027-02-08T13:18:52Z", capture.callArgs("azure-native:storage:listStorageAccountServiceSAS")["sharedAccessExpiryTime"].StringValue())
	})

	t.Run("no rotation without a containerSasUrl export", func(t *testing.T) {
		capture := newBlobContainerMocks()
		assert.NoError(t, deployTestBlobContainer(t, capture, newBlobContainer("my-container"), nil))
		assert.Empty(t, capture.byType["time:index/rotating:Rotating"])
		assert.Nil(t, capture.callArgs("azure-native:storage:listStorageAccountServiceSAS"))
	})
}
