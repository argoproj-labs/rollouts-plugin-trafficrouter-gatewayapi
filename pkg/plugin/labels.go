package plugin

import (
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
)

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
