package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	jobTemplateLabel = "provisioning.totalsoft.ro/migration-job-template"
	domainLabel      = "platform.totalsoft.ro/domain"
	
	// DefaultTTLSecondsAfterFinished is the default TTL (in seconds) for migration jobs.
	// The default value is 86400 seconds (24 hours).
	DefaultTTLSecondsAfterFinished int32 = 86400
)

func KubeJobsMigrationForTenant(kubeClient kubernetes.Interface,
	nsFilter func(string, string) bool, ttlSecondsAfterFinished int32) func(platform string, tenant *platformv1.Tenant, domain string) error {
	namer := func(jName, tenant string) string {
		return fmt.Sprintf("%s-%s-%d", jName, tenant, time.Now().Unix())
	}

	return func(platform string, tenant *platformv1.Tenant, domain string) error {
		klog.InfoS("Creating migrations jobs", "tenant", tenant.Name, "domain", domain)

		labelSelector, err := labels.ValidatedSelectorFromSet(map[string]string{
			domainLabel:      domain,
			jobTemplateLabel: "true",
		})
		if err != nil {
			return err
		}

		jobs, err := kubeClient.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector.String(),
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
					TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      workloadIdentityMetadata(job.Spec.Template.Labels),
							Annotations: workloadIdentityMetadata(job.Spec.Template.Annotations),
						},
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

// workloadIdentityPrefix prefixes the AKS Workload Identity pod labels and annotations, e.g. the
// azure.workload.identity/use label the webhook selects the pods to inject tokens into by.
const workloadIdentityPrefix = "azure.workload.identity/"

// workloadIdentityMetadata returns the entries of a template Job's pod labels or annotations that
// a per-tenant clone must keep for its pods to authenticate through AKS Workload Identity, or nil
// if there are none. Nothing else is copied: the template is a live Job, so its pod labels include
// its own controller-uid / job-name, which would make the API server reject the clone, and any
// other metadata (app labels, sidecar annotations) has never been copied to clones.
func workloadIdentityMetadata(metadata map[string]string) map[string]string {
	var result map[string]string
	for key, value := range metadata {
		if strings.HasPrefix(key, workloadIdentityPrefix) {
			if result == nil {
				result = map[string]string{}
			}
			result[key] = value
		}
	}
	return result
}
