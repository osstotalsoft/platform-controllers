package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestDeployLocalScript(t *testing.T) {
	t.Run("maximal entra user spec", func(t *testing.T) {
		platform := "dev"
		tenant := newTenant("tenant1", platform)
		script := &provisioningv1.LocalScript{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-pwsh-script",
			},
			Spec: provisioningv1.LocalScriptSpec{
				CreateScriptContent: "Write-Host 'Hello, World!'",
				DeleteScriptContent: "Write-Host 'Goodbye, World!'",
				Shell:               provisioningv1.LocalScriptShellPwsh,
				Environment:         map[string]string{"key": "value"},
				ProvisioningMeta: provisioningv1.ProvisioningMeta{
					DomainRef: "example-domain",
				},
			},
		}

		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			script, err := deployLocalScript(tenant, script, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			assert.NotNil(t, script)
			return nil

		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}
