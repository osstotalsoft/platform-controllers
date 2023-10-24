package provisioning

import (
	"encoding/json"
	"reflect"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

type CreateInfrastructureFunc[T ProvisioningTarget] func(
	platform string,
	target T,
	domain string,
	infra *InfrastructureManifests) ProvisioningResult

type InfrastructureManifests struct {
	AzureDbs             []*provisioningv1.AzureDatabase
	AzureManagedDbs      []*provisioningv1.AzureManagedDatabase
	HelmReleases         []*provisioningv1.HelmRelease
	AzureVirtualMachines []*provisioningv1.AzureVirtualMachine
	AzureVirtualDesktops []*provisioningv1.AzureVirtualDesktop
}

type ProvisioningResult struct {
	Error      error
	HasChanges bool
}

type ProvisioningResource interface {
	*provisioningv1.AzureDatabase | *provisioningv1.AzureManagedDatabase | *provisioningv1.HelmRelease | *provisioningv1.AzureVirtualMachine | *provisioningv1.AzureVirtualDesktop

	GetProvisioningMeta() *provisioningv1.ProvisioningMeta
	GetSpec() any
	GetName() string
	GetNamespace() string
}

type ProvisioningTarget interface {
	*Tenant | *Platform

	GetCategory() provisioningv1.ProvisioningTargetCategory
	GetDeletePolicy() platformv1.DeletePolicy
	GetName() string
	GetDescription() string
	GetNamespace() string
	GetPlatformName() string
	GetRuntimeObject() runtime.Object
	GetPathSegment() string
	GetSuccessEvent(domain string) (string, any)
	GetFailureEvent(domain string, err error) (string, any)
	GetTemplateContext() any
	GetOverrides(provisioningv1.ProvisioningMeta) map[string]*apiextensionsv1.JSON
	IsEnabled() bool
}

type Tenant platformv1.Tenant
type Platform platformv1.Platform

func (tenant *Tenant) GetDeletePolicy() platformv1.DeletePolicy {
	return tenant.Spec.DeletePolicy
}

func (tenant *Tenant) GetDescription() string {
	return tenant.Spec.Description
}

func (tenant *Tenant) GetCategory() provisioningv1.ProvisioningTargetCategory {
	return provisioningv1.ProvisioningTargetCategoryTenant
}

func (tenant *Tenant) GetPlatformName() string {
	return tenant.Spec.PlatformRef
}

func (tenant *Tenant) GetRuntimeObject() runtime.Object {
	return (*platformv1.Tenant)(tenant)
}

func (tenant *Tenant) GetPathSegment() string {
	return tenant.GetName()
}

func (tenant *Tenant) GetSuccessEvent(domain string) (string, any) {
	ev := struct {
		TenantId          string
		TenantName        string
		TenantDescription string
		Platform          string
		Domain            string
	}{
		TenantId:          tenant.Spec.Id,
		TenantName:        tenant.Name,
		TenantDescription: tenant.Spec.Description,
		Platform:          tenant.GetPlatformName(),
		Domain:            domain,
	}

	topic := tenantProvisionedSuccessfullyTopic

	return topic, ev
}

func (tenant *Tenant) GetFailureEvent(domain string, err error) (string, any) {
	ev := struct {
		TenantId          string
		TenantName        string
		TenantDescription string
		Platform          string
		Domain            string
		Error             string
	}{
		TenantId:          tenant.Spec.Id,
		TenantName:        tenant.Name,
		TenantDescription: tenant.Spec.Description,
		Platform:          tenant.GetPlatformName(),
		Domain:            domain,
		Error:             err.Error(),
	}

	topic := tenantProvisionningFailedTopic

	return topic, ev
}

func (tenant *Tenant) GetTemplateContext() any {
	return struct {
		Platform string
		Tenant   struct {
			Id          string
			Code        string
			Description string
		}
	}{
		Platform: tenant.GetPlatformName(),
		Tenant: struct {
			Id          string
			Code        string
			Description string
		}{
			Id:          tenant.Spec.Id,
			Code:        tenant.GetName(),
			Description: tenant.GetDescription(),
		},
	}
}

func (tenant *Tenant) GetOverrides(meta provisioningv1.ProvisioningMeta) map[string]*apiextensionsv1.JSON {
	return meta.TenantOverrides
}

func (tenant *Tenant) IsEnabled() bool {
	return tenant.Spec.Enabled
}

func (platform *Platform) GetDeletePolicy() platformv1.DeletePolicy {
	return platformv1.DeletePolicyRetainStatefulResources
}

func (platform *Platform) GetDescription() string {
	return platform.GetName()
}

func (platform *Platform) GetCategory() provisioningv1.ProvisioningTargetCategory {
	return provisioningv1.ProvisioningTargetCategoryPlatform
}

func (platform *Platform) GetPlatformName() string {
	return platform.GetName()
}

func (platform *Platform) GetRuntimeObject() runtime.Object {
	return (*platformv1.Platform)(platform)
}

func (platform *Platform) GetPathSegment() string {
	return "platform"
}

func (platform *Platform) GetSuccessEvent(domain string) (string, any) {
	ev := struct {
		Platform string
		Domain   string
	}{
		Platform: platform.GetName(),
		Domain:   domain,
	}

	topic := platformProvisionedSuccessfullyTopic

	return topic, ev
}

func (platform *Platform) GetFailureEvent(domain string, err error) (string, any) {
	ev := struct {
		Platform string
		Domain   string
		Error    string
	}{
		Platform: platform.GetName(),
		Domain:   domain,
		Error:    err.Error(),
	}

	topic := platformProvisionningFailedTopic

	return topic, ev
}

func (platform *Platform) GetTemplateContext() any {
	return struct {
		Platform string
	}{
		Platform: platform.GetName(),
	}
}

func (platform *Platform) GetOverrides(meta provisioningv1.ProvisioningMeta) map[string]*apiextensionsv1.JSON {
	return nil
}

func (platform *Platform) IsEnabled() bool {
	return true
}

type Cloner[C any] interface {
	DeepCopy() C
}

func getPlatformAndDomain[R ProvisioningResource](res R) (platform, domain string, target provisioningv1.ProvisioningTargetCategory, ok bool) {
	platform = res.GetProvisioningMeta().PlatformRef
	domain = res.GetProvisioningMeta().DomainRef
	target = res.GetProvisioningMeta().Target.Category

	if len(platform) == 0 || len(domain) == 0 {
		return platform, domain, target, false
	}

	return platform, domain, target, true
}

func exludeTenant(filter provisioningv1.ProvisioningTargetFilter, tenant string) bool {
	tenantInList := slices.Contains(filter.Values, tenant)
	if filter.Kind == provisioningv1.ProvisioningFilterKindBlacklist && tenantInList {
		return true
	}

	if filter.Kind == provisioningv1.ProvisioningFilterKindWhitelist && !tenantInList {
		return true
	}

	return false
}

func selectItemsInTarget[R ProvisioningResource, T ProvisioningTarget](platform string, domain string, source []R, target T) []R {
	result := []R{}
	for _, res := range source {
		provisioningMeta := res.GetProvisioningMeta()

		if provisioningMeta.Target.Category == provisioningv1.ProvisioningTargetCategoryTenant {
			if exludeTenant(provisioningMeta.Target.Filter, target.GetName()) {
				continue
			}

		}

		if target.GetCategory() != provisioningMeta.Target.Category {
			continue
		}

		if provisioningMeta.PlatformRef == platform && provisioningMeta.DomainRef == domain {
			result = append(result, res)
		}
	}
	return result
}

func applyTargetOverrides[R interface {
	ProvisioningResource
	Cloner[R]
}, T ProvisioningTarget](source []R, target T) ([]R, error) {
	if source == nil {
		return source, nil
	}

	result := []R{}

	for _, res := range source {
		overrides := target.GetOverrides(*res.GetProvisioningMeta())

		if overrides == nil {
			result = append(result, res)
			continue
		}

		tenantOverridesJson, exists := overrides[target.GetName()]
		if !exists {
			result = append(result, res)
			continue
		}

		var tenantOverridesMap map[string]any
		if err := json.Unmarshal(tenantOverridesJson.Raw, &tenantOverridesMap); err != nil {
			return nil, err
		}

		resSpecJsonBytes, err := json.Marshal(res.GetSpec())
		if err != nil {
			return nil, err
		}

		var targetSpecMap map[string]any
		if err := json.Unmarshal(resSpecJsonBytes, &targetSpecMap); err != nil {
			return nil, err
		}

		if err := mergo.Merge(&targetSpecMap, tenantOverridesMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
			return nil, err
		}

		resSpecJsonBytes, err = json.Marshal(targetSpecMap)
		if err != nil {
			return nil, err
		}

		resClone := res.DeepCopy()

		if err := json.Unmarshal(resSpecJsonBytes, resClone.GetSpec()); err != nil {
			return nil, err
		}

		result = append(result, resClone)
	}

	return result, nil
}

type jsonTransformer struct {
}

func (t jsonTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(apiextensionsv1.JSON{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				srcRaw := src.FieldByName("Raw").Bytes()
				var srcMap map[string]any
				if err := json.Unmarshal(srcRaw, &srcMap); err != nil {
					return err
				}

				dstRaw := dst.FieldByName("Raw").Bytes()
				var dstMap map[string]any
				if err := json.Unmarshal(dstRaw, &dstMap); err != nil {
					return err
				}

				if err := mergo.Merge(&dstMap, srcMap, mergo.WithOverride, mergo.WithTransformers(jsonTransformer{})); err != nil {
					return err
				}

				dstRaw, err := json.Marshal(dstMap)
				if err != nil {
					return err
				}

				dst.FieldByName("Raw").SetBytes(dstRaw)
			}
			return nil
		}
	}
	return nil
}
