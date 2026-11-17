package v1alpha1

import (
	"context"
	"os"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// TestDomainCRDValidation runs the generated CRD's defaulting and CEL rules, as the apiserver does
// on admission, so a regression in the kubebuilder markers or a stale helm/crds file fails here.
func TestDomainCRDValidation(t *testing.T) {
	schema := loadDomainSchema(t)

	tests := []struct {
		name    string
		spec    map[string]interface{}
		wantErr bool
	}{
		{"neither set", map[string]interface{}{"platformRef": "p"}, false},
		{"url without visibleInPortal", map[string]interface{}{"platformRef": "p", "url": "https://{{ .Tenant.Code }}.app"}, false},
		{"visibleInPortal false without url", map[string]interface{}{"platformRef": "p", "visibleInPortal": false}, false},
		{"visibleInPortal with url", map[string]interface{}{"platformRef": "p", "visibleInPortal": true, "url": "https://{{ .Tenant.Code }}.app"}, false},
		{"visibleInPortal without url", map[string]interface{}{"platformRef": "p", "visibleInPortal": true}, true},
		{"visibleInPortal with empty url", map[string]interface{}{"platformRef": "p", "visibleInPortal": true, "url": ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := map[string]interface{}{
				"apiVersion": SchemeGroupVersion.String(),
				"kind":       "Domain",
				"metadata":   map[string]interface{}{"name": "d", "namespace": "ns"},
				"spec":       tt.spec,
			}
			structuraldefaulting.Default(obj, schema)

			validator := cel.NewValidator(schema, true, celconfig.PerCallLimit)
			errs, _ := validator.Validate(context.TODO(), field.NewPath("root"), schema, obj, nil, celconfig.RuntimeCELCostBudget)

			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("wantErr %v, got errors: %v", tt.wantErr, errs)
			}
		})
	}

	t.Run("visibleInPortal defaults to false", func(t *testing.T) {
		obj := map[string]interface{}{"spec": map[string]interface{}{"platformRef": "p"}}
		structuraldefaulting.Default(obj, schema)

		if got := obj["spec"].(map[string]interface{})["visibleInPortal"]; got != false {
			t.Errorf("expected visibleInPortal to default to false, got %v", got)
		}
	})
}

func loadDomainSchema(t *testing.T) *structuralschema.Structural {
	t.Helper()

	data, err := os.ReadFile("../../../../helm/crds/platform.totalsoft.ro_domains.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}

	var internal apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &internal, nil); err != nil {
		t.Fatal(err)
	}

	schema, err := structuralschema.NewStructural(&internal)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
