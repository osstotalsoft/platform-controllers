package pulumi

import (
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// resourceCaptureMocks records every NewResource call the test triggers, keyed both by Pulumi
// TypeToken (e.g. "pulumi:providers:mssql", "mssql:index/sqlLogin:SqlLogin",
// "kubernetes:core/v1:ConfigMap") and by resource Name, so a test can assert:
//   - which Pulumi resource types were (or, importantly, were NOT) registered at all — e.g. no
//     mssql-namespaced resource when neither User nor ManagedIdentity is configured;
//   - the resolved Inputs of one specific resource — e.g. an mssql.Provider's "hostname", or a
//     ConfigMap's rendered "data".
//
// It also lets a test stub a fixed response for a specific Invoke (Call) token — the default
// WithMocks Call behavior just echoes the invoke's own request Args back as the result, which
// doesn't have fields (like getManagedInstance's "fullyQualifiedDomainName") that only exist on the
// real response shape.
type resourceCaptureMocks struct {
	mu          sync.Mutex
	byType      map[string][]pulumi.MockResourceArgs
	byName      map[string]pulumi.MockResourceArgs
	callResults map[string]resource.PropertyMap
}

func newResourceCaptureMocks() *resourceCaptureMocks {
	return &resourceCaptureMocks{
		byType:      map[string][]pulumi.MockResourceArgs{},
		byName:      map[string]pulumi.MockResourceArgs{},
		callResults: map[string]resource.PropertyMap{},
	}
}

func (m *resourceCaptureMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	m.byType[args.TypeToken] = append(m.byType[args.TypeToken], args)
	m.byName[args.Name] = args
	m.mu.Unlock()

	// The default WithMocks convention is to echo Inputs back as the resource's outputs. That's not
	// enough for provider-computed, output-only properties (e.g. RandomPassword.Result,
	// UserAssignedIdentity.ClientId/PrincipalId) which have no corresponding input — real tests that
	// assert on exported values downstream of those properties need a synthesized non-empty value,
	// or the Output would silently resolve to its zero value ("") and any "non-empty" assertion on
	// an export built from it would be vacuous.
	outputs := resource.PropertyMap{}
	for k, v := range args.Inputs {
		outputs[k] = v
	}
	switch args.TypeToken {
	case "random:index/randomPassword:RandomPassword":
		outputs["result"] = resource.NewStringProperty("Test-Password-123!")
	case "azure-native:managedidentity:UserAssignedIdentity":
		outputs["clientId"] = resource.NewStringProperty("11111111-1111-1111-1111-111111111111")
		outputs["principalId"] = resource.NewStringProperty("22222222-2222-2222-2222-222222222222")
	}

	return args.Name + "_id", outputs, nil
}

func (m *resourceCaptureMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	m.mu.Lock()
	result, ok := m.callResults[args.Token]
	m.mu.Unlock()
	if ok {
		return result, nil
	}
	return args.Args, nil
}

// stubCall registers a fixed response for the given Invoke token. Must be called before the test's
// pulumi.RunErr starts (Invokes may run concurrently with resource registration once it does).
func (m *resourceCaptureMocks) stubCall(token string, result resource.PropertyMap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callResults[token] = result
}

// hasAnyTypeWithPrefix reports whether any registered resource's TypeToken starts with prefix
// (e.g. "mssql:" to check for any pulumi-mssql-managed resource).
func (m *resourceCaptureMocks) hasAnyTypeWithPrefix(prefix string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for typeToken, rs := range m.byType {
		if strings.HasPrefix(typeToken, prefix) && len(rs) > 0 {
			return true
		}
	}
	return false
}

// retainOnDelete reports the RetainOnDelete flag the engine received for the resource registered
// under name (false if the resource wasn't captured or the flag wasn't set).
func (m *resourceCaptureMocks) retainOnDelete(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	args, ok := m.byName[name]
	if !ok || args.RegisterRPC == nil {
		return false
	}
	return args.RegisterRPC.GetRetainOnDelete()
}
