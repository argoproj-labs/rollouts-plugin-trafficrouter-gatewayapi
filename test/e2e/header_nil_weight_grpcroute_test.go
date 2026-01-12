//go:build flaky

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// TestHeaderNilWeightGRPCRoute verifies that header route weights remain nil
// (100% traffic to canary for matching requests) while the main route's canary
// weight is updated according to setWeight steps.
func TestHeaderNilWeightGRPCRoute(t *testing.T) {
	feature := features.New("Header nil weight GRPCRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupHeaderNilWeightGRPCRouteEnv,
	).Assess(
		"Testing header nil weight GRPCRoute feature",
		testHeaderNilWeightGRPCRoute,
	).Teardown(
		teardownHeaderNilWeightGRPCRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupHeaderNilWeightGRPCRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var grpcRoute gatewayv1.GRPCRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)
	firstGRPCRouteFile, err := os.Open(GRPC_ROUTE_HEADER_NIL_WEIGHT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", GRPC_ROUTE_HEADER_NIL_WEIGHT_PATH, err)
		t.Error()
		return ctx
	}
	defer firstGRPCRouteFile.Close()
	logrus.Infof("file %q was opened", GRPC_ROUTE_HEADER_NIL_WEIGHT_PATH)
	rolloutFile, err := os.Open(GRPC_ROUTE_HEADER_NIL_WEIGHT_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", GRPC_ROUTE_HEADER_NIL_WEIGHT_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", GRPC_ROUTE_HEADER_NIL_WEIGHT_ROLLOUT_PATH)
	err = decoder.Decode(firstGRPCRouteFile, &grpcRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", GRPC_ROUTE_HEADER_NIL_WEIGHT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", GRPC_ROUTE_HEADER_NIL_WEIGHT_PATH)
	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", GRPC_ROUTE_HEADER_NIL_WEIGHT_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", GRPC_ROUTE_HEADER_NIL_WEIGHT_ROLLOUT_PATH)
	grpcRouteObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&grpcRoute)
	if err != nil {
		logrus.Errorf("grpcRoute %q converting to unstructured was failed: %s", grpcRoute.GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was converted to unstructured", grpcRoute.GetName())
	resourcesMap[GRPC_ROUTE_KEY] = &unstructured.Unstructured{
		Object: grpcRouteObject,
	}
	rolloutObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&rollout)
	if err != nil {
		logrus.Errorf("rollout %q converting to unstructured was failed: %s", rollout.GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was converted to unstructured", rollout.GetName())
	unstructured.RemoveNestedField(rolloutObject, "spec", "template", "metadata", "creationTimestamp")
	resourcesMap[ROLLOUT_KEY] = &unstructured.Unstructured{
		Object: rolloutObject,
	}
	err = clusterResources.Create(ctx, resourcesMap[GRPC_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("grpcRoute %q creation was failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was created", resourcesMap[GRPC_ROUTE_KEY].GetName())
	err = clusterResources.Create(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("rollout %q creation was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was created", resourcesMap[ROLLOUT_KEY].GetName())
	waitCondition := conditions.New(clusterResources)
	logrus.Infof("waiting for grpcRoute %q to connect with rollout %q (expecting canary weight: %d)", resourcesMap[GRPC_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), FIRST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[GRPC_ROUTE_KEY],
			getMatchGRPCRouteFetcher(t, FIRST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(MEDIUM_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("checking grpcRoute %q connection with rollout %q was failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q connected with rollout %q", resourcesMap[GRPC_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName())
	return ctx
}

func testHeaderNilWeightGRPCRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	clusterResources := config.Client().Resources()
	resourcesMap, ok := ctx.Value(RESOURCES_MAP_KEY).(map[string]*unstructured.Unstructured)
	if !ok {
		logrus.Errorf("%q type assertion was failed", RESOURCES_MAP_KEY)
		t.Error()
		return ctx
	}
	logrus.Infof("%q was type asserted", RESOURCES_MAP_KEY)
	containersObject, isFound, err := unstructured.NestedFieldNoCopy(resourcesMap[ROLLOUT_KEY].Object, strings.Split(ROLLOUT_TEMPLATE_CONTAINERS_FIELD, ".")...)
	if !isFound {
		logrus.Errorf("rollout %q field %q was not found", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_CONTAINERS_FIELD)
		t.Error()
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting rollout %q field %q was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_CONTAINERS_FIELD, err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q field %q was received", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_CONTAINERS_FIELD)
	unstructuredContainerList, ok := containersObject.([]interface{})
	if !ok {
		logrus.Errorf("rollout %q field %q type assertion was failed", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_CONTAINERS_FIELD)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q field %q was type asserted", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_CONTAINERS_FIELD)
	unstructuredContainer, ok := unstructuredContainerList[0].(map[string]interface{})
	if !ok {
		logrus.Errorf("rollout %q field %q type assertion was failed", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_FIRST_CONTAINER_FIELD)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q field %q was type asserted", resourcesMap[ROLLOUT_KEY].GetName(), ROLLOUT_TEMPLATE_FIRST_CONTAINER_FIELD)
	unstructured.RemoveNestedField(resourcesMap[ROLLOUT_KEY].Object, "metadata", "resourceVersion")
	unstructuredContainer["image"] = NEW_IMAGE_FIELD_VALUE
	serializedRollout, err := json.Marshal(resourcesMap[ROLLOUT_KEY].Object)
	if err != nil {
		logrus.Errorf("rollout %q serializing was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was serialized", resourcesMap[ROLLOUT_KEY].GetName())
	rolloutPatch := k8s.Patch{
		PatchType: types.MergePatchType,
		Data:      serializedRollout,
	}
	err = clusterResources.Patch(ctx, resourcesMap[ROLLOUT_KEY], rolloutPatch)
	if err != nil {
		logrus.Errorf("rollout %q updating was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was updated", resourcesMap[ROLLOUT_KEY].GetName())
	waitCondition := conditions.New(clusterResources)
	// Test the new behavior: header route weight should be nil (100% to canary),
	// while main route canary weight should be LAST_CANARY_ROUTE_WEIGHT
	logrus.Infof("waiting for grpcRoute %q to update with header-based routing where header route weight is nil and main route canary weight is %d", resourcesMap[GRPC_ROUTE_KEY].GetName(), LAST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[GRPC_ROUTE_KEY],
			getMatchHeaderNilWeightGRPCRouteFetcher(
				t,
				LAST_CANARY_ROUTE_WEIGHT,
				LAST_HEADER_BASED_GRPC_ROUTE_VALUE,
			),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("grpcRoute %q updating failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was updated with nil weight for header route", resourcesMap[GRPC_ROUTE_KEY].GetName())
	// Manually promote the canary by removing the pause step to allow rollout to finish
	unstructured.RemoveNestedField(resourcesMap[ROLLOUT_KEY].Object, "metadata", "resourceVersion")
	// Remove the pause step from the canary strategy steps
	steps, found, err := unstructured.NestedSlice(resourcesMap[ROLLOUT_KEY].Object, "spec", "strategy", "canary", "steps")
	if !found || err != nil {
		logrus.Errorf("rollout %q canary steps not found or error: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	// Filter out the pause step from the steps array
	var filteredSteps []interface{}
	for _, step := range steps {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			continue
		}
		// Skip any step that contains a "pause" key
		if _, hasPause := stepMap["pause"]; !hasPause {
			filteredSteps = append(filteredSteps, step)
		}
	}
	err = unstructured.SetNestedSlice(resourcesMap[ROLLOUT_KEY].Object, filteredSteps, "spec", "strategy", "canary", "steps")
	if err != nil {
		logrus.Errorf("rollout %q steps update failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	serializedRolloutPromotion, err := json.Marshal(resourcesMap[ROLLOUT_KEY].Object)
	if err != nil {
		logrus.Errorf("rollout %q promotion serializing was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q promotion was serialized", resourcesMap[ROLLOUT_KEY].GetName())
	rolloutPromotionPatch := k8s.Patch{
		PatchType: types.MergePatchType,
		Data:      serializedRolloutPromotion,
	}
	err = clusterResources.Patch(ctx, resourcesMap[ROLLOUT_KEY], rolloutPromotionPatch)
	if err != nil {
		logrus.Errorf("rollout %q promotion was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was promoted to finish canary deployment", resourcesMap[ROLLOUT_KEY].GetName())
	// Wait for rollout to reach a healthy finished state
	logrus.Infof("waiting for rollout %q to complete and reach healthy status", resourcesMap[ROLLOUT_KEY].GetName())
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[ROLLOUT_KEY],
			getRolloutHealthyFetcher(t),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("rollout %q completion was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q completed successfully", resourcesMap[ROLLOUT_KEY].GetName())
	// Wait for header route to be removed and main route to be reset
	logrus.Infof("waiting for grpcRoute %q to clean up header-based routing and reset canary weight to %d", resourcesMap[GRPC_ROUTE_KEY].GetName(), FIRST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[GRPC_ROUTE_KEY],
			getMatchHeaderNilWeightGRPCRouteFetcher(
				t,
				FIRST_CANARY_ROUTE_WEIGHT,
				LAST_HEADER_BASED_GRPC_ROUTE_VALUE,
			),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("last grpcRoute %q update failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was updated lastly", resourcesMap[GRPC_ROUTE_KEY].GetName())
	return ctx
}

func teardownHeaderNilWeightGRPCRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	clusterResources := config.Client().Resources()
	resourcesMap, ok := ctx.Value(RESOURCES_MAP_KEY).(map[string]*unstructured.Unstructured)
	if !ok {
		logrus.Errorf("%q type assertion was failed", RESOURCES_MAP_KEY)
		t.Error()
		return ctx
	}
	logrus.Infof("%q was type asserted", RESOURCES_MAP_KEY)
	err := clusterResources.Delete(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("deleting rollout %q was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was deleted", resourcesMap[ROLLOUT_KEY].GetName())
	err = clusterResources.Delete(ctx, resourcesMap[GRPC_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("deleting grpcRoute %q was failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was deleted", resourcesMap[GRPC_ROUTE_KEY].GetName())
	return ctx
}

// getMatchHeaderNilWeightGRPCRouteFetcher returns a fetcher that checks:
// 1. The main route's canary weight matches targetWeight
// 2. The header route's weight is nil (meaning 100% traffic to canary for matching requests)
func getMatchHeaderNilWeightGRPCRouteFetcher(t *testing.T, targetWeight int32, targetHeaderBasedRouteValue gatewayv1.GRPCHeaderMatch) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var grpcRoute gatewayv1.GRPCRoute
		unstructuredGRPCRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredGRPCRoute.Object, &grpcRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured grpcRoute %q to the typed grpcRoute was failed", unstructuredGRPCRoute.GetName())
			t.Error()
			return false
		}
		rules := grpcRoute.Spec.Rules
		// If header type is nil, it means header route was removed - check only main route
		if targetHeaderBasedRouteValue.Type == nil {
			return len(rules) == LAST_HEADER_BASED_RULES_LENGTH &&
				*rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
		}
		// Check that we have 2 rules (main route + header route)
		if len(rules) != FIRST_HEADER_BASED_RULES_LENGTH {
			return false
		}
		// Check main route canary weight
		if *rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight != targetWeight {
			return false
		}
		// Check header route value matches
		headerBasedRouteValue := rules[HEADER_BASED_RULE_INDEX].Matches[HEADER_BASED_MATCH_INDEX].Headers[HEADER_BASED_HEADER_INDEX]
		if !isHeaderBasedGRPCRouteValuesEqual(headerBasedRouteValue, targetHeaderBasedRouteValue) {
			return false
		}
		// NEW BEHAVIOR: Check that header route weight is nil (100% to canary for matching requests)
		weight := rules[HEADER_BASED_RULE_INDEX].BackendRefs[HEADER_BASED_BACKEND_REF_INDEX].Weight
		return weight == nil
	}
}
