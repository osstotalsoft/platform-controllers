package provisioning

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

const (
	// WaitingForDomainReason is the reason of the event written on a Tenant when one of its domains waits for a domain
	// it depends on (Domain.spec.dependsOn) to be provisioned for the same tenant.
	WaitingForDomainReason = "WaitingForDomain"
	// DomainDependencyCycleReason is the reason of the event written on a Tenant when one of its domains is on a
	// dependency cycle among the tenant's domains.
	DomainDependencyCycleReason = "DomainDependencyCycle"
)

// domainDependencies is the in-memory state of the provisioning order of tenant domains. Both maps are keyed by work
// queue keys (platform::tenantNamespace/tenantName::domain).
type domainDependencies struct {
	mu sync.Mutex
	// ready holds the tenant domains known to be provisioned, so the provisioner is asked about each at most once.
	ready map[string]struct{}
	// waiting maps a tenant domain to the domains of the same tenant that wait for it.
	waiting map[string]map[string]struct{}
}

func newDomainDependencies() *domainDependencies {
	return &domainDependencies{
		ready:   map[string]struct{}{},
		waiting: map[string]map[string]struct{}{},
	}
}

func (d *domainDependencies) isReady(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.ready[key]
	return ok
}

func (d *domainDependencies) markReady(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ready[key] = struct{}{}
}

func (d *domainDependencies) forget(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.ready, key)
}

func (d *domainDependencies) addWaiting(key, domain string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.waiting[key] == nil {
		d.waiting[key] = map[string]struct{}{}
	}
	d.waiting[key][domain] = struct{}{}
}

func (d *domainDependencies) removeWaiting(key, domain string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.waiting[key], domain)
	if len(d.waiting[key]) == 0 {
		delete(d.waiting, key)
	}
}

// succeeded records a successful sync of the tenant domain identified by key: it's ready if it has resources, and
// removed otherwise. It returns, and clears, the domains that were waiting for it. Both happen under the same lock
// addWaiting takes, so a domain added to the waiting set before its readiness check can't miss this wake-up.
func (d *domainDependencies) succeeded(key string, hasResources bool) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if hasResources {
		d.ready[key] = struct{}{}
	} else {
		delete(d.ready, key)
	}
	waiting := make([]string, 0, len(d.waiting[key]))
	for domain := range d.waiting[key] {
		waiting = append(waiting, domain)
	}
	delete(d.waiting, key)
	return waiting
}

// canProvisionDomainForTenant tells whether domain can be provisioned for tenant now, as far as Domain.spec.dependsOn
// is concerned. When it can't, it records why: a WaitingForDomain event, when a domain it depends on isn't provisioned
// yet for the tenant (the domain is woken when that one is), or a DomainDependencyCycle event, when the domain is on a
// dependency cycle among the tenant's domains. An error means readiness couldn't be determined; the sync should be
// retried.
func (c *ProvisioningController) canProvisionDomainForTenant(tenant *platformv1.Tenant, domain string) (bool, error) {
	deps := c.domainDependenciesForTenant(tenant, domain)
	if len(deps) == 0 {
		return true, nil
	}

	tenantKey, err := cache.MetaNamespaceKeyFunc(tenant)
	if err != nil {
		return false, err
	}

	if cycle := findDependencyCycle(domain, func(d string) []string { return c.domainDependenciesForTenant(tenant, d) }); cycle != nil {
		cycleErr := fmt.Errorf("domain dependency cycle: %s", strings.Join(cycle, " -> "))
		klog.InfoS("Domain is on a dependency cycle, not provisioning it", "tenant", tenantKey, "domain", domain, "cycle", cycle)
		c.recorder.Event(tenant, corev1.EventTypeWarning, DomainDependencyCycleReason, cycleErr.Error())
		c.publishFailureEvents(tenant, domain, cycleErr)
		return false, nil
	}

	for _, dep := range deps {
		depKey := encodeKey(tenant.Spec.PlatformRef, tenantKey, dep)
		// Register before checking, so a dependency that succeeds meanwhile finds this domain in its waiting set.
		c.dependencies.addWaiting(depKey, domain)

		ready, err := c.isTenantDomainReady(tenant, dep)
		if err != nil {
			return false, err
		}
		if !ready {
			msg := fmt.Sprintf("domain %s waits for domain %s to be provisioned", domain, dep)
			klog.InfoS("Domain waits for a dependency", "tenant", tenantKey, "domain", domain, "dependency", dep)
			c.recorder.Event(tenant, corev1.EventTypeNormal, WaitingForDomainReason, msg)
			return false, nil
		}
		c.dependencies.removeWaiting(depKey, domain)
	}

	return true, nil
}

