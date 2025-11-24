package e2e

import (
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

func getMatchNewHeaderBasedHTTPRouteFetcher(t *testing.T, targetHeaderBasedRouteValue gatewayv1.HTTPHeaderMatch) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var httpRoute gatewayv1.HTTPRoute
		unstructuredHTTPRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			if typed, ok := obj.(*gatewayv1.HTTPRoute); ok {
				httpRoute = *typed
			} else {
				logrus.Error("k8s object type assertion was failed")
				t.Error()
				return false
			}
		} else {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredHTTPRoute.Object, &httpRoute)
			if err != nil {
				logrus.Errorf("conversation from unstructured httpRoute %q to the typed httpRoute was failed", unstructuredHTTPRoute.GetName())
				t.Error()
				return false
			}
		}

		rules := httpRoute.Spec.Rules
		if len(rules) != 1 {
			return false
		}
		if len(rules[0].Matches) == 0 {
			return false
		}
		found := false
		for _, match := range rules[0].Matches {
			for _, header := range match.Headers {
				if isHeaderBasedHTTPRouteValuesEqual(header, targetHeaderBasedRouteValue) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		return found
	}
}

func isHeaderBasedHTTPRouteValuesEqual(first, second gatewayv1.HTTPHeaderMatch) bool {
	return first.Name == second.Name && *first.Type == *second.Type && first.Value == second.Value
}

func getRolloutHealthyFetcher(t *testing.T) func(k8s.Object) bool {
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
		// Check if rollout is healthy (completed successfully)
		// A rollout is considered finished when its phase is "Healthy"
		return rollout.Status.Phase == "Healthy"
	}
}
