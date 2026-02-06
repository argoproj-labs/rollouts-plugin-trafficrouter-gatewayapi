package e2e

import (
	"fmt"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func getMatchHTTPRouteFetcher(t *testing.T, targetWeight int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var httpRoute gatewayv1.HTTPRoute
		unstructuredHTTPRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		// logrus.Info("k8s object was type asserted")
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredHTTPRoute.Object, &httpRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured httpRoute %q to the typed httpRoute was failed", unstructuredHTTPRoute.GetName())
			t.Error()
			return false
		}
		// logrus.Infof("unstructured httpRoute %q was converted to the typed httpRoute", httpRoute.GetName())
		return *httpRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}

func getMatchGRPCRouteFetcher(t *testing.T, targetWeight int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var grpcRoute gatewayv1.GRPCRoute
		unstructuredGRPCRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		// logrus.Info("k8s object was type asserted")
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredGRPCRoute.Object, &grpcRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured grpcRoute %q to the typed grpcRoute was failed", unstructuredGRPCRoute.GetName())
			t.Error()
			return false
		}
		// logrus.Infof("Looking for grpcRoute %q to reach weight %d", grpcRoute.GetName(), targetWeight)
		return *grpcRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}

// getRolloutHealthyFetcher returns a function that checks if a rollout has reached a healthy, stable state.
// It verifies multiple conditions beyond just phase to avoid race conditions with informer cache updates.
// This implements the principle from Argo Rollouts PR #1698 - ensuring cache consistency.
func getRolloutHealthyFetcher(t *testing.T, desiredReplicas int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var rollout v1alpha1.Rollout
		unstructuredRollout, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s rollout object type assertion was failed")
			t.Error()
			return false
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredRollout.Object, &rollout)
		if err != nil {
			logrus.Errorf("conversation from unstructured rollout %q to the typed rollout was failed", unstructuredRollout.GetName())
			t.Error()
			return false
		}

		// Check Phase is Healthy
		if rollout.Status.Phase != "Healthy" {
			logrus.Debugf("rollout %q phase is %q, waiting for Healthy", rollout.Name, rollout.Status.Phase)
			return false
		}

		// Verify ObservedGeneration matches current generation (controller has processed latest change)
		// This is critical - similar to the issue fixed in Argo Rollouts PR #1698
		// Note: ObservedGeneration is stored as string in v1alpha1.Rollout
		observedGen := rollout.Status.ObservedGeneration
		currentGen := fmt.Sprintf("%d", rollout.Generation)
		if observedGen != currentGen {
			logrus.Debugf("rollout %q observedGeneration (%s) != generation (%s), waiting for sync",
				rollout.Name, observedGen, currentGen)
			return false
		}

		// Verify all replicas are ready (actual pod state is stable)
		if rollout.Status.Replicas != desiredReplicas {
			logrus.Debugf("rollout %q has %d replicas, want %d", rollout.Name, rollout.Status.Replicas, desiredReplicas)
			return false
		}
		if rollout.Status.ReadyReplicas != desiredReplicas {
			logrus.Debugf("rollout %q has %d ready replicas, want %d", rollout.Name, rollout.Status.ReadyReplicas, desiredReplicas)
			return false
		}
		if rollout.Status.AvailableReplicas != desiredReplicas {
			logrus.Debugf("rollout %q has %d available replicas, want %d", rollout.Name, rollout.Status.AvailableReplicas, desiredReplicas)
			return false
		}

		logrus.Infof("rollout %q is healthy and stable (generation=%d, replicas=%d/%d ready)",
			rollout.Name, rollout.Generation, rollout.Status.ReadyReplicas, desiredReplicas)
		return true
	}
}
