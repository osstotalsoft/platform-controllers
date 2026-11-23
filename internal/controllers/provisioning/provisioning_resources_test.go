package provisioning

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInfrastructureManifestsIsEmpty sets each resource list in turn, so a resource kind added to
// InfrastructureManifests but not to IsEmpty fails here.
func TestInfrastructureManifestsIsEmpty(t *testing.T) {
	assert.True(t, (&InfrastructureManifests{}).IsEmpty())

	typ := reflect.TypeOf(InfrastructureManifests{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Slice {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			infra := &InfrastructureManifests{}
			list := reflect.MakeSlice(field.Type, 1, 1)
			list.Index(0).Set(reflect.New(field.Type.Elem().Elem()))
			reflect.ValueOf(infra).Elem().Field(i).Set(list)

			assert.False(t, infra.IsEmpty(), "IsEmpty ignores %s", field.Name)
		})
	}
}
