package pulumi

import (
	"fmt"

	"encoding/json"

	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	fluxcd "totalsoft.ro/platform-controllers/internal/controllers/provisioning/provisioners/pulumi/fluxcd/kubernetes/helm/v2beta1"
	"totalsoft.ro/platform-controllers/internal/template"
	"totalsoft.ro/platform-controllers/internal/tuple"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployHelmRelease(target provisioning.ProvisioningTarget,
	hr *provisioningv1.HelmRelease,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*fluxcd.HelmRelease, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("HelmRelease")

	args, err := pulumiFluxHrArgs(target, hr)
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
	ctx.Export(fmt.Sprintf("helmRelease:%s", hr.Name), fluxHr.Spec.ReleaseName().Elem())

	return fluxHr, nil
}

func pulumiFluxHrArgs(target provisioning.ProvisioningTarget, hr *provisioningv1.HelmRelease) (*fluxcd.HelmReleaseArgs, error) {
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
		json.Unmarshal([]byte(valuesJson), &values)
		pulumiValues = pulumi.ToMap(values)
	}

	args := fluxcd.HelmReleaseArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fluxHelmReleaseName),
			Namespace: pulumi.String(hr.Namespace),
		},
		Spec: fluxcd.HelmReleaseSpecArgs{
			Chart: fluxcd.HelmReleaseSpecChartArgs{
				Spec: fluxcd.HelmReleaseSpecChartSpecArgs{
					Chart:   pulumi.String(hr.Spec.Release.Chart.Spec.Chart),
					Version: pulumi.String(hr.Spec.Release.Chart.Spec.Version),
					SourceRef: fluxcd.HelmReleaseSpecChartSpecSourcerefArgs{
						Kind:      pulumi.String(hr.Spec.Release.Chart.Spec.SourceRef.Kind),
						Name:      pulumi.String(hr.Spec.Release.Chart.Spec.SourceRef.Name),
						Namespace: pulumi.String(hr.Spec.Release.Chart.Spec.SourceRef.Namespace),
					},
				},
			},
			Interval:    pulumi.String(hr.Spec.Release.Interval.Duration.String()),
			ReleaseName: pulumi.String(helmReleaseName),
			Upgrade: fluxcd.HelmReleaseSpecUpgradeArgs{
				Remediation: fluxcd.HelmReleaseSpecUpgradeRemediationArgs{
					RemediateLastFailure: pulumi.Bool(hr.Spec.Release.GetUpgrade().GetRemediation().MustRemediateLastFailure()),
				},
			},
			Values: pulumiValues,
		},
	}

	return &args, nil
}
