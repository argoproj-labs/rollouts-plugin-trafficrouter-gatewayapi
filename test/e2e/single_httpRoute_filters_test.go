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

func TestSingleHTTPRouteWithFilters(t *testing.T) {
	feature := features.New("Single HTTPRoute with filters feature").Setup(
		setupEnvironment,
	).Setup(
		setupSingleHTTPRouteFiltersEnv,
	).Assess(
		"Testing HTTPRoute filter preservation",
		testSingleHTTPRouteFilters,
	).Teardown(
		teardownSingleHTTPRouteFiltersEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupSingleHTTPRouteFiltersEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var httpRoute gatewayv1.HTTPRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)

	httpRouteFile, err := os.Open(HTTP_ROUTE_FILTERS_PATH)
	if err != nil {
		logrus.Errorf("file %q opening was failed: %s", HTTP_ROUTE_FILTERS_PATH, err)
		t.Error()
		return ctx
	}
	defer httpRouteFile.Close()
	logrus.Infof("file %q was opened", HTTP_ROUTE_FILTERS_PATH)

	rolloutFile, err := os.Open(HTTP_ROUTE_FILTERS_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q opening was failed: %s", HTTP_ROUTE_FILTERS_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", HTTP_ROUTE_FILTERS_ROLLOUT_PATH)

	err = decoder.Decode(httpRouteFile, &httpRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_FILTERS_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", HTTP_ROUTE_FILTERS_PATH)

	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_FILTERS_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", HTTP_ROUTE_FILTERS_ROLLOUT_PATH)

	httpRouteObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&httpRoute)
	if err != nil {
		logrus.Errorf("httpRoute %q converting to unstructured was failed: %s", httpRoute.GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("httpRoute %q was converted to unstructured", httpRoute.GetName())
	resourcesMap[HTTP_ROUTE_KEY] = &unstructured.Unstructured{
		Object: httpRouteObject,
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

	err = clusterResources.Create(ctx, resourcesMap[HTTP_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("httpRoute %q creation was failed: %s", resourcesMap[HTTP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("httpRoute %q was created", resourcesMap[HTTP_ROUTE_KEY].GetName())

	err = clusterResources.Create(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("rollout %q creation was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was created", resourcesMap[ROLLOUT_KEY].GetName())

	waitCondition := conditions.New(clusterResources)
	logrus.Infof("waiting for httpRoute %q to connect with rollout %q (expecting canary weight: %d)", resourcesMap[HTTP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), FIRST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[HTTP_ROUTE_KEY],
			getMatchHTTPRouteFetcher(t, FIRST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(MEDIUM_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("checking httpRoute %q connection with rollout %q was failed: %s", resourcesMap[HTTP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("httpRoute %q connected with rollout %q", resourcesMap[HTTP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName())
	return ctx
}

func testSingleHTTPRouteFilters(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	clusterResources := config.Client().Resources()
	resourcesMap, ok := ctx.Value(RESOURCES_MAP_KEY).(map[string]*unstructured.Unstructured)
	if !ok {
		logrus.Errorf("%q type assertion was failed", RESOURCES_MAP_KEY)
		t.Error()
		return ctx
	}
	logrus.Infof("%q was type asserted", RESOURCES_MAP_KEY)

	httpRouteName := resourcesMap[HTTP_ROUTE_KEY].GetName()

	// Update rollout image to trigger progression through steps
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

	// Wait for httpRoute to update with header-based routing and verify filters are preserved
	waitCondition := conditions.New(clusterResources)
	logrus.Infof("waiting for httpRoute %q to update with header-based routing and canary weight %d", resourcesMap[HTTP_ROUTE_KEY].GetName(), LAST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[HTTP_ROUTE_KEY],
			func(obj k8s.Object) bool {
				unstructuredHTTPRoute, ok := obj.(*unstructured.Unstructured)
				if !ok {
					logrus.Error("k8s object type assertion was failed")
					return false
				}

				// Check if we have 2 rules (original + header route)
				rules, found, err := unstructured.NestedSlice(unstructuredHTTPRoute.Object, "spec", "rules")
				if !found || err != nil {
					return false
				}

				// Should have exactly 2 rules now (original + header-based)
				return len(rules) == 2
			},
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)

	if err != nil {
		logrus.Errorf("httpRoute %q did not create header-based routing: %s", httpRouteName, err)
		t.Errorf("httpRoute %q did not create header-based routing: %s", httpRouteName, err)
		return ctx
	}
	logrus.Infof("httpRoute %q created header-based routing", httpRouteName)

	// Verify that we have the original rule plus the header-based rule using unstructured access
	rules, found, err := unstructured.NestedSlice(resourcesMap[HTTP_ROUTE_KEY].Object, "spec", "rules")
	if !found {
		logrus.Errorf("httpRoute %q rules field was not found", httpRouteName)
		t.Errorf("httpRoute %q rules field was not found", httpRouteName)
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting httpRoute %q rules was failed: %s", httpRouteName, err)
		t.Errorf("getting httpRoute %q rules was failed: %s", httpRouteName, err)
		return ctx
	}

	if len(rules) != 2 {
		logrus.Errorf("httpRoute %q should have 2 rules (original + header), but has %d", httpRouteName, len(rules))
		t.Errorf("httpRoute %q should have 2 rules (original + header), but has %d", httpRouteName, len(rules))
		return ctx
	}

	originalRule, ok := rules[0].(map[string]interface{})
	if !ok {
		logrus.Errorf("httpRoute %q original rule type assertion failed", httpRouteName)
		t.Errorf("httpRoute %q original rule type assertion failed", httpRouteName)
		return ctx
	}

	headerRule, ok := rules[1].(map[string]interface{})
	if !ok {
		logrus.Errorf("httpRoute %q header rule type assertion failed", httpRouteName)
		t.Errorf("httpRoute %q header rule type assertion failed", httpRouteName)
		return ctx
	}

	// Verify original rule still has all filters
	originalFilters, found, err := unstructured.NestedSlice(originalRule, "filters")
	if !found {
		logrus.Errorf("httpRoute %q original rule filters field was not found", httpRouteName)
		t.Errorf("httpRoute %q original rule filters field was not found", httpRouteName)
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting httpRoute %q original rule filters was failed: %s", httpRouteName, err)
		t.Errorf("getting httpRoute %q original rule filters was failed: %s", httpRouteName, err)
		return ctx
	}

	expectedFiltersCount := 4 // RequestHeaderModifier, ResponseHeaderModifier, RequestMirror, URLRewrite
	if len(originalFilters) != expectedFiltersCount {
		logrus.Errorf("original rule should have %d filters, but has %d", expectedFiltersCount, len(originalFilters))
		t.Errorf("original rule should have %d filters, but has %d", expectedFiltersCount, len(originalFilters))
		return ctx
	}

	// Verify header rule has copied all filters from original rule
	headerFilters, found, err := unstructured.NestedSlice(headerRule, "filters")
	if !found {
		logrus.Errorf("httpRoute %q header rule filters field was not found", httpRouteName)
		t.Errorf("httpRoute %q header rule filters field was not found", httpRouteName)
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting httpRoute %q header rule filters was failed: %s", httpRouteName, err)
		t.Errorf("getting httpRoute %q header rule filters was failed: %s", httpRouteName, err)
		return ctx
	}

	if len(headerFilters) != len(originalFilters) {
		logrus.Errorf("header rule should have same number of filters as original (%d), but has %d", len(originalFilters), len(headerFilters))
		t.Errorf("header rule should have same number of filters as original (%d), but has %d", len(originalFilters), len(headerFilters))
		return ctx
	}

	// Verify specific filter types are preserved
	filterTypes := make(map[string]bool)
	for _, filter := range headerFilters {
		filterMap, ok := filter.(map[string]interface{})
		if !ok {
			logrus.Errorf("httpRoute %q header rule filter type assertion failed", httpRouteName)
			t.Errorf("httpRoute %q header rule filter type assertion failed", httpRouteName)
			return ctx
		}
		filterType, found, err := unstructured.NestedString(filterMap, "type")
		if !found || err != nil {
			logrus.Errorf("httpRoute %q header rule filter type not found or error: %v", httpRouteName, err)
			t.Errorf("httpRoute %q header rule filter type not found or error: %v", httpRouteName, err)
			return ctx
		}
		filterTypes[filterType] = true
	}

	expectedTypes := []string{
		"RequestHeaderModifier",
		"ResponseHeaderModifier",
		"RequestMirror",
		"URLRewrite",
	}

	for _, expectedType := range expectedTypes {
		if !filterTypes[expectedType] {
			logrus.Errorf("header rule is missing filter type: %s", expectedType)
			t.Errorf("header rule is missing filter type: %s", expectedType)
			return ctx
		}
	}

	// Verify header route has the correct header match
	headerMatches, found, err := unstructured.NestedSlice(headerRule, "matches")
	if !found {
		logrus.Errorf("httpRoute %q header rule matches field was not found", httpRouteName)
		t.Errorf("httpRoute %q header rule matches field was not found", httpRouteName)
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting httpRoute %q header rule matches was failed: %s", httpRouteName, err)
		t.Errorf("getting httpRoute %q header rule matches was failed: %s", httpRouteName, err)
		return ctx
	}

	if len(headerMatches) == 0 {
		logrus.Errorf("header rule should have matches")
		t.Errorf("header rule should have matches")
		return ctx
	}

	firstMatch, ok := headerMatches[0].(map[string]interface{})
	if !ok {
		logrus.Errorf("httpRoute %q header rule first match type assertion failed", httpRouteName)
		t.Errorf("httpRoute %q header rule first match type assertion failed", httpRouteName)
		return ctx
	}

	headers, found, err := unstructured.NestedSlice(firstMatch, "headers")
	if !found {
		logrus.Errorf("httpRoute %q header rule match headers field was not found", httpRouteName)
		t.Errorf("httpRoute %q header rule match headers field was not found", httpRouteName)
		return ctx
	}
	if err != nil {
		logrus.Errorf("getting httpRoute %q header rule match headers was failed: %s", httpRouteName, err)
		t.Errorf("getting httpRoute %q header rule match headers was failed: %s", httpRouteName, err)
		return ctx
	}

	if len(headers) == 0 {
		logrus.Errorf("header rule match should have headers")
		t.Errorf("header rule match should have headers")
		return ctx
	}

	headerMatch, ok := headers[0].(map[string]interface{})
	if !ok {
		logrus.Errorf("httpRoute %q header rule match header type assertion failed", httpRouteName)
		t.Errorf("httpRoute %q header rule match header type assertion failed", httpRouteName)
		return ctx
	}

	headerName, found, err := unstructured.NestedString(headerMatch, "name")
	if !found || err != nil {
		logrus.Errorf("httpRoute %q header match name not found or error: %v", httpRouteName, err)
		t.Errorf("httpRoute %q header match name not found or error: %v", httpRouteName, err)
		return ctx
	}

	if headerName != "X-Filter-Test" {
		logrus.Errorf("header match name should be 'X-Filter-Test', but is '%s'", headerName)
		t.Errorf("header match name should be 'X-Filter-Test', but is '%s'", headerName)
		return ctx
	}

	headerValue, found, err := unstructured.NestedString(headerMatch, "value")
	if !found || err != nil {
		logrus.Errorf("httpRoute %q header match value not found or error: %v", httpRouteName, err)
		t.Errorf("httpRoute %q header match value not found or error: %v", httpRouteName, err)
		return ctx
	}

	if headerValue != "preserve-filters" {
		logrus.Errorf("header match value should be 'preserve-filters', but is '%s'", headerValue)
		t.Errorf("header match value should be 'preserve-filters', but is '%s'", headerValue)
		return ctx
	}

	logrus.Infof("HTTPRoute filter preservation test passed successfully")
	return ctx
}

func teardownSingleHTTPRouteFiltersEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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

	err = clusterResources.Delete(ctx, resourcesMap[HTTP_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("deleting httpRoute %q was failed: %s", resourcesMap[HTTP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("httpRoute %q was deleted", resourcesMap[HTTP_ROUTE_KEY].GetName())

	return ctx
}
