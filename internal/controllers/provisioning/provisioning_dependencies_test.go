package provisioning

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
	fakeClientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/fake"
)

// depsHarness runs a ProvisioningController with fakes that record what gets provisioned, which stack readiness
// lookups happen, what's published and which events are written.
type depsHarness struct {
	t         *testing.T
	c         *ProvisioningController
	clientset *fakeClientset.Clientset
	recorder  *record.FakeRecorder

	mu             sync.Mutex
	provisioned    []string        // "<target>/<domain>", in order
	stackReady     map[string]bool // domain -> what the stack history says
	readinessErr   error
	readinessCalls map[string]int // domain -> number of stack history lookups
	topics         []string
}

func newDepsHarness(t *testing.T, objects ...runtime.Object) *depsHarness {
	t.Helper()
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	h := &depsHarness{
		t:              t,
		clientset:      fakeClientset.NewSimpleClientset(objects...),
		recorder:       record.NewFakeRecorder(100),
		stackReady:     map[string]bool{},
		readinessCalls: map[string]int{},
	}

	provisioner := func(target ProvisioningTarget, domain string, infra *InfrastructureManifests) ProvisioningResult {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.provisioned = append(h.provisioned, target.GetName()+"/"+domain)
		return ProvisioningResult{}
	}
	readiness := func(target ProvisioningTarget, domain string) (bool, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.readinessCalls[domain]++
		return h.stackReady[domain], h.readinessErr
	}
	publisher := func(ctx context.Context, topic string, data interface{}, platform string) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.topics = append(h.topics, topic)
		return nil
	}

	h.c = NewProvisioningController(h.clientset, provisioner, readiness, nil, nil, publisher)
	h.c.recorder = h.recorder
	h.c.factory.Start(nil)
	h.c.factory.WaitForCacheSync(nil)
	require.True(t, cache.WaitForCacheSync(nil, h.c.handlerSyncFuncs()...))
	// Start from an empty queue, so the tests see only what the syncs under test enqueue.
	h.c.drainWorkqueue()
	return h
}

func (h *depsHarness) sync(key string) {
	h.t.Helper()
	require.NoError(h.t, h.c.syncHandler(key))
}

// queued returns the keys in the queue, emptying it.
func (h *depsHarness) queued() []string {
	keys := []string{}
	for h.c.workqueue.Len() > 0 {
		key, _ := h.c.workqueue.Get()
		keys = append(keys, key.(string))
		h.c.workqueue.Forget(key)
		h.c.workqueue.Done(key)
	}
	return keys
}

// processQueue syncs every queued key, including the ones enqueued while doing so.
func (h *depsHarness) processQueue() {
	h.t.Helper()
	for h.c.workqueue.Len() > 0 {
		h.c.processNextWorkItem(0)
	}
}

func (h *depsHarness) events() []string {
	events := []string{}
	for {
		select {
		case e := <-h.recorder.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}

func (h *depsHarness) provisionedTargets() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string{}, h.provisioned...)
}

func (h *depsHarness) publishedTopics() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string{}, h.topics...)
}

func eventsWithReason(events []string, reason string) []string {
	matching := []string{}
	for _, e := range events {
		if strings.Contains(e, " "+reason+" ") {
			matching = append(matching, e)
		}
	}
	return matching
}

func newDomain(name, platform string, dependsOn ...string) *platformv1.Domain {
	deps := []platformv1.DomainDependency{}
	for _, d := range dependsOn {
		deps = append(deps, platformv1.DomainDependency{Domain: d})
	}
	return &platformv1.Domain{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
		Spec:       platformv1.DomainSpec{PlatformRef: platform, DependsOn: deps},
	}
}

