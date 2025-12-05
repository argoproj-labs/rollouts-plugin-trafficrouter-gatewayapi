package plugin

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
)

func ensureInProgressLabel(obj metav1.Object, desiredWeight int32, config *GatewayAPITrafficRouting) bool {
	if obj == nil || config == nil || config.DisableInProgressLabel {
		return false
	}

	key := config.inProgressLabelKey()
	if key == "" {
		return false
	}

	labels := obj.GetLabels()
	if desiredWeight == 0 {
		if labels == nil {
			return false
		}
		if _, ok := labels[key]; ok {
			delete(labels, key)
			obj.SetLabels(labels)
			return true
		}
		return false
	}

	value := config.inProgressLabelValue()
	if labels == nil {
		labels = make(map[string]string)
	}
	if current, ok := labels[key]; ok && current == value {
		return false
	}
	labels[key] = value
	obj.SetLabels(labels)
	return true
}

func (c *GatewayAPITrafficRouting) inProgressLabelKey() string {
	if c.InProgressLabelKey != "" {
		return c.InProgressLabelKey
	}
	return defaults.InProgressLabelKey
}

func (c *GatewayAPITrafficRouting) inProgressLabelValue() string {
	if c.InProgressLabelValue != "" {
		return c.InProgressLabelValue
	}
	return defaults.InProgressLabelValue
}
