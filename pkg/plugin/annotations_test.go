package plugin

import (
	"testing"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
	"github.com/stretchr/testify/assert"
)

func TestRolloutIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		info     *RolloutInfo
		expected string
	}{
		{
			name:     "nil info returns empty string",
			info:     nil,
			expected: "",
		},
		{
			name:     "valid info returns namespace/name",
			info:     &RolloutInfo{Namespace: "default", Name: "my-rollout"},
			expected: "default/my-rollout",
		},
		{
			name:     "empty namespace",
			info:     &RolloutInfo{Namespace: "", Name: "my-rollout"},
			expected: "/my-rollout",
		},
		{
			name:     "empty name",
			info:     &RolloutInfo{Namespace: "default", Name: ""},
			expected: "default/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rolloutIdentifier(tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseActiveRollouts(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			name:     "empty string returns nil",
			value:    "",
			expected: nil,
		},
		{
			name:     "single rollout",
			value:    "default/rollout-a",
			expected: []string{"default/rollout-a"},
		},
		{
			name:     "multiple rollouts",
			value:    "default/rollout-a,default/rollout-b",
			expected: []string{"default/rollout-a", "default/rollout-b"},
		},
		{
			name:     "handles whitespace",
			value:    "default/rollout-a, default/rollout-b , default/rollout-c",
			expected: []string{"default/rollout-a", "default/rollout-b", "default/rollout-c"},
		},
		{
			name:     "filters empty parts",
			value:    "default/rollout-a,,default/rollout-b",
			expected: []string{"default/rollout-a", "default/rollout-b"},
		},
		{
			name:     "only whitespace parts",
			value:    "  ,  ,  ",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseActiveRollouts(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatActiveRollouts(t *testing.T) {
	tests := []struct {
		name     string
		rollouts []string
		expected string
	}{
		{
			name:     "nil returns empty string",
			rollouts: nil,
			expected: "",
		},
		{
			name:     "empty slice returns empty string",
			rollouts: []string{},
			expected: "",
		},
		{
			name:     "single rollout",
			rollouts: []string{"default/rollout-a"},
			expected: "default/rollout-a",
		},
		{
			name:     "multiple rollouts are sorted",
			rollouts: []string{"default/rollout-b", "default/rollout-a"},
			expected: "default/rollout-a,default/rollout-b",
		},
		{
			name:     "already sorted",
			rollouts: []string{"default/rollout-a", "default/rollout-b"},
			expected: "default/rollout-a,default/rollout-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatActiveRollouts(tt.rollouts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddRolloutToActive(t *testing.T) {
	tests := []struct {
		name           string
		initial        []string
		rolloutID      string
		expectedResult bool
		expectedSlice  []string
	}{
		{
			name:           "add to empty slice",
			initial:        []string{},
			rolloutID:      "default/rollout-a",
			expectedResult: true,
			expectedSlice:  []string{"default/rollout-a"},
		},
		{
			name:           "add to existing slice",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "default/rollout-b",
			expectedResult: true,
			expectedSlice:  []string{"default/rollout-a", "default/rollout-b"},
		},
		{
			name:           "duplicate not added",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "default/rollout-a",
			expectedResult: false,
			expectedSlice:  []string{"default/rollout-a"},
		},
		{
			name:           "empty rolloutID not added",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "",
			expectedResult: false,
			expectedSlice:  []string{"default/rollout-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := make([]string, len(tt.initial))
			copy(slice, tt.initial)
			result := addRolloutToActive(&slice, tt.rolloutID)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedSlice, slice)
		})
	}
}

func TestRemoveRolloutFromActive(t *testing.T) {
	tests := []struct {
		name           string
		initial        []string
		rolloutID      string
		expectedResult bool
		expectedSlice  []string
	}{
		{
			name:           "remove from empty slice",
			initial:        []string{},
			rolloutID:      "default/rollout-a",
			expectedResult: false,
			expectedSlice:  []string{},
		},
		{
			name:           "remove existing element",
			initial:        []string{"default/rollout-a", "default/rollout-b"},
			rolloutID:      "default/rollout-a",
			expectedResult: true,
			expectedSlice:  []string{"default/rollout-b"},
		},
		{
			name:           "remove last element",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "default/rollout-a",
			expectedResult: true,
			expectedSlice:  []string{},
		},
		{
			name:           "remove non-existing element",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "default/rollout-b",
			expectedResult: false,
			expectedSlice:  []string{"default/rollout-a"},
		},
		{
			name:           "empty rolloutID",
			initial:        []string{"default/rollout-a"},
			rolloutID:      "",
			expectedResult: false,
			expectedSlice:  []string{"default/rollout-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := make([]string, len(tt.initial))
			copy(slice, tt.initial)
			result := removeRolloutFromActive(&slice, tt.rolloutID)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedSlice, slice)
		})
	}
}

func TestActiveRolloutsAnnotationKey(t *testing.T) {
	t.Run("returns default when not set", func(t *testing.T) {
		config := &GatewayAPITrafficRouting{}
		assert.Equal(t, defaults.ActiveRolloutsAnnotationKey, config.activeRolloutsAnnotationKey())
	})

	t.Run("returns custom when set", func(t *testing.T) {
		customKey := "custom.io/my-annotation"
		config := &GatewayAPITrafficRouting{
			ActiveRolloutsAnnotationKey: customKey,
		}
		assert.Equal(t, customKey, config.activeRolloutsAnnotationKey())
	})
}
