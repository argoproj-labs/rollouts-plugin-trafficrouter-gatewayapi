package plugin

import (
	"sort"
	"strings"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
)

// activeRolloutsAnnotationKey returns the annotation key used to track active rollouts
func (c *GatewayAPITrafficRouting) activeRolloutsAnnotationKey() string {
	if c.ActiveRolloutsAnnotationKey != "" {
		return c.ActiveRolloutsAnnotationKey
	}
	return defaults.ActiveRolloutsAnnotationKey
}

// rolloutIdentifier creates a unique identifier for a rollout in format "namespace/name"
func rolloutIdentifier(info *RolloutInfo) string {
	if info == nil {
		return ""
	}
	return info.Namespace + "/" + info.Name
}

// parseActiveRollouts parses a comma-separated list of rollout identifiers
func parseActiveRollouts(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// formatActiveRollouts formats a list of rollout identifiers as a comma-separated string
func formatActiveRollouts(rollouts []string) string {
	if len(rollouts) == 0 {
		return ""
	}
	// Sort for deterministic output
	sorted := make([]string, len(rollouts))
	copy(sorted, rollouts)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// addRolloutToActive adds a rollout identifier to the active set if not already present
// Returns true if the rollout was added (i.e., it wasn't already present)
func addRolloutToActive(rollouts *[]string, rolloutID string) bool {
	if rolloutID == "" {
		return false
	}
	for _, r := range *rollouts {
		if r == rolloutID {
			return false // Already present
		}
	}
	*rollouts = append(*rollouts, rolloutID)
	return true
}

// removeRolloutFromActive removes a rollout identifier from the active set
// Returns true if the rollout was removed (i.e., it was present)
func removeRolloutFromActive(rollouts *[]string, rolloutID string) bool {
	if rolloutID == "" {
		return false
	}
	for i, r := range *rollouts {
		if r == rolloutID {
			*rollouts = append((*rollouts)[:i], (*rollouts)[i+1:]...)
			return true
		}
	}
	return false
}