func newLocalScript(name, platform, domain string, category provisioningv1.ProvisioningTargetCategory) *provisioningv1.LocalScript {
	return &provisioningv1.LocalScript{
		TypeMeta:   metav1.TypeMeta{APIVersion: provisioningv1.SchemeGroupVersion.String(), Kind: "LocalScript"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
		Spec: provisioningv1.LocalScriptSpec{
			CreateScriptContent: "echo " + name,
			Shell:               "pwsh",
			ProvisioningMeta: provisioningv1.ProvisioningMeta{
				PlatformRef: platform,
				DomainRef:   domain,
				Target:      provisioningv1.ProvisioningTarget{Category: category},
			},
		},
	}
}

func newPlatform(name string) *platformv1.Platform {
	return &platformv1.Platform{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// tenantKey is the work queue key of domain for tenant t1 on platform dev.
func t1Key(domain string) string {
	return encodeKey("dev", "default/t1", domain)
}

// baseObjects: tenant t1 has domains a and b, b depends on a, and both have a tenant script.
func baseObjects() []runtime.Object {
	return []runtime.Object{
		newPlatform("dev"),
		newTenant("t1", "dev", "a", "b"),
		newDomain("a", "dev"),
		newDomain("b", "dev", "a"),
		newLocalScript("script-a", "dev", "a", provisioningv1.ProvisioningTargetCategoryTenant),
		newLocalScript("script-b", "dev", "b", provisioningv1.ProvisioningTargetCategoryTenant),
	}
}

func TestDomainDependencies(t *testing.T) {
	t.Run("U1 a domain waits for a dependency that isn't provisioned", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)

		h.c.workqueue.Add(t1Key("b"))
		h.c.processNextWorkItem(0)

		assert.Empty(t, h.provisionedTargets(), "b must not be provisioned before a")
		waiting := eventsWithReason(h.events(), WaitingForDomainReason)
		if assert.Len(t, waiting, 1) {
			assert.Contains(t, waiting[0], "domain b waits for domain a to be provisioned")
			assert.True(t, strings.HasPrefix(waiting[0], "Normal "))
		}
		assert.Empty(t, h.publishedTopics(), "waiting is neither a success nor a failure")
		assert.Zero(t, h.c.workqueue.Len())
		assert.Zero(t, h.c.workqueue.NumRequeues(t1Key("b")), "waiting must not be retried with backoff")
	})

	t.Run("U2 the dependency's success wakes the waiting domain", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)
		h.sync(t1Key("b"))
		require.Empty(t, h.provisionedTargets())

		h.sync(t1Key("a"))
		assert.Equal(t, []string{t1Key("b")}, h.queued())

		h.sync(t1Key("b"))
		assert.Equal(t, []string{"t1/a", "t1/b"}, h.provisionedTargets())
		assert.Equal(t, []string{tenantProvisionedSuccessfullyTopic, tenantProvisionedSuccessfullyTopic}, h.publishedTopics())
	})

	t.Run("U3 the dependency's success wakes a dependent that never succeeded, even if it wasn't seen waiting", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)

		h.sync(t1Key("a"))
		assert.Equal(t, []string{t1Key("b")}, h.queued(), "b's stack has no successful update, so b is woken")

		h2 := newDepsHarness(t, baseObjects()...)
		h2.stackReady["b"] = true
		h2.sync(t1Key("a"))
		assert.Empty(t, h2.queued(), "b's stack was already provisioned, so it's left alone")
	})

	t.Run("U4 a dependency the tenant doesn't have is ignored", func(t *testing.T) {
		objects := baseObjects()
		objects[1] = newTenant("t1", "dev", "b")
		h := newDepsHarness(t, objects...)

		h.sync(t1Key("b"))

		assert.Equal(t, []string{"t1/b"}, h.provisionedTargets())
		assert.Empty(t, eventsWithReason(h.events(), WaitingForDomainReason))
		assert.Zero(t, h.readinessCalls["a"])
	})

	t.Run("U5 a dependency without resources for the tenant counts as provisioned", func(t *testing.T) {
		objects := baseObjects()
		objects = append(objects[:4], objects[5]) // drop script-a
		h := newDepsHarness(t, objects...)

		h.sync(t1Key("b"))

		assert.Equal(t, []string{"t1/b"}, h.provisionedTargets())
		assert.Zero(t, h.readinessCalls["a"], "no resources means nothing to wait for, without reading any history")
	})

	t.Run("U6 platform targets ignore dependsOn", func(t *testing.T) {
		objects := append(baseObjects(),
			newLocalScript("platform-b", "dev", "b", provisioningv1.ProvisioningTargetCategoryPlatform))
		h := newDepsHarness(t, objects...)

		h.sync(encodeKey("dev", "", "b"))

		assert.Equal(t, []string{"dev/b"}, h.provisionedTargets())
		assert.Empty(t, h.readinessCalls)
		assert.Empty(t, h.queued(), "a platform success wakes nothing")
	})

	t.Run("U7 a dependency cycle fails without retrying, and fixing it provisions both domains", func(t *testing.T) {
		objects := baseObjects()
		objects[2] = newDomain("a", "dev", "b")
		h := newDepsHarness(t, objects...)

		h.c.workqueue.Add(t1Key("b"))
		h.c.processNextWorkItem(0)

		assert.Empty(t, h.provisionedTargets())
		cycle := eventsWithReason(h.events(), DomainDependencyCycleReason)
		if assert.Len(t, cycle, 1) {
			assert.Contains(t, cycle[0], "b -> a -> b")
			assert.True(t, strings.HasPrefix(cycle[0], "Warning "))
		}
		assert.Equal(t, []string{tenantProvisioningFailedTopic}, h.publishedTopics())
		assert.Zero(t, h.c.workqueue.NumRequeues(t1Key("b")))
		assert.Zero(t, h.c.workqueue.Len())

		// Fix the cycle: a no longer depends on b.
		_, err := h.clientset.PlatformV1alpha1().Domains(metav1.NamespaceDefault).
			Update(context.TODO(), newDomain("a", "dev"), metav1.UpdateOptions{})
		require.NoError(t, err)
		assert.Eventually(t, func() bool { return h.c.workqueue.Len() == 1 }, 10*time.Second, 20*time.Millisecond,
			"the Domain change should enqueue a")

		h.processQueue()
		assert.Equal(t, []string{"t1/a", "t1/b"}, h.provisionedTargets())
	})

	t.Run("U8 Domain changes enqueue the domain only for the tenants that have it", func(t *testing.T) {
		objects := append(baseObjects(), newTenant("t2", "dev", "a", "c"))
		h := newDepsHarness(t, objects...)
		domains := h.clientset.PlatformV1alpha1().Domains(metav1.NamespaceDefault)

		_, err := domains.Create(context.TODO(), newDomain("d", "dev"), metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = domains.Create(context.TODO(), newDomain("c", "dev", "a"), metav1.CreateOptions{})
		require.NoError(t, err)
		assert.Eventually(t, func() bool { return h.c.workqueue.Len() == 1 }, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []string{encodeKey("dev", "default/t2", "c")}, h.queued(),
			"only the Domain with dependencies is enqueued, and only for t2")

		_, err = domains.Update(context.TODO(), newDomain("b", "dev"), metav1.UpdateOptions{})
		require.NoError(t, err)
		assert.Eventually(t, func() bool { return h.c.workqueue.Len() == 1 }, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []string{t1Key("b")}, h.queued())
	})

	t.Run("U9 stack readiness is read from the history once, and not at all after a success in this process", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)
		h.stackReady["a"] = true

		h.sync(t1Key("b"))
		h.sync(t1Key("b"))
		assert.Equal(t, []string{"t1/b", "t1/b"}, h.provisionedTargets())
		assert.Equal(t, 1, h.readinessCalls["a"])

		h2 := newDepsHarness(t, baseObjects()...)
		h2.sync(t1Key("a"))
		h2.sync(t1Key("b"))
		assert.Equal(t, []string{"t1/a", "t1/b"}, h2.provisionedTargets())
		assert.Zero(t, h2.readinessCalls["a"])
	})

	t.Run("U9 a destroyed stack no longer counts as provisioned", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)
		h.sync(t1Key("a"))
		h.queued()
		require.True(t, h.c.dependencies.isReady(t1Key("a")))

		// a is removed from the tenant, so its stack is destroyed on the cleanup path
		_, err := h.clientset.PlatformV1alpha1().Tenants(metav1.NamespaceDefault).
			Update(context.TODO(), newTenant("t1", "dev", "b"), metav1.UpdateOptions{})
		require.NoError(t, err)
		h.sync(t1Key("a"))

		assert.False(t, h.c.dependencies.isReady(t1Key("a")))
	})

	t.Run("U10 a readiness lookup error is retried with backoff", func(t *testing.T) {
		h := newDepsHarness(t, baseObjects()...)
		h.readinessErr = errors.New("backend unavailable")

		h.c.workqueue.Add(t1Key("b"))
		h.c.processNextWorkItem(0)

		assert.Empty(t, h.provisionedTargets())
		assert.Equal(t, 1, h.c.workqueue.NumRequeues(t1Key("b")))
	})
}

func TestFindDependencyCycle(t *testing.T) {
	graph := func(edges map[string][]string) func(string) []string {
		return func(d string) []string { return edges[d] }
	}

	tests := []struct {
		name  string
		start string
		edges map[string][]string
		want  []string
	}{
		{"no dependencies", "a", map[string][]string{}, nil},
		{"chain", "c", map[string][]string{"c": {"b"}, "b": {"a"}}, nil},
		{"self dependency", "a", map[string][]string{"a": {"a"}}, []string{"a", "a"}},
		{"two domains", "a", map[string][]string{"a": {"b"}, "b": {"a"}}, []string{"a", "b", "a"}},
		{"three domains", "a", map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, []string{"a", "b", "c", "a"}},
		{"cycle not through start", "x", map[string][]string{"x": {"a"}, "a": {"b"}, "b": {"a"}}, nil},
		{"diamond", "d", map[string][]string{"d": {"b", "c"}, "b": {"a"}, "c": {"a"}}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, findDependencyCycle(tt.start, graph(tt.edges)))
		})
	}
}
