package v1alpha1

import (
	"os"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// TestDomainCRD checks the generated CRD carries the visibleInPortal default and the url rule,
// so a removed kubebuilder marker or a stale helm/crds file fails here.
func TestDomainCRD(t *testing.T) {
	data, err := os.ReadFile("../../../../helm/crds/platform.totalsoft.ro_domains.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]

	if _, ok := spec.Properties["url"]; !ok {
		t.Error("expected spec.url in the CRD schema")
	}

	visibleInPortal, ok := spec.Properties["visibleInPortal"]
	if !ok {
		t.Fatal("expected spec.visibleInPortal in the CRD schema")
	}
	if visibleInPortal.Default == nil || string(visibleInPortal.Default.Raw) != "false" {
		t.Errorf("expected visibleInPortal to default to false, got %v", visibleInPortal.Default)
	}

	const wantRule = "!has(self.visibleInPortal) || !self.visibleInPortal || (has(self.url) && self.url != '')"
	found := false
	for _, v := range spec.XValidations {
		if v.Rule == wantRule {
			found = true
		}
	}
	if !found {
		t.Errorf("expected spec rule %q, got %+v", wantRule, spec.XValidations)
	}
}
