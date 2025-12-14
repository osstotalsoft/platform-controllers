package migration

import (
	"context"
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
			hasTenantID := false
			for _, container := range job.Spec.Template.Spec.Containers {
				for _, env := range container.Env {
					if env.Name == "TENANT_ID" {
						hasTenantID = true
						break
					}
				}
			}
			if hasTenantID {
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
			PlatformRef: platform,
			Description: name + " description",
		},
	}
}
