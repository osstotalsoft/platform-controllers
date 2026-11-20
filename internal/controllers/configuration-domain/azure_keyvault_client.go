package configuration

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// listKeyVaultSecrets lists the secrets of the vault named vaultName: their names, tags and whether they are enabled,
// never their values. Tests replace it.
var listKeyVaultSecrets = listKeyVaultSecretsWithDefaultCredential

var (
	azureCredentialOnce sync.Once
	azureCredential     azcore.TokenCredential
	azureCredentialErr  error

	// keyVaultClients holds one *azsecrets.Client per vault name.
	keyVaultClients sync.Map
)

// listKeyVaultSecretsWithDefaultCredential lists the vault's secret properties through its data plane, signed in with
// the Azure SDK's default credential: workload identity (AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_FEDERATED_TOKEN_FILE)
// or a client secret (AZURE_CLIENT_SECRET). The identity needs Key Vault Reader on the vault. A credential that can't
// be created fails every listing, i.e. only the syncs of the domains whose platform uses Azure Key Vault.
func listKeyVaultSecretsWithDefaultCredential(ctx context.Context, vaultName string) ([]keyVaultSecret, error) {
	client, err := keyVaultClient(vaultName)
	if err != nil {
		return nil, err
	}

	var secrets []keyVaultSecret
	pager := client.NewListSecretPropertiesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot list the secrets of Azure Key Vault %s: %w", vaultName, err)
		}
		for _, props := range page.Value {
			if props == nil || props.ID == nil {
				continue
			}
			secret := keyVaultSecret{Name: props.ID.Name(), Tags: map[string]string{}, Enabled: true}
			if props.Attributes != nil && props.Attributes.Enabled != nil {
				secret.Enabled = *props.Attributes.Enabled
			}
			for k, v := range props.Tags {
				if v != nil {
					secret.Tags[k] = *v
				}
			}
			secrets = append(secrets, secret)
		}
	}
	return secrets, nil
}

func keyVaultClient(vaultName string) (*azsecrets.Client, error) {
	if client, ok := keyVaultClients.Load(vaultName); ok {
		return client.(*azsecrets.Client), nil
	}

	azureCredentialOnce.Do(func() {
		azureCredential, azureCredentialErr = azidentity.NewDefaultAzureCredential(nil)
	})
	if azureCredentialErr != nil {
		return nil, fmt.Errorf("cannot create the Azure credential: %w", azureCredentialErr)
	}

	client, err := azsecrets.NewClient(fmt.Sprintf("https://%s.vault.azure.net/", vaultName), azureCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create the client of Azure Key Vault %s: %w", vaultName, err)
	}
	actual, _ := keyVaultClients.LoadOrStore(vaultName, client)
	return actual.(*azsecrets.Client), nil
}

// secretNameFromID returns the name of the secret a Key Vault secret ID (https://<vault>.vault.azure.net/secrets/<name>[/<version>])
// identifies.
func secretNameFromID(id string) string {
	secretID := azsecrets.ID(id)
	return secretID.Name()
}
