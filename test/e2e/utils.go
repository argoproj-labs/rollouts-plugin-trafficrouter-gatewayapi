package e2e

import (
	"testing"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
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
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredHTTPRoute.Object, &httpRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured httpRoute %q to the typed httpRoute was failed", unstructuredHTTPRoute.GetName())
			t.Error()
			return false
		}
		return *httpRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}

func getMatchHTTPRouteWithLabelFetcher(t *testing.T, targetWeight int32, expectLabel bool) func(k8s.Object) bool {
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
		if *httpRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight != targetWeight {
			return false
		}

		labels := httpRoute.GetLabels()
		value, ok := labels[defaults.InProgressLabelKey]
		if expectLabel {
			return ok && value == defaults.InProgressLabelValue
		}
		// we explicitly expect the label to be absent
		return !ok
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
