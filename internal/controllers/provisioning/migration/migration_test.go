package migration

import (
	"context"
	"testing"

	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func TestKubeJobsMigrationForTenant(t *testing.T) {
	objects := []runtime.Object{
		newJob("dev1", "lsng", true),
		newJob("dev2", "lsng", true),
		newJob("dev5", "", true),
		newJob("dev3", "lsng", false),
	}
	kubeClient := fake.NewSimpleClientset(objects...)
	migrator := KubeJobsMigrationForTenant(kubeClient, func(s string, s2 string) bool {
		return true
	})
	t.Run("test job selection by label", func(t *testing.T) {
		migrator("test", newTenant("qa", "lsng", "qa"))
		jobs, _ := kubeClient.BatchV1().Jobs(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
		if len(jobs.Items) != 6 {
			t.Errorf("Error running migration, expected 6 jobs but found %d", len(jobs.Items))
		}
	})
}

func newJob(name, domain string, template bool) *v1.Job {
	j := &v1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1.JobSpec{},
	}
	if template {
		j.SetLabels(map[string]string{JobLabelSelectorKey: "true", DomainLabelSelectorKey: domain})
	}
	return j
}

func newTenant(name, domain, platform string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			PlatformRef: platform,
			DomainRefs:  []string{domain},
			Description: name + " description",
		},
	}
}
