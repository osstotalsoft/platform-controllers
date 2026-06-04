package pulumi

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-keycloak/sdk/v6/go/keycloak"
	"github.com/pulumi/pulumi-keycloak/sdk/v6/go/keycloak/openid"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	"totalsoft.ro/platform-controllers/internal/template"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployKeycloakClient(target provisioning.ProvisioningTarget,
	keycloakClient *provisioningv1.KeycloakClient,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*openid.Client, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("KeycloakClient")

	spec := keycloakClient.Spec
	realm, err := keycloak.LookupRealm(ctx, &keycloak.LookupRealmArgs{
		Realm: spec.Realm,
	}, nil)
	if err != nil {
		return nil, err
	}

	// Determine access type based on PublicClient
	accessType := "CONFIDENTIAL"
	if spec.PublicClient {
		accessType = "PUBLIC"
	}

	clientName := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s", spec.ClientName, tenant.GetName())
		},
		func(platform *platformv1.Platform) string { return spec.ClientName },
	)

	clientId := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s", spec.ClientId, tenant.GetName())
		},
		func(platform *platformv1.Platform) string { return spec.ClientName },
	)

	client, err := openid.NewClient(ctx, keycloakClient.Name, &openid.ClientArgs{
		RealmId:                   pulumi.String(realm.Id),
		ClientId:                  pulumi.String(clientId),
		Name:                      pulumi.String(clientName),
		Enabled:                   pulumi.Bool(spec.Enabled),
		AccessType:                pulumi.String(accessType),
		ConsentRequired:           pulumi.Bool(spec.ConsentRequired),
		ServiceAccountsEnabled:    pulumi.Bool(spec.ServiceAccountsEnabled),
		StandardFlowEnabled:       pulumi.Bool(spec.StandardFlowEnabled),
		ImplicitFlowEnabled:       pulumi.Bool(spec.ImplicitFlowEnabled),
		DirectAccessGrantsEnabled: pulumi.Bool(spec.DirectAccessGrantsEnabled),
		FullScopeAllowed:          pulumi.Bool(spec.FullScopeAllowed),
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	// Deploy protocol mappers
	for _, pm := range spec.ProtocolMappers {
		config := pulumi.StringMap{}
		for k, v := range pm.Config {
			config[k] = pulumi.String(v)
		}
		_, err = keycloak.NewGenericProtocolMapper(ctx,
			fmt.Sprintf("%s-%s", keycloakClient.Name, pm.Name),
			&keycloak.GenericProtocolMapperArgs{
				RealmId:        pulumi.String(realm.Id),
				ClientId:       client.ID(),
				Name:           pulumi.String(pm.Name),
				Protocol:       pulumi.String(pm.Protocol),
				ProtocolMapper: pulumi.String(pm.ProtocolMapper),
				Config:         config,
			}, pulumi.Parent(client))
		if err != nil {
			return nil, err
		}
	}

	// Assign default client scopes
	if len(spec.DefaultClientScopes) > 0 {
		scopes := make(pulumi.StringArray, len(spec.DefaultClientScopes))
		for i, s := range spec.DefaultClientScopes {
			scopes[i] = pulumi.String(s)
		}
		_, err = openid.NewClientDefaultScopes(ctx,
			fmt.Sprintf("%s-default-scopes", keycloakClient.Name),
			&openid.ClientDefaultScopesArgs{
				RealmId:       pulumi.String(realm.Id),
				ClientId:      client.ID(),
				DefaultScopes: scopes,
			}, pulumi.Parent(client))
		if err != nil {
			return nil, err
		}
	}

	// Assign optional client scopes
	if len(spec.OptionalClientScopes) > 0 {
		scopes := make(pulumi.StringArray, len(spec.OptionalClientScopes))
		for i, s := range spec.OptionalClientScopes {
			scopes[i] = pulumi.String(s)
		}
		_, err = openid.NewClientOptionalScopes(ctx,
			fmt.Sprintf("%s-optional-scopes", keycloakClient.Name),
			&openid.ClientOptionalScopesArgs{
				RealmId:        pulumi.String(realm.Id),
				ClientId:       client.ID(),
				OptionalScopes: scopes,
			}, pulumi.Parent(client))
		if err != nil {
			return nil, err
		}
	}

	if spec.Organization != "" {
		// Get the service account user associated with this client
		saUser := openid.GetClientServiceAccountUserOutput(ctx, openid.GetClientServiceAccountUserOutputArgs{
			RealmId:  pulumi.String(realm.Id),
			ClientId: client.ID().ToStringOutput(),
		}, nil)

		// Use Keycloak REST API to add service account to organization
		// POST /admin/realms/{realm}/organizations/{orgId}/members
		keycloakURL := os.Getenv("KEYCLOAK_URL")
		keycloakClientID := os.Getenv("KEYCLOAK_CLIENT_ID")
		keycloakClientSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET")

		if keycloakURL == "" || keycloakClientID == "" || keycloakClientSecret == "" {
			return nil, fmt.Errorf("KEYCLOAK_URL, KEYCLOAK_CLIENT_ID and KEYCLOAK_CLIENT_SECRET environment variables must be set")
		}
		tc := provisioning.GetTemplateContext(target)
		orgId, err := template.ParseTemplate(spec.Organization, tc)

		createCmd := saUser.Id().ApplyT(func(userId string) string {
			return fmt.Sprintf(
				`TOKEN=$(curl -s -X POST "$KEYCLOAK_URL/realms/master/protocol/openid-connect/token" `+
					`-d "grant_type=client_credentials&client_id=$KEYCLOAK_CLIENT_ID&client_secret=$KEYCLOAK_CLIENT_SECRET" | jq -r .access_token) && `+
					`ORG_ID=$(curl -s -X GET "$KEYCLOAK_URL/admin/realms/%s/organizations?q=tid:%s" `+
					`-H "Authorization: Bearer $TOKEN" | jq -r '.[0].id') && `+
					`curl -s -X POST "$KEYCLOAK_URL/admin/realms/%s/organizations/$ORG_ID/members" `+
					`-H "Authorization: Bearer $TOKEN" `+
					`-H "Content-Type: application/json" -d '"%s"'`,
				spec.Realm, orgId, spec.Realm, userId,
			)
		}).(pulumi.StringOutput)

		//interpreter := []string{"/bin/sh", "-c"}
		//if runtime.GOOS == "windows" {
		//	interpreter = []string{"C:\\Program Files\\Git\\bin\\bash.exe", "-c"}
		//}

		_, err = local.NewCommand(ctx,
			fmt.Sprintf("%s-add-member-organization", keycloakClient.Name),
			&local.CommandArgs{
				Create: createCmd,
				//Interpreter: pulumi.ToStringArray(interpreter),
				Environment: pulumi.StringMap{
					"KEYCLOAK_URL":           pulumi.String(keycloakURL),
					"KEYCLOAK_CLIENT_ID":     pulumi.String(keycloakClientID),
					"KEYCLOAK_CLIENT_SECRET": pulumi.String(keycloakClientSecret),
				},
			}, pulumi.Parent(client))
		if err != nil {
			return nil, err
		}
	}

	// Export values
	for _, exp := range spec.Exports {
		domain := exp.Domain
		if domain == "" {
			domain = spec.DomainRef
		}

		err = valueExporter(newExportContext(ctx, domain, "keycloak-"+keycloakClient.Name, keycloakClient.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{
				"clientId":     {exp.ClientId, client.ClientId},
				"clientSecret": {exp.ClientSecret, client.ClientSecret},
			})
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}
