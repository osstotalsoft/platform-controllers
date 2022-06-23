package migration

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	JobLabelSelectorKey = "provisioning.totalsoft.ro/migration-job-template"
)

func KubeJobsMigrationForTenant(kubeClient kubernetes.Interface,
	nsFilter func(string, string) bool) func(platform string, tenant *platformv1.Tenant) error {
	namer := func(jName, tenant string) string {
		return fmt.Sprintf("%s-%s-%d", jName, tenant, time.Now().Unix())
	}

	return func(platform string, tenant *platformv1.Tenant) error {
		klog.InfoS("Creating migrations jobs", "tenant", tenant.Name)

		jobs, err := kubeClient.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: JobLabelSelectorKey + "=true",
		})
		if err != nil {
			return err
		}

		klog.V(4).InfoS("migrations", "template jobs found", len(jobs.Items))

		for _, job := range jobs.Items {
			if !nsFilter(job.Namespace, platform) {
				continue
			}
			j := &v1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namer(job.Name, tenant.Name),
					Namespace: job.Namespace,
				},
				Spec: v1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: job.Spec.Template.Spec,
					},
				},
			}

			for i, c := range j.Spec.Template.Spec.InitContainers {
				c.Env = append(c.Env, corev1.EnvVar{Name: "TENANT_ID", Value: tenant.Spec.Id})
				j.Spec.Template.Spec.InitContainers[i] = c
			}
			for i, c := range j.Spec.Template.Spec.Containers {
				c.Env = append(c.Env, corev1.EnvVar{Name: "TENANT_ID", Value: tenant.Spec.Id})
				j.Spec.Template.Spec.Containers[i] = c
			}
			_, err = kubeClient.BatchV1().Jobs(job.Namespace).Create(context.TODO(), j, metav1.CreateOptions{})
			if err != nil {
				klog.ErrorS(err, "error creating migration job")
			}
		}

		return err
	}
}
