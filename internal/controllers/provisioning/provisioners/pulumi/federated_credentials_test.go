package pulumi

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

// testOidcIssuer is a real-shaped AKS issuer URL. Its trailing slash is significant: Entra matches
// it against the token's iss claim exactly, so it must reach the credential verbatim.
const testOidcIssuer = "https://westeurope.oic.prod-aks.azure.com/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111/"

const federatedCredentialType = "azure-native:managedidentity:FederatedIdentityCredential"

// dependsOnResource reports whether the resource registered under name declared a dependency on the
// resource registered under dependencyName (mock URNs end with "::<resource name>").
func (m *resourceCaptureMocks) dependsOnResource(name, dependencyName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	args, ok := m.byName[name]
	if !ok || args.RegisterRPC == nil {
		return false
	}
	for _, urn := range args.RegisterRPC.GetDependencies() {
		if strings.HasSuffix(urn, "::"+dependencyName) {
			return true
		}
	}
	return false
}

func deployTestManagedIdentity(t *testing.T, capture *resourceCaptureMocks, identitySpec *provisioningv1.ManagedIdentitySpec,
	ownerNamespace string, retainOnDelete bool) error {
	return pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := newTestProvider(t, ctx, "test-provider-fic")
		_, _, err := deployManagedIdentity(ctx, provider, "my-db-app", pulumi.String("1").ToStringOutput(),
			identitySpec, "my-db_qa_mf", ownerNamespace, []pulumi.Resource{}, retainOnDelete, nil)
		return err
	}, pulumi.WithMocks("project", "stack", capture))
}

func TestDeployFederatedCredentials(t *testing.T) {
	t.Run("creates one credential per service account trusting the cluster issuer", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name:              "app",
			ResourceGroupName: "my-rg",
			Location:          "westeurope",
			ServiceAccounts: []provisioningv1.FederatedServiceAccount{
				{Name: "charisma-hcm"},
				{Name: "worker", Namespace: "other-ns"},
			},
		}, "qa-hcm", false)
		assert.NoError(t, err)

		assert.Len(t, capture.byType[federatedCredentialType], 2)

		first := capture.byName["my-db-app-identity-fic-qa-hcm_charisma-hcm"].Inputs
		assert.Equal(t, testOidcIssuer, first["issuer"].StringValue(), "the issuer must be passed through verbatim, trailing slash included")
		assert.Equal(t, "system:serviceaccount:qa-hcm:charisma-hcm", first["subject"].StringValue(),
			"a service account without namespace must default to the owning resource's namespace")
		assert.Equal(t, "qa-hcm_charisma-hcm", first["federatedIdentityCredentialResourceName"].StringValue())
		assert.Equal(t, "app_my-db_qa_mf", first["resourceName"].StringValue(), "the credential must be created on the tenant-scoped identity")
		assert.Equal(t, "my-rg", first["resourceGroupName"].StringValue())
		audiences := first["audiences"].ArrayValue()
		assert.Len(t, audiences, 1)
		assert.Equal(t, "api://AzureADTokenExchange", audiences[0].StringValue())

		second := capture.byName["my-db-app-identity-fic-other-ns_worker"].Inputs
		assert.Equal(t, "system:serviceaccount:other-ns:worker", second["subject"].StringValue())
		assert.Equal(t, "other-ns_worker", second["federatedIdentityCredentialResourceName"].StringValue())
	})

	t.Run("credentials are created one after another, starting after the identity", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name: "app", ResourceGroupName: "my-rg", Location: "westeurope",
			ServiceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}, {Name: "sa2"}, {Name: "sa3"}},
		}, "ns", false)
		assert.NoError(t, err)

		assert.True(t, capture.dependsOnResource("my-db-app-identity-fic-ns_sa1", "my-db-app-identity"))
		assert.True(t, capture.dependsOnResource("my-db-app-identity-fic-ns_sa2", "my-db-app-identity-fic-ns_sa1"),
			"Azure rejects concurrent credential writes on the same identity")
		assert.True(t, capture.dependsOnResource("my-db-app-identity-fic-ns_sa3", "my-db-app-identity-fic-ns_sa2"))
	})

	t.Run("credentials are never retained on delete, unlike the identity", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name: "app", ResourceGroupName: "my-rg", Location: "westeurope",
			ServiceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}},
		}, "ns", true)
		assert.NoError(t, err)

		assert.True(t, capture.retainOnDelete("my-db-app-identity"))
		assert.False(t, capture.retainOnDelete("my-db-app-identity-fic-ns_sa1"),
			"a retained identity must not stay usable from the cluster once its owner is gone")
	})

	t.Run("dots in a service account name become dashes in the credential name", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name: "app", ResourceGroupName: "my-rg", Location: "westeurope",
			ServiceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "my.app"}},
		}, "ns", false)
		assert.NoError(t, err)

		credential := capture.byName["my-db-app-identity-fic-ns_my-app"].Inputs
		assert.Equal(t, "ns_my-app", credential["federatedIdentityCredentialResourceName"].StringValue())
		assert.Equal(t, "system:serviceaccount:ns:my.app", credential["subject"].StringValue(), "the subject keeps the real service account name")
	})

	t.Run("no service accounts — no credential, issuer not required", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, "")
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name: "app", ResourceGroupName: "my-rg", Location: "westeurope",
		}, "ns", false)
		assert.NoError(t, err)
		assert.Empty(t, capture.byType[federatedCredentialType])
	})

	t.Run("service accounts without the issuer fail", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, "")
		capture := newResourceCaptureMocks()
		err := deployTestManagedIdentity(t, capture, &provisioningv1.ManagedIdentitySpec{
			Name: "app", ResourceGroupName: "my-rg", Location: "westeurope",
			ServiceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}},
		}, "ns", false)
		assert.ErrorContains(t, err, "spec.managedIdentities[app].serviceAccounts requires AZURE_AKS_OIDC_ISSUER_URL to be set")
		assert.Empty(t, capture.byType[federatedCredentialType])
	})
}