// isTenantDomainReady tells whether domain counts as provisioned for tenant: it has no resources for the tenant, or
// it has been provisioned.
func (c *ProvisioningController) isTenantDomainReady(tenant *platformv1.Tenant, domain string) (bool, error) {
	if c.dependencies.isReady(c.tenantDomainKey(tenant, domain)) {
		return true, nil
	}

	resources, err := c.provisioningResourcesFor(tenant, domain)
	if err != nil {
		return false, err
	}
	if resources.IsEmpty() {
		return true, nil
	}

	return c.isTenantDomainProvisioned(tenant, domain)
}

// isTenantDomainProvisioned tells whether domain has been provisioned for tenant, from the ready cache or, failing
// that, from the provisioner.
func (c *ProvisioningController) isTenantDomainProvisioned(tenant *platformv1.Tenant, domain string) (bool, error) {
	key := c.tenantDomainKey(tenant, domain)
	if c.dependencies.isReady(key) {
		return true, nil
	}

	ready, err := c.provisioner.IsProvisioned(tenant, domain)
	if err != nil {
		return false, fmt.Errorf("cannot tell whether domain %s is provisioned for tenant %s: %w", domain, tenant.GetName(), err)
	}
	klog.V(4).InfoS("Asked the provisioner whether the domain is provisioned", "tenant", tenant.GetName(), "domain", domain, "provisioned", ready)
	if ready {
		c.dependencies.markReady(key)
	}
	return ready, nil
}

// wakeDependentDomains is called after domain was synced successfully for tenant. It enqueues the tenant's domains
// that were waiting for it, and those that depend on it and have never been provisioned (they may have been waiting
// before a restart).
func (c *ProvisioningController) wakeDependentDomains(tenant *platformv1.Tenant, domain string, hasResources bool) {
	toEnqueue := map[string]struct{}{}
	for _, waiting := range c.dependencies.succeeded(c.tenantDomainKey(tenant, domain), hasResources) {
		toEnqueue[waiting] = struct{}{}
	}

	for _, dependent := range tenant.Spec.DomainRefs {
		if _, ok := toEnqueue[dependent]; ok || dependent == domain {
			continue
		}
		if !slices.Contains(c.domainDependsOn(tenant.Spec.PlatformRef, dependent), domain) {
			continue
		}
		ready, err := c.isTenantDomainProvisioned(tenant, dependent)
		if err != nil {
			klog.ErrorS(err, "Enqueuing a dependent domain whose readiness is unknown", "tenant", tenant.GetName(), "domain", dependent)
		} else if ready {
			continue
		}
		toEnqueue[dependent] = struct{}{}
	}

	for dependent := range toEnqueue {
		klog.V(4).InfoS("Waking up a dependent domain", "tenant", tenant.GetName(), "domain", dependent, "dependency", domain)
		c.enqueueTenantDomain(tenant, dependent)
	}
}

// domainDependenciesForTenant returns the domains of the tenant that domain depends on directly. Dependencies the
// tenant doesn't have are ignored.
func (c *ProvisioningController) domainDependenciesForTenant(tenant *platformv1.Tenant, domain string) []string {
	deps := []string{}
	for _, dep := range c.domainDependsOn(tenant.Spec.PlatformRef, domain) {
		if slices.Contains(tenant.Spec.DomainRefs, dep) && !slices.Contains(deps, dep) {
			deps = append(deps, dep)
		}
	}
	return deps
}

// domainDependsOn returns the names in the dependsOn of the Domain named domain on platform, or nothing when there's
// no such Domain.
func (c *ProvisioningController) domainDependsOn(platform, domain string) []string {
	domains, err := c.domainInformer.Lister().List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list domains")
		return nil
	}

	for _, d := range domains {
		if d.Name == domain && d.Spec.PlatformRef == platform {
			names := make([]string, 0, len(d.Spec.DependsOn))
			for _, dep := range d.Spec.DependsOn {
				names = append(names, dep.Domain)
			}
			return names
		}
	}
	return nil
}

func (c *ProvisioningController) tenantDomainKey(tenant *platformv1.Tenant, domain string) string {
	tenantKey, _ := cache.MetaNamespaceKeyFunc(tenant)
	return encodeKey(tenant.Spec.PlatformRef, tenantKey, domain)
}

// findDependencyCycle returns a dependency cycle through start, as the path start -> ... -> start, or nil when start
// isn't on a cycle. dependencies returns the direct dependencies of a domain.
func findDependencyCycle(start string, dependencies func(domain string) []string) []string {
	visited := map[string]bool{start: true}
	path := []string{}

	var visit func(domain string) []string
	visit = func(domain string) []string {
		path = append(path, domain)
		for _, dep := range dependencies(domain) {
			if dep == start {
				return append(append([]string{}, path...), start)
			}
			if !visited[dep] {
				visited[dep] = true
				if cycle := visit(dep); cycle != nil {
					return cycle
				}
			}
		}
		path = path[:len(path)-1]
		return nil
	}

	return visit(start)
}
