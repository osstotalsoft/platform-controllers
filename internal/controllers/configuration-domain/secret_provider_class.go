package configuration

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	csiClientset "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	csiListers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/listers/apis/v1"

	controllers "totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
)

// spcWriter writes a configuration domain's aggregated SecretProviderClass, <domain>-aggregate, whichever secret store
// it comes from.
type spcWriter struct {
	csiClientset csiClientset.Interface
	spcLister    csiListers.SecretProviderClassLister

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func newSpcWriter(csiClientset csiClientset.Interface, spcLister csiListers.SecretProviderClassLister, recorder record.EventRecorder) *spcWriter {
	return &spcWriter{csiClientset: csiClientset, spcLister: spcLister, recorder: recorder}
}

// newSpcMeta returns the metadata of configDomain's aggregated SecretProviderClass: its name, its labels, and its
// owner, the configuration domain.
func newSpcMeta(configDomain *v1alpha1.ConfigurationDomain) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: getOutputSpcName(configDomain.Name),
		Labels: map[string]string{
			controllers.DomainLabelName:   configDomain.Name,
			controllers.PlatformLabelName: configDomain.Spec.PlatformRef,
		},
		Namespace: configDomain.Namespace,
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(configDomain, v1alpha1.SchemeGroupVersion.WithKind("ConfigurationDomain")),
		},
	}
}

// Apply creates desired, or updates the existing SecretProviderClass when its provider or parameters differ, so that a
// platform switching secret store gets its SecretProviderClass rewritten in place. A SecretProviderClass of the same
// name that configDomain doesn't control is left alone, with a warning.
func (w *spcWriter) Apply(configDomain *v1alpha1.ConfigurationDomain, desired *csiv1.SecretProviderClass) error {
	outputSpc, err := w.spcLister.SecretProviderClasses(desired.Namespace).Get(desired.Name)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		outputSpc, err = w.csiClientset.SecretsstoreV1().SecretProviderClasses(desired.Namespace).Create(context.TODO(), desired, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		w.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}

	// If the SPC is not controlled by this ConfigurationDomain resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputSpc, configDomain) {
		msg := fmt.Sprintf(MessageResourceExists, outputSpc.Name)
		w.recorder.Event(configDomain, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil
	}

	if desired.Spec.Provider == outputSpc.Spec.Provider && reflect.DeepEqual(desired.Spec.Parameters, outputSpc.Spec.Parameters) {
		return nil
	}

	klog.V(4).Infof("Secret values changed")
	outputSpc = outputSpc.DeepCopy()
	outputSpc.Spec.Provider = desired.Spec.Provider
	outputSpc.Spec.Parameters = desired.Spec.Parameters
	_, err = w.csiClientset.SecretsstoreV1().SecretProviderClasses(desired.Namespace).Update(context.TODO(), outputSpc, metav1.UpdateOptions{})
	if err != nil {
		w.recorder.Event(configDomain, corev1.EventTypeWarning, controllers.ErrorSynced, err.Error())
		return err
	}
	return nil
}

// Cleanup deletes the domain's aggregated SecretProviderClass, if any.
func (w *spcWriter) Cleanup(namespace, domain string) error {
	err := w.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Delete(context.TODO(), getOutputSpcName(domain), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}