func TestValidateServiceAccounts(t *testing.T) {
	identity := func(serviceAccounts ...provisioningv1.FederatedServiceAccount) []provisioningv1.ManagedIdentitySpec {
		return []provisioningv1.ManagedIdentitySpec{{Name: "app", ServiceAccounts: serviceAccounts}}
	}

	t.Run("identities without service accounts don't need the issuer", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, "")
		assert.NoError(t, validateServiceAccounts(identity(), ""))
		assert.NoError(t, validateServiceAccounts(nil, ""))
	})

	t.Run("valid service accounts", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		assert.NoError(t, validateServiceAccounts(identity(
			provisioningv1.FederatedServiceAccount{Name: "sa1"},
			provisioningv1.FederatedServiceAccount{Name: "sa1", Namespace: "other"},
		), "qa-hcm"))
	})

	t.Run("service accounts whose namespace and name only differ in where they are split are distinct", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		assert.NoError(t, validateServiceAccounts(identity(
			provisioningv1.FederatedServiceAccount{Name: "hcm-api", Namespace: "qa"},
			provisioningv1.FederatedServiceAccount{Name: "api", Namespace: "qa-hcm"},
		), ""))
	})

	t.Run("20 service accounts are allowed", func(t *testing.T) {
		t.Setenv(EnvAzureAksOidcIssuerUrl, testOidcIssuer)
		assert.NoError(t, validateServiceAccounts(identity(manyServiceAccounts(20)...), "ns"))
	})

	tests := []struct {
		name            string
		issuer          string
		serviceAccounts []provisioningv1.FederatedServiceAccount
		ownerNamespace  string
		wantErr         string
	}{
		{
			name:            "service accounts without the issuer",
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}},
			ownerNamespace:  "ns",
			wantErr:         "spec.managedIdentities[app].serviceAccounts requires AZURE_AKS_OIDC_ISSUER_URL to be set",
		},
		{
			name:            "issuer that is not an https URL",
			issuer:          "westeurope.oic.prod-aks.azure.com/tenant/cluster/",
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}},
			ownerNamespace:  "ns",
			wantErr:         `AZURE_AKS_OIDC_ISSUER_URL must be an https URL (the AKS cluster's oidcIssuerProfile.issuerUrl), got "westeurope.oic.prod-aks.azure.com/tenant/cluster/"`,
		},
		{
			name:            "missing name",
			issuer:          testOidcIssuer,
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Namespace: "ns"}},
			ownerNamespace:  "ns",
			wantErr:         "spec.managedIdentities[app].serviceAccounts[].name is required",
		},
		{
			name:            "no namespace and no owner namespace",
			issuer:          testOidcIssuer,
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}},
			wantErr:         "spec.managedIdentities[app].serviceAccounts[sa1].namespace is required",
		},
		{
			name:            "duplicate service account",
			issuer:          testOidcIssuer,
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "sa1"}, {Name: "sa1", Namespace: "ns"}},
			ownerNamespace:  "ns",
			wantErr:         `spec.managedIdentities[app].serviceAccounts[ns/sa1] is duplicated (federated credential name "ns_sa1")`,
		},
		{
			name:            "different service accounts with the same credential name",
			issuer:          testOidcIssuer,
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: "my.app"}, {Name: "my-app"}},
			ownerNamespace:  "ns",
			wantErr:         `spec.managedIdentities[app].serviceAccounts[ns/my-app] is duplicated (federated credential name "ns_my-app")`,
		},
		{
			name:            "credential name too long",
			issuer:          testOidcIssuer,
			serviceAccounts: []provisioningv1.FederatedServiceAccount{{Name: strings.Repeat("a", 120)}},
			ownerNamespace:  "ns",
			wantErr:         "must be 3-120 letters, digits",
		},
		{
			name:            "more than 20 service accounts",
			issuer:          testOidcIssuer,
			serviceAccounts: manyServiceAccounts(21),
			ownerNamespace:  "ns",
			wantErr:         "spec.managedIdentities[app].serviceAccounts has 21 entries, Azure allows at most 20 federated credentials per identity",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvAzureAksOidcIssuerUrl, tt.issuer)
			assert.ErrorContains(t, validateServiceAccounts(identity(tt.serviceAccounts...), tt.ownerNamespace), tt.wantErr)
		})
	}
}

func manyServiceAccounts(n int) []provisioningv1.FederatedServiceAccount {
	serviceAccounts := make([]provisioningv1.FederatedServiceAccount, n)
	for i := range serviceAccounts {
		serviceAccounts[i] = provisioningv1.FederatedServiceAccount{Name: "sa" + strings.Repeat("x", i+1)}
	}
	return serviceAccounts
}
