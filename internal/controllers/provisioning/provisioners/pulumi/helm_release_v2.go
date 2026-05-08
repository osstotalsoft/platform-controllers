package pulumi

import (
	"encoding/json"
	"fmt"

	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	fluxcd "totalsoft.ro/platform-controllers/internal/controllers/provisioning/provisioners/pulumi/fluxcd/generated/kubernetes/helm/v2"
	"totalsoft.ro/platform-controllers/internal/template"
	"totalsoft.ro/platform-controllers/internal/tuple"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployHelmReleaseV2(target provisioning.ProvisioningTarget,
	hr *provisioningv1.HelmReleaseV2,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*fluxcd.HelmRelease, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("HelmReleaseV2")

	args, err := pulumiFluxHrV2Args(target, hr)
	if err != nil {
		return nil, err
	}

	fluxHr, err := fluxcd.NewHelmRelease(ctx, hr.Name, args, pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	for _, exp := range hr.Spec.Exports {
		err = valueExporter(newExportContext(ctx, exp.Domain, hr.Name, hr.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{"releaseName": {exp.ReleaseName, fluxHr.Spec.ReleaseName().Elem()}})
		if err != nil {
			return nil, err
		}
	}
	ctx.Export(fmt.Sprintf("helmReleaseV2:%s", hr.Name), fluxHr.Spec.ReleaseName().Elem())

	return fluxHr, nil
}

func pulumiFluxHrV2Args(target provisioning.ProvisioningTarget, hr *provisioningv1.HelmReleaseV2) (*fluxcd.HelmReleaseArgs, error) {
	helmReleaseName, fluxHelmReleaseName := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) tuple.T2[string, string] {
			helmReleaseName := fmt.Sprintf("%s-%s", hr.Spec.Release.ReleaseName, tenant.GetName())
			fluxHelmReleaseName := fmt.Sprintf("%s-%s", hr.Name, tenant.GetName())
			return tuple.New2(helmReleaseName, fluxHelmReleaseName)
		}, func(*platformv1.Platform) tuple.T2[string, string] {
			helmReleaseName := hr.Spec.Release.ReleaseName
			fluxHelmReleaseName := hr.Name
			return tuple.New2(helmReleaseName, fluxHelmReleaseName)
		},
	).Values()

	pulumiValues := pulumi.Map{}
	if hr.Spec.Release.Values != nil {
		tc := provisioning.GetTemplateContext(target)
		valuesJson := string(hr.Spec.Release.Values.Raw)
		valuesJson, err := template.ParseTemplate(valuesJson, tc)
		if err != nil {
			return nil, err
		}

		var values map[string]interface{}
		if err := json.Unmarshal([]byte(valuesJson), &values); err != nil {
			return nil, fmt.Errorf("failed to unmarshal templated Helm values as JSON: %w", err)
		}
		pulumiValues = pulumi.ToMap(values)
	}

	spec := fluxcd.HelmReleaseSpecArgs{
		Interval:    pulumi.String(hr.Spec.Release.Interval.Duration.String()),
		ReleaseName: pulumi.String(helmReleaseName),
		Values:      pulumiValues,
	}

	switch {
	case hr.Spec.Release.Chart != nil:
		sourceRef := fluxcd.HelmReleaseSpecChartSpecSourceRefArgs{
			Kind: pulumi.String(hr.Spec.Release.Chart.Spec.SourceRef.Kind),
			Name: pulumi.String(hr.Spec.Release.Chart.Spec.SourceRef.Name),
		}
		if hr.Spec.Release.Chart.Spec.SourceRef.Namespace != "" {
			sourceRef.Namespace = pulumi.StringPtr(hr.Spec.Release.Chart.Spec.SourceRef.Namespace)
		}
		spec.Chart = fluxcd.HelmReleaseSpecChartArgs{
			Spec: fluxcd.HelmReleaseSpecChartSpecArgs{
				Chart:     pulumi.String(hr.Spec.Release.Chart.Spec.Chart),
				Version:   pulumi.String(hr.Spec.Release.Chart.Spec.Version),
				SourceRef: sourceRef,
			},
		}
	case hr.Spec.Release.ChartRef != nil:
		chartRef := fluxcd.HelmReleaseSpecChartRefArgs{
			Kind: pulumi.StringPtr(hr.Spec.Release.ChartRef.Kind),
			Name: pulumi.StringPtr(hr.Spec.Release.ChartRef.Name),
		}
		if hr.Spec.Release.ChartRef.APIVersion != "" {
			chartRef.ApiVersion = pulumi.StringPtr(hr.Spec.Release.ChartRef.APIVersion)
		}
		if hr.Spec.Release.ChartRef.Namespace != "" {
			chartRef.Namespace = pulumi.StringPtr(hr.Spec.Release.ChartRef.Namespace)
		}
		spec.ChartRef = chartRef
	default:
		return nil, fmt.Errorf("helm release %q must define either spec.release.chart or spec.release.chartRef", hr.Name)
	}

	if hr.Spec.Release.Upgrade != nil {
		spec.Upgrade = fluxcd.HelmReleaseSpecUpgradeArgs{
			Remediation: fluxcd.HelmReleaseSpecUpgradeRemediationArgs{
				RemediateLastFailure: pulumi.Bool(hr.Spec.Release.Upgrade.GetRemediation().MustRemediateLastFailure()),
			},
		}
	}

	args := fluxcd.HelmReleaseArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fluxHelmReleaseName),
			Namespace: pulumi.String(hr.Namespace),
		},
		Spec: spec,
	}

	return &args, nil
}
