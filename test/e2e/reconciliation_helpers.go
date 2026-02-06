package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
)

// forceRolloutReconciliation adds a timestamp annotation to force the controller to reconcile
// This increments the metadata.generation and ensures the controller processes the change immediately
func forceRolloutReconciliation(ctx context.Context, clusterResources *resources.Resources, rollout *unstructured.Unstructured) error {
	logrus.Infof("forcing reconciliation for rollout %q by adding timestamp annotation", rollout.GetName())

	// Get current annotations
	annotations := rollout.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add a timestamp annotation to force reconciliation
	annotations["test.e2e.rollouts-plugin/force-reconcile"] = time.Now().Format(time.RFC3339Nano)
	rollout.SetAnnotations(annotations)

	// Update the rollout
	err := clusterResources.Update(ctx, rollout)
	if err != nil {
		return fmt.Errorf("failed to update rollout with reconciliation annotation: %w", err)
	}

	logrus.Infof("rollout %q annotation updated to force reconciliation", rollout.GetName())
	return nil
}

// waitForGenerationObserved waits for the controller to observe a specific generation
// This is more reliable than just waiting an arbitrary time
func waitForGenerationObserved(ctx context.Context, clusterResources *resources.Resources, rollout *unstructured.Unstructured, expectedGeneration int64, timeout time.Duration) error {
	logrus.Infof("waiting for rollout %q to have observedGeneration=%d", rollout.GetName(), expectedGeneration)

	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for observedGeneration to reach %d", expectedGeneration)
		}

		// Get latest rollout
		err := clusterResources.Get(ctx, rollout.GetName(), rollout.GetNamespace(), rollout)
		if err != nil {
			return fmt.Errorf("failed to get rollout: %w", err)
		}

		// Check observedGeneration
		observedGen, found, err := unstructured.NestedString(rollout.Object, "status", "observedGeneration")
		if err != nil {
			return fmt.Errorf("failed to get observedGeneration: %w", err)
		}
		if !found {
			logrus.Debugf("rollout %q does not have observedGeneration yet", rollout.GetName())
			time.Sleep(time.Second)
			continue
		}

		currentGen := rollout.GetGeneration()
		logrus.Debugf("rollout %q: generation=%d, observedGeneration=%s", rollout.GetName(), currentGen, observedGen)

		if observedGen == fmt.Sprintf("%d", expectedGeneration) {
			logrus.Infof("rollout %q observedGeneration reached %d", rollout.GetName(), expectedGeneration)
			return nil
		}

		time.Sleep(time.Second)
	}
}
