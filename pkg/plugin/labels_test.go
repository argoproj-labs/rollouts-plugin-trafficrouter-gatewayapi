package plugin

import (
	"testing"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockObject implements metav1.Object for testing
type mockObject struct {
	metav1.ObjectMeta
}

func (m *mockObject) GetObjectKind() interface{}  { return nil }
func (m *mockObject) DeepCopyObject() interface{} { return nil }

func TestEnsureInProgressMetadata(t *testing.T) {
	t.Run("nil object returns false", func(t *testing.T) {
		config := &GatewayAPITrafficRouting{}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout"}
		result := ensureInProgressMetadata(nil, 30, config, rolloutInfo)
		assert.False(t, result)
	})

	t.Run("nil config returns false", func(t *testing.T) {
		obj := &mockObject{}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout"}
		result := ensureInProgressMetadata(obj, 30, nil, rolloutInfo)
		assert.False(t, result)
	})

	t.Run("disabled in-progress label returns false", func(t *testing.T) {
		obj := &mockObject{}
		config := &GatewayAPITrafficRouting{DisableInProgressLabel: true}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout"}
		result := ensureInProgressMetadata(obj, 30, config, rolloutInfo)
		assert.False(t, result)
	})

	t.Run("nil rollout info returns false", func(t *testing.T) {
		obj := &mockObject{}
		config := &GatewayAPITrafficRouting{}
		result := ensureInProgressMetadata(obj, 30, config, nil)
		assert.False(t, result)
	})

	t.Run("adds label and annotation when weight > 0", func(t *testing.T) {
		obj := &mockObject{}
		config := &GatewayAPITrafficRouting{}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout-a"}

		result := ensureInProgressMetadata(obj, 30, config, rolloutInfo)

		assert.True(t, result)
		assert.Equal(t, defaults.InProgressLabelValue, obj.Labels[defaults.InProgressLabelKey])
		assert.Equal(t, "default/rollout-a", obj.Annotations[defaults.ActiveRolloutsAnnotationKey])
	})

	t.Run("multiple rollouts accumulate in annotation", func(t *testing.T) {
		obj := &mockObject{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					defaults.InProgressLabelKey: defaults.InProgressLabelValue,
				},
				Annotations: map[string]string{
					defaults.ActiveRolloutsAnnotationKey: "default/rollout-a",
				},
			},
		}
		config := &GatewayAPITrafficRouting{}
		rolloutInfoB := &RolloutInfo{Namespace: "default", Name: "rollout-b"}

		result := ensureInProgressMetadata(obj, 50, config, rolloutInfoB)

		assert.True(t, result)
		assert.Equal(t, defaults.InProgressLabelValue, obj.Labels[defaults.InProgressLabelKey])
		assert.Equal(t, "default/rollout-a,default/rollout-b", obj.Annotations[defaults.ActiveRolloutsAnnotationKey])
	})

	t.Run("first rollout finishes but label remains for second rollout", func(t *testing.T) {
		obj := &mockObject{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					defaults.InProgressLabelKey: defaults.InProgressLabelValue,
				},
				Annotations: map[string]string{
					defaults.ActiveRolloutsAnnotationKey: "default/rollout-a,default/rollout-b",
				},
			},
		}
		config := &GatewayAPITrafficRouting{}
		rolloutInfoA := &RolloutInfo{Namespace: "default", Name: "rollout-a"}

		result := ensureInProgressMetadata(obj, 0, config, rolloutInfoA)

		assert.True(t, result)
		// Label should still be present
		assert.Equal(t, defaults.InProgressLabelValue, obj.Labels[defaults.InProgressLabelKey])
		// Only rollout-b should remain
		assert.Equal(t, "default/rollout-b", obj.Annotations[defaults.ActiveRolloutsAnnotationKey])
	})

	t.Run("last rollout finishes removes both label and annotation", func(t *testing.T) {
		obj := &mockObject{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					defaults.InProgressLabelKey: defaults.InProgressLabelValue,
				},
				Annotations: map[string]string{
					defaults.ActiveRolloutsAnnotationKey: "default/rollout-a",
				},
			},
		}
		config := &GatewayAPITrafficRouting{}
		rolloutInfoA := &RolloutInfo{Namespace: "default", Name: "rollout-a"}

		result := ensureInProgressMetadata(obj, 0, config, rolloutInfoA)

		assert.True(t, result)
		// Label should be removed
		_, hasLabel := obj.Labels[defaults.InProgressLabelKey]
		assert.False(t, hasLabel)
		// Annotation should be removed
		_, hasAnnotation := obj.Annotations[defaults.ActiveRolloutsAnnotationKey]
		assert.False(t, hasAnnotation)
	})

	t.Run("no change when same rollout sets weight again", func(t *testing.T) {
		obj := &mockObject{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					defaults.InProgressLabelKey: defaults.InProgressLabelValue,
				},
				Annotations: map[string]string{
					defaults.ActiveRolloutsAnnotationKey: "default/rollout-a",
				},
			},
		}
		config := &GatewayAPITrafficRouting{}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout-a"}

		result := ensureInProgressMetadata(obj, 50, config, rolloutInfo)

		assert.False(t, result) // No modification needed
		assert.Equal(t, defaults.InProgressLabelValue, obj.Labels[defaults.InProgressLabelKey])
		assert.Equal(t, "default/rollout-a", obj.Annotations[defaults.ActiveRolloutsAnnotationKey])
	})

	t.Run("custom annotation key is used", func(t *testing.T) {
		obj := &mockObject{}
		customAnnotationKey := "custom.io/active-rollouts"
		config := &GatewayAPITrafficRouting{
			ActiveRolloutsAnnotationKey: customAnnotationKey,
		}
		rolloutInfo := &RolloutInfo{Namespace: "default", Name: "rollout-a"}

		result := ensureInProgressMetadata(obj, 30, config, rolloutInfo)

		assert.True(t, result)
		assert.Equal(t, "default/rollout-a", obj.Annotations[customAnnotationKey])
		_, hasDefaultAnnotation := obj.Annotations[defaults.ActiveRolloutsAnnotationKey]
		assert.False(t, hasDefaultAnnotation)
	})
}
