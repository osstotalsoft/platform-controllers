package configuration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	kubeFakeClientSet "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	configurationv1 "totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
)

// useFakeRecorder makes the controller and its handlers record their events to the returned recorder.
func useFakeRecorder(c *ConfigurationDomainController) *record.FakeRecorder {
	recorder := record.NewFakeRecorder(20)
	c.recorder = recorder
	c.configurationHandler.recorder = recorder
	c.kubeSecretsHandler.recorder = recorder
	if c.spcWriter != nil {
		c.spcWriter.recorder = recorder
	}
	return recorder
}

// recordedEvents drains the events recorded so far.
func recordedEvents(recorder *record.FakeRecorder) []string {
	var events []string
	for {
		select {
		case e := <-recorder.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}

func withVaultSecrets(t *testing.T, secrets ...secretSpec) {
	oldGetSecrets := getSecrets
	t.Cleanup(func() { getSecrets = oldGetSecrets })
	getSecrets = func(platform, namespace, domain, role string) ([]secretSpec, error) {
		return secrets, nil
	}
}

func TestConfigurationDomainController_secretProviderClasses(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)
	// another test shortens it; a requeue would fill the queue these tests check
	oldRequeueInterval := requeueInterval
	requeueInterval = 2 * time.Minute
	t.Cleanup(func() { requeueInterval = oldRequeueInterval })

	t.Run("kube secret changes re-enqueue the domain with VAULT_ENABLED=false", func(t *testing.T) {
		t.Setenv(EnvVaultEnabled, "false")
		platform, namespace, domain := "dev", "team1", "domain1"
		c := runController(
			[]runtime.Object{newPlatform(platform, platform)},
			[]runtime.Object{newConfigurationDomain(domain, namespace, platform, false, true)},
			nil,
			[]runtime.Object{newSecret("secret1", domain, namespace, platform, map[string][]byte{"k1": []byte("v1")})},
			nil)
		assert.True(t, c.processNextWorkItem())
		assert.Equal(t, 0, c.workqueue.Len())

		_, err := c.kubeClientset.CoreV1().Secrets(namespace).Create(context.TODO(),
			newSecret("secret2", domain, namespace, platform, map[string][]byte{"k2": []byte("v2")}), metav1.CreateOptions{})
		assert.NoError(t, err)

		assert.Eventually(t, func() bool { return c.workqueue.Len() == 1 }, 5*time.Second, 10*time.Millisecond,
			"a kube secret of the domain must re-enqueue it")
	})

	t.Run("provider change rewrites the SPC", func(t *testing.T) {
		platform, namespace, domain := "qa", "qa-t1", "domain1"
		configDomain := newConfigurationDomain(domain, namespace, platform, false, true)
		existing := &csiv1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getOutputSpcName(domain),
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(configDomain, configurationv1.SchemeGroupVersion.WithKind("ConfigurationDomain")),
				},
			},
			Spec: csiv1.SecretProviderClassSpec{Provider: "azure"},
		}
		// the same Vault parameters as the aggregation's, so only the provider differs
		existing.Spec.Parameters = (&secretsHandler{}).aggregateSecrets(configDomain, []secretSpec{{Key: "key1", Path: "path1"}},
			platform+"-readonly").Spec.Parameters
		withVaultSecrets(t, secretSpec{Key: "key1", Path: "path1"})
		c := runController([]runtime.Object{newPlatform(platform, platform)}, []runtime.Object{configDomain}, nil, nil,
			[]runtime.Object{existing})

		assert.True(t, c.processNextWorkItem())

		spc, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), getOutputSpcName(domain), metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.Equal(t, csiv1.Provider("vault"), spc.Spec.Provider)
		}
	})

	t.Run("no SPC informer: secrets aggregation still runs", func(t *testing.T) {
		platform, namespace, domain := "dev", "team1", "domain1"
		withVaultSecrets(t, secretSpec{Key: "key1", Path: "path1"})
		c := runControllerWithoutSpc(
			[]runtime.Object{newPlatform(platform, platform)},
			[]runtime.Object{newConfigurationDomain(domain, namespace, platform, false, true)},
			nil,
			[]runtime.Object{newSecret("secret1", domain, namespace, platform, map[string][]byte{"k1": []byte("v1")})})
		recorder := useFakeRecorder(c)

		assert.True(t, c.processNextWorkItem())

		output, err := c.kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), getOutputSecretName(domain), metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.Equal(t, map[string][]byte{"k1": []byte("v1")}, output.Data)
		}
		events := recordedEvents(recorder)
		assert.Condition(t, func() bool {
			for _, e := range events {
				if strings.Contains(e, "SecretProviderClass API") {
					return true
				}
			}
			return false
		}, "events: %v", events)
	})
}

func TestSpcAPIServed(t *testing.T) {
	served := kubeFakeClientSet.NewSimpleClientset().Discovery().(*fakediscovery.FakeDiscovery)
	served.Resources = []*metav1.APIResourceList{{
		GroupVersion: "secrets-store.csi.x-k8s.io/v1",
		APIResources: []metav1.APIResource{{Name: "secretproviderclasses", Kind: "SecretProviderClass"}},
	}}
	ok, err := SpcAPIServed(served)
	assert.NoError(t, err)
	assert.True(t, ok)

	notServed := kubeFakeClientSet.NewSimpleClientset().Discovery().(*fakediscovery.FakeDiscovery)
	ok, err = SpcAPIServed(notServed)
	assert.NoError(t, err, "a group version the server doesn't know is an answer, not an error")
	assert.False(t, ok)

	// a transient failure must not be taken for a missing CSI driver: that would turn SPCs off until a restart
	failing := kubeFakeClientSet.NewSimpleClientset()
	failing.PrependReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	_, err = SpcAPIServed(failing.Discovery())
	assert.ErrorContains(t, err, "connection refused")
}

func TestCleanupKeepsForeignSpc(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)
	platform, namespace, domain := "qa", "qa-t1", "domain1"
	configDomain := newConfigurationDomain(domain, namespace, platform, false, false) // aggregateSecrets: false
	foreign := &csiv1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: getOutputSpcName(domain), Namespace: namespace}}
	c := runController([]runtime.Object{newPlatform(platform, platform)}, []runtime.Object{configDomain}, nil, nil,
		[]runtime.Object{foreign})

	assert.True(t, c.processNextWorkItem())

	_, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), getOutputSpcName(domain), metav1.GetOptions{})
	assert.NoError(t, err, "an SPC the domain doesn't control must survive its cleanup")
}

func TestCleanupDeletesOwnedSpc(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)
	platform, namespace, domain := "qa", "qa-t1", "domain1"
	configDomain := newConfigurationDomain(domain, namespace, platform, false, false)
	owned := &csiv1.SecretProviderClass{ObjectMeta: newSpcMeta(configDomain)}
	c := runController([]runtime.Object{newPlatform(platform, platform)}, []runtime.Object{configDomain}, nil, nil,
		[]runtime.Object{owned})

	assert.True(t, c.processNextWorkItem())

	_, err := c.csiClientset.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.TODO(), getOutputSpcName(domain), metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err), "the domain's own SPC is deleted: %v", err)
}
