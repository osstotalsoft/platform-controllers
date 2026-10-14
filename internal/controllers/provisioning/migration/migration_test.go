package migration

import (
	"context"
	"reflect"
	"testing"

	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func TestKubeJobsMigrationForTenant(t *testing.T) {
	domain := "test-domain"
	objects := []runtime.Object{
		newJob("dev1", domain, true),
		newJob("dev2", domain, true),
		newJob("dev3", domain, false),
		newJob("dev4", "some-other-domain", true),
	}
	kubeClient := fake.NewSimpleClientset(objects...)
	ttlSecondsAfterFinished := int32(3600) // 1 hour for testing
	migrator := KubeJobsMigrationForTenant(kubeClient, func(s string, s2 string) bool {
		return true
	}, ttlSecondsAfterFinished)
	t.Run("test job selection by label", func(t *testing.T) {
		migrator("test", newTenant("qa", "qa"), domain)
		jobs, _ := kubeClient.BatchV1().Jobs(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
		expectedNoOfJobs := 4 + 2 //4 existing + 2 new jobs
		if len(jobs.Items) != expectedNoOfJobs {
			t.Errorf("Error running migration, expected %d jobs but found %d", expectedNoOfJobs, len(jobs.Items))
		}
	})
	t.Run("test ttlSecondsAfterFinished is set", func(t *testing.T) {
		jobs, _ := kubeClient.BatchV1().Jobs(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
		newJobsCount := 0
		for _, job := range jobs.Items {
			// Check only newly created jobs (those with TENANT_ID environment variable)
			if hasTenantIDEnvVar(&job) {
				newJobsCount++
				if job.Spec.TTLSecondsAfterFinished == nil {
					t.Errorf("TTLSecondsAfterFinished not set for job %s", job.Name)
				} else if *job.Spec.TTLSecondsAfterFinished != ttlSecondsAfterFinished {
					t.Errorf("Expected TTLSecondsAfterFinished to be %d but got %d for job %s", 
						ttlSecondsAfterFinished, *job.Spec.TTLSecondsAfterFinished, job.Name)
				}
			}
		}
		if newJobsCount != 2 {
			t.Errorf("Expected 2 new jobs to be created but found %d", newJobsCount)
		}
	})
}

// The pod metadata of a live template Job, as found in qa-hcm (charisma-hcm-migrations): the Job
// controller added its own controller-uid / job-name labels, the chart added app labels.
func liveTemplatePodMetadata() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Labels: map[string]string{
			"app.kubernetes.io/instance":         "charisma-hcm",
			"app.kubernetes.io/name":             "charisma-hcm-migrations",
			"batch.kubernetes.io/controller-uid": "963d14ad-e3cd-409c-8b51-364b6a93d903",
			"batch.kubernetes.io/job-name":       "charisma-hcm-migrations",
			"controller-uid":                     "963d14ad-e3cd-409c-8b51-364b6a93d903",
			"job-name":                           "charisma-hcm-migrations",
		},
		Annotations: map[string]string{
			"rusi.io/enabled": "true",
		},
	}
}

func TestKubeJobsMigrationForTenantPodMetadata(t *testing.T) {
	domain := "test-domain"

	t.Run("workload identity labels and annotations are copied, nothing else", func(t *testing.T) {
		template := newJob("wi", domain, true)
		template.Spec.Template.ObjectMeta = liveTemplatePodMetadata()
		template.Spec.Template.Labels["azure.workload.identity/use"] = "true"
		template.Spec.Template.Annotations["azure.workload.identity/skip-containers"] = "istio-proxy"

		clone := migrateSingleTemplate(t, template, domain)

		expectedLabels := map[string]string{"azure.workload.identity/use": "true"}
		if !reflect.DeepEqual(clone.Spec.Template.Labels, expectedLabels) {
			t.Errorf("expected clone pod labels %v but got %v", expectedLabels, clone.Spec.Template.Labels)
		}
		expectedAnnotations := map[string]string{"azure.workload.identity/skip-containers": "istio-proxy"}
		if !reflect.DeepEqual(clone.Spec.Template.Annotations, expectedAnnotations) {
			t.Errorf("expected clone pod annotations %v but got %v", expectedAnnotations, clone.Spec.Template.Annotations)
		}
	})

	t.Run("a template without workload identity metadata is cloned as before", func(t *testing.T) {
		template := newJob("no-wi", domain, true)
		template.Spec.Template.ObjectMeta = liveTemplatePodMetadata()

		clone := migrateSingleTemplate(t, template, domain)

		if clone.Spec.Template.Labels != nil || clone.Spec.Template.Annotations != nil {
			t.Errorf("expected no clone pod metadata but got labels %v, annotations %v",
				clone.Spec.Template.Labels, clone.Spec.Template.Annotations)
		}
	})
}

// migrateSingleTemplate runs the migration for one tenant against the single template Job given,
// and returns the per-tenant clone it created.
func migrateSingleTemplate(t *testing.T, template *v1.Job, domain string) v1.Job {
	t.Helper()
	kubeClient := fake.NewSimpleClientset(template)
	migrator := KubeJobsMigrationForTenant(kubeClient, func(string, string) bool { return true }, DefaultTTLSecondsAfterFinished)
	if err := migrator("test", newTenant("mf", "qa"), domain); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	jobs, _ := kubeClient.BatchV1().Jobs(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
	var clones []v1.Job
	for _, job := range jobs.Items {
		if hasTenantIDEnvVar(&job) {
			clones = append(clones, job)
		}
	}
	if len(clones) != 1 {
		t.Fatalf("expected 1 clone but found %d", len(clones))
	}
	return clones[0]
}

func hasTenantIDEnvVar(job *v1.Job) bool {
	for _, container := range job.Spec.Template.Spec.InitContainers {
		for _, env := range container.Env {
			if env.Name == "TENANT_ID" {
				return true
			}
		}
	}
	for _, container := range job.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == "TENANT_ID" {
				return true
			}
		}
	}
	return false
}

func newJob(name, domain string, template bool) *v1.Job {
	j := &v1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
						},
					},
				},
			},
		},
	}
	if template {
		j.SetLabels(map[string]string{
			jobTemplateLabel: "true",
			domainLabel:      domain,
		})
	}
	return j
}

func newTenant(name, platform string) *platformv1.Tenant {
	return &platformv1.Tenant{
		TypeMeta: metav1.TypeMeta{APIVersion: platformv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: platformv1.TenantSpec{
			Id:          name,
			PlatformRef: platform,
			Description: name + " description",
		},
	}
}
