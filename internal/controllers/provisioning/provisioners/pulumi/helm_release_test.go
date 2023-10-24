package pulumi

import (
	"encoding/json"
	"testing"
	"time"

	flux "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestHelmReleaseDeployFunc(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)
	hr := newHr("my-helm-release", platform)

	t.Run("maximal helm release spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			err := helmReleaseDeployFunc(tenant, []*provisioningv1.HelmRelease{hr})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release with nil upgrade spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Release.Upgrade = nil
			err := helmReleaseDeployFunc(tenant, []*provisioningv1.HelmRelease{hr})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release with nil exports spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Exports = nil
			err := helmReleaseDeployFunc(tenant, []*provisioningv1.HelmRelease{hr})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release with nil values spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Release.Values = nil
			err := helmReleaseDeployFunc(tenant, []*provisioningv1.HelmRelease{hr})(ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func newTenant(name, platform string) *provisioning.Tenant {
	tenant := provisioning.Tenant(platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			Description: name + " description",
			Id:          uuid.New().String(),
			Enabled:     true,
		},
	})

	return &tenant
}

func newHr(name, platform string) *provisioningv1.HelmRelease {
	false := false
	values := map[string]interface{}{
		"val1": "",
		"val2": 8,
		"val3": map[string]interface{}{
			"val1": "",
			"val2": 8,
		},
	}
	valuesBytes, _ := json.Marshal(values)

	hr := provisioningv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.HelmReleaseSpec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
			},
			Release: flux.HelmReleaseSpec{
				Interval: metav1.Duration{Duration: 10 * time.Second},
				Chart: flux.HelmChartTemplate{
					Spec: flux.HelmChartTemplateSpec{
						Chart:   "my-chart",
						Version: "1.0.0",
						SourceRef: flux.CrossNamespaceObjectReference{
							Kind:      "HelmRepository",
							Name:      "my-helm-repo",
							Namespace: metav1.NamespaceDefault,
						},
					},
				},
				ReleaseName: "my-helm-release",
				Upgrade: &flux.Upgrade{
					Remediation: &flux.UpgradeRemediation{
						RemediateLastFailure: &(false),
					},
				},
				Values: &apiextensionsv1.JSON{
					Raw: valuesBytes,
				},
			},
		},
	}
	return &hr
}
