package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrationJobsEnabled(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"", true},
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"FALSE", false},
		{"no", true},
		{"garbage", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.want, migrationJobsEnabled(tt.value))
		})
	}
}
