package pulumi

import (
	"encoding/json"
	"testing"
	"time"

	fluxv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fluxcd "totalsoft.ro/platform-controllers/internal/controllers/provisioning/provisioners/pulumi/fluxcd/generated/kubernetes/helm/v2"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func TestHelmReleaseV2DeployFunc(t *testing.T) {
	platform := "dev"
	tenant := newTenant("tenant1", platform)
	hr := newHrV2("my-helm-release-v2", platform)

	t.Run("maximal helm release v2 spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := deployHelmReleaseV2(tenant, hr, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release v2 with nil upgrade spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Release.Upgrade = nil
			_, err := deployHelmReleaseV2(tenant, hr, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release v2 with nil exports spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Exports = nil
			_, err := deployHelmReleaseV2(tenant, hr, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})

	t.Run("helm release v2 with nil values spec", func(t *testing.T) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			hr := hr.DeepCopy()
			hr.Spec.Release.Values = nil
			_, err := deployHelmReleaseV2(tenant, hr, []pulumi.Resource{}, ctx)
			assert.NoError(t, err)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func TestPulumiFluxHrV2ArgsOmitsUpgradeWhenNotConfigured(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	hr := newHrV2("my-helm-release-v2", "dev")
	hr.Spec.Release.Upgrade = nil

	args, err := pulumiFluxHrV2Args(tenant, hr)

	assert.NoError(t, err)
	spec, ok := args.Spec.(fluxcd.HelmReleaseSpecArgs)
	assert.True(t, ok)
	assert.Nil(t, spec.Upgrade)
}

func TestPulumiFluxHrV2ArgsUsesChartRefWhenConfigured(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	hr := newHrV2("my-helm-release-v2", "dev")
	hr.Spec.Release.Chart = nil
	hr.Spec.Release.ChartRef = &fluxv2.CrossNamespaceSourceReference{
		APIVersion: "source.toolkit.fluxcd.io/v1",
		Kind:       "OCIRepository",
		Name:       "my-chart",
		Namespace:  "flux-system",
	}

	args, err := pulumiFluxHrV2Args(tenant, hr)

	assert.NoError(t, err)
	spec, ok := args.Spec.(fluxcd.HelmReleaseSpecArgs)
	assert.True(t, ok)
	assert.Nil(t, spec.Chart)
	assert.IsType(t, fluxcd.HelmReleaseSpecChartRefArgs{}, spec.ChartRef)
	assert.NotNil(t, spec.ChartRef)
}

func TestPulumiFluxHrV2ArgsErrorsWhenChartConfigurationMissing(t *testing.T) {
	tenant := newTenant("tenant1", "dev")
	hr := newHrV2("my-helm-release-v2", "dev")
	hr.Spec.Release.Chart = nil
	hr.Spec.Release.ChartRef = nil

	args, err := pulumiFluxHrV2Args(tenant, hr)

	assert.Nil(t, args)
	assert.ErrorContains(t, err, "must define either spec.release.chart or spec.release.chartRef")
}

func newHrV2(name, platform string) *provisioningv1.HelmReleaseV2 {
	remediateLastFailure := false
	values := map[string]interface{}{
		"val1": "",
		"val2": 8,
		"val3": map[string]interface{}{
			"val1": "",
			"val2": 8,
		},
	}
	valuesBytes, _ := json.Marshal(values)

	hr := provisioningv1.HelmReleaseV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: provisioningv1.HelmReleaseV2Spec{
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
			},
			Release: fluxv2.HelmReleaseSpec{
				Interval: metav1.Duration{Duration: 10 * time.Second},
				Chart: &fluxv2.HelmChartTemplate{
					Spec: fluxv2.HelmChartTemplateSpec{
						Chart:   "my-chart",
						Version: "1.0.0",
						SourceRef: fluxv2.CrossNamespaceObjectReference{
							Kind:      "HelmRepository",
							Name:      "my-helm-repo",
							Namespace: metav1.NamespaceDefault,
						},
					},
				},
				ReleaseName: "my-helm-release",
				Upgrade: &fluxv2.Upgrade{
					Remediation: &fluxv2.UpgradeRemediation{
						RemediateLastFailure: &remediateLastFailure,
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
