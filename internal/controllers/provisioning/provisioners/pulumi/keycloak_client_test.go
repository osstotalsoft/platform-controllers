package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type keycloakMocks struct {
	resourceInputs map[string]resource.PropertyMap
}

func (m *keycloakMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.resourceInputs[args.TypeToken] = args.Inputs
	return args.Name + "_id", args.Inputs, nil
}

func (m *keycloakMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{
		"id":    resource.NewStringProperty("test-realm-id"),
		"realm": resource.NewStringProperty("test-realm"),
	}, nil
}

func newKeycloakMocks() *keycloakMocks {
	return &keycloakMocks{
		resourceInputs: make(map[string]resource.PropertyMap),
	}
}

func TestDeployKeycloakClient(t *testing.T) {
	t.Run("deploys confidential client with service accounts", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		kc := &provisioningv1.KeycloakClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-kc-client",
			},
			Spec: provisioningv1.KeycloakClientSpec{
				ClientName:                "test-client",
				ClientId:                  "test-client-id",
				Realm:                     "test-realm",
				Enabled:                   true,
				PublicClient:              false,
				ServiceAccountsEnabled:    true,
				StandardFlowEnabled:       false,
				ImplicitFlowEnabled:       false,
				DirectAccessGrantsEnabled: false,
				FullScopeAllowed:          true,
				ConsentRequired:           false,
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
		mocks := newKeycloakMocks()

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			client, err := deployKeycloakClient(tenant, kc, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, client)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)

		clientInputs := mocks.resourceInputs["keycloak:openid/client:Client"]
		assert.Equal(t, "test-client-id", clientInputs["clientId"].StringValue())
		assert.Equal(t, "test-client", clientInputs["name"].StringValue())
		assert.Equal(t, "CONFIDENTIAL", clientInputs["accessType"].StringValue())
		assert.Equal(t, true, clientInputs["enabled"].BoolValue())
		assert.Equal(t, true, clientInputs["serviceAccountsEnabled"].BoolValue())
		assert.Equal(t, false, clientInputs["standardFlowEnabled"].BoolValue())
		assert.Equal(t, true, clientInputs["fullScopeAllowed"].BoolValue())
	})

	t.Run("deploys with protocol mappers", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		kc := &provisioningv1.KeycloakClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-kc-client",
			},
			Spec: provisioningv1.KeycloakClientSpec{
				ClientName: "test-client",
				ClientId:   "test-client-id",
				Realm:      "test-realm",
				Enabled:    true,
				ProtocolMappers: []provisioningv1.ProtocolMapper{
					{
						Name:           "audience-mapper",
						Protocol:       "openid-connect",
						ProtocolMapper: "oidc-audience-mapper",
						Config: map[string]string{
							"included.custom.audience": "agent-gateway",
							"access.token.claim":       "true",
						},
					},
				},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
		mocks := newKeycloakMocks()

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			client, err := deployKeycloakClient(tenant, kc, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, client)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)

		mapperInputs := mocks.resourceInputs["keycloak:index/genericProtocolMapper:GenericProtocolMapper"]
		assert.Equal(t, "audience-mapper", mapperInputs["name"].StringValue())
		assert.Equal(t, "oidc-audience-mapper", mapperInputs["protocolMapper"].StringValue())
	})

	t.Run("deploys with default and optional scopes", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		kc := &provisioningv1.KeycloakClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-kc-client",
			},
			Spec: provisioningv1.KeycloakClientSpec{
				ClientName:           "test-client",
				ClientId:             "test-client-id",
				Realm:                "test-realm",
				Enabled:              true,
				DefaultClientScopes:  []string{"profile", "email", "roles"},
				OptionalClientScopes: []string{"offline_access", "phone"},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
		mocks := newKeycloakMocks()

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			client, err := deployKeycloakClient(tenant, kc, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, client)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)

		defaultScopeInputs := mocks.resourceInputs["keycloak:openid/clientDefaultScopes:ClientDefaultScopes"]
		assert.NotNil(t, defaultScopeInputs)

		optionalScopeInputs := mocks.resourceInputs["keycloak:openid/clientOptionalScopes:ClientOptionalScopes"]
		assert.NotNil(t, optionalScopeInputs)
	})

	t.Run("deploys without scopes when none specified", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		kc := &provisioningv1.KeycloakClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-kc-client",
			},
			Spec: provisioningv1.KeycloakClientSpec{
				ClientName: "test-client",
				ClientId:   "test-client-id",
				Realm:      "test-realm",
				Enabled:    true,
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}
		mocks := newKeycloakMocks()

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			client, err := deployKeycloakClient(tenant, kc, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, client)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err)

		_, hasDefault := mocks.resourceInputs["keycloak:openid/clientDefaultScopes:ClientDefaultScopes"]
		assert.False(t, hasDefault)

		_, hasOptional := mocks.resourceInputs["keycloak:openid/clientOptionalScopes:ClientOptionalScopes"]
		assert.False(t, hasOptional)
	})
}
