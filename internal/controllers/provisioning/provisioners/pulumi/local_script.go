package pulumi

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	"totalsoft.ro/platform-controllers/internal/template"

	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployLocalScript(target provisioning.ProvisioningTarget,
	localScript *provisioningv1.LocalScript,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*local.Command, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("LocalScript")

	tc := provisioning.GetTemplateContext(target)

	for key, value := range localScript.Spec.Environment {
		parsedValue, err := template.ParseTemplate(value, tc)
		if err != nil {
			return nil, err
		}
		localScript.Spec.Environment[key] = parsedValue
	}

	args := &local.CommandArgs{
		Create:      pulumi.String(localScript.Spec.CreateScriptContent),
		Delete:      pulumi.String(localScript.Spec.DeleteScriptContent),
		Environment: pulumi.ToStringMap(localScript.Spec.Environment),
		Dir:         pulumi.String(localScript.Spec.WorkingDir),
		Triggers:    pulumi.ToArray([]any{localScript.Spec.ForceUpdateTag}), // caution: performs delete-replace and triggers the Delete script
	}

	switch localScript.Spec.Shell {
	case provisioningv1.LocalScriptShellPwsh:
		args.Interpreter = pulumi.ToStringArray([]string{"pwsh", "-c"})
	case provisioningv1.LocalScriptShellSh:
		args.Interpreter = pulumi.ToStringArray([]string{"sh", "-c"})
	}

	script, err := local.NewCommand(ctx, localScript.Name, args,
		pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	for _, exp := range localScript.Spec.Exports {
		err = valueExporter(newExportContext(ctx, exp.Domain, localScript.Name, localScript.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{"scriptOutput": {exp.ScriptOutput, script.Stdout}})
		if err != nil {
			return nil, err
		}
	}
	return script, nil
}
