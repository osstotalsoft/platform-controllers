package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Keycloak client name",type=string,JSONPath=`.spec.clientName`
// +kubebuilder:printcolumn:name="Platform",type=string,JSONPath=`.spec.platformRef`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainRef`

type KeycloakClient struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KeycloakClientSpec `json:"spec"`
}

type KeycloakClientSpec struct {
	// ClientName represents the client name.
	ClientName string `json:"clientName"`

	// ClientId represents the client id.
	ClientId string `json:"clientId"`

	// Associated realm.
	Realm string `json:"realm"`

	// Associated tenant name.
	Organization string `json:"organization,omitempty"`

	// Enabled indicates if the client is enabled.
	Enabled bool `json:"enabled"`

	// ConsentRequired indicates if consent is required.
	ConsentRequired bool `json:"consentRequired"`

	// PublicClient indicates if this is a public client.
	PublicClient bool `json:"publicClient"`

	// ServiceAccountsEnabled enables service accounts.
	ServiceAccountsEnabled bool `json:"serviceAccountsEnabled"`

	// StandardFlowEnabled enables the standard (authorization code) flow.
	StandardFlowEnabled bool `json:"standardFlowEnabled"`

	// ImplicitFlowEnabled enables the implicit flow.
	ImplicitFlowEnabled bool `json:"implicitFlowEnabled"`

	// DirectAccessGrantsEnabled enables direct access grants (resource owner password).
	DirectAccessGrantsEnabled bool `json:"directAccessGrantsEnabled"`

	// Protocol e.g. "openid-connect"
	Protocol string `json:"protocol,omitempty"`

	// FullScopeAllowed allows full scope.
	FullScopeAllowed bool `json:"fullScopeAllowed"`

	// ProtocolMappers defines protocol mapper configurations.
	ProtocolMappers []ProtocolMapper `json:"protocolMappers,omitempty"`

	// DefaultClientScopes is the list of default client scopes.
	DefaultClientScopes []string `json:"defaultClientScopes,omitempty"`

	// OptionalClientScopes is the list of optional client scopes.
	OptionalClientScopes []string `json:"optionalClientScopes,omitempty"`

	// Export provisioning values spec.
	// +optional
	Exports          []KeycloakClientExportsSpec `json:"exports,omitempty"`
	ProvisioningMeta `json:",inline"`
}

// ProtocolMapper represents a Keycloak protocol mapper configuration.
type ProtocolMapper struct {
	// Name of the protocol mapper.
	Name string `json:"name"`

	// Protocol e.g. "openid-connect"
	Protocol string `json:"protocol"`

	// ProtocolMapper is the mapper type e.g. "oidc-audience-mapper"
	ProtocolMapper string `json:"protocolMapper"`

	// Config is a key-value map of mapper configuration.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type KeycloakClientExportsSpec struct {
	// The domain or bounded-context in which this client will be used.
	Domain string `json:"domain"`

	ClientId     ValueExport `json:"clientId"`
	ClientSecret ValueExport `json:"clientSecret"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KeycloakClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KeycloakClient `json:"items"`
}

func (kc *KeycloakClient) GetProvisioningMeta() *ProvisioningMeta {
	return &kc.Spec.ProvisioningMeta
}

func (kc *KeycloakClient) GetSpec() any {
	return &kc.Spec
}
