package plugin

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ensureInProgressMetadata manages the in-progress label and active rollouts annotation on a route object.
// It uses reference counting to track active rollouts: the label is only removed when no rollouts are active.
// Returns true if the object's metadata was modified.
func ensureInProgressMetadata(obj metav1.Object, desiredWeight int32, config *GatewayAPITrafficRouting, rolloutInfo *RolloutInfo) bool {
	if obj == nil || config == nil || config.DisableInProgressLabel || rolloutInfo == nil {
		return false
	}

	key := config.inProgressLabelKey()
	if key == "" {
		return false
	}

	annotationKey := config.activeRolloutsAnnotationKey()
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	activeRollouts := parseActiveRollouts(annotations[annotationKey])
	rolloutID := rolloutIdentifier(rolloutInfo)
	modified := false

	if desiredWeight == 0 {
		// Remove this rollout from active set
		if removeRolloutFromActive(&activeRollouts, rolloutID) {
			modified = true
		}
	} else {
		// Add this rollout to active set
		if addRolloutToActive(&activeRollouts, rolloutID) {
			modified = true
		}
	}

	// Update annotation
	if len(activeRollouts) == 0 {
		if _, exists := annotations[annotationKey]; exists {
			delete(annotations, annotationKey)
			modified = true
		}
	} else {
		newValue := formatActiveRollouts(activeRollouts)
		if annotations[annotationKey] != newValue {
			annotations[annotationKey] = newValue
			modified = true
		}
	}
	obj.SetAnnotations(annotations)

	// Manage label based on active rollouts
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	if len(activeRollouts) == 0 {
		// No active rollouts, remove label
		if _, exists := labels[key]; exists {
			delete(labels, key)
			obj.SetLabels(labels)
			modified = true
		}
	} else {
		// Active rollouts exist, ensure label is present
		value := config.inProgressLabelValue()
		if labels[key] != value {
			labels[key] = value
			obj.SetLabels(labels)
			modified = true
		}
	}

	return modified
}
