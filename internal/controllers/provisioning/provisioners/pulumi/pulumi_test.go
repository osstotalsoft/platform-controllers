package pulumi

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
)

func TestHasSucceededUpdate(t *testing.T) {
	tests := []struct {
		name    string
		history []auto.UpdateSummary
		want    bool
	}{
		{"no history", nil, false},
		{"succeeded update", []auto.UpdateSummary{{Kind: "update", Result: "succeeded"}}, true},
		{"failed update only", []auto.UpdateSummary{{Kind: "update", Result: "failed"}}, false},
		{"in-progress update only", []auto.UpdateSummary{{Kind: "update", Result: "in-progress"}}, false},
		{"succeeded refresh only", []auto.UpdateSummary{{Kind: "refresh", Result: "succeeded"}}, false},
		{"succeeded destroy only", []auto.UpdateSummary{{Kind: "destroy", Result: "succeeded"}}, false},
		{"failed latest update after an earlier success", []auto.UpdateSummary{
			{Kind: "update", Result: "failed"},
			{Kind: "refresh", Result: "succeeded"},
			{Kind: "update", Result: "succeeded"},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasSucceededUpdate(tt.history))
		})
	}
}

func TestStackNameFor(t *testing.T) {
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant1"}}
	platform := &platformv1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "dev"}}

	assert.Equal(t, "tenant1-d1", stackNameFor(tenant, "d1"))
	assert.Equal(t, "platform-d1", stackNameFor(platform, "d1"))
}
