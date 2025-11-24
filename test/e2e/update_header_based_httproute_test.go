package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestUpdateHeaderBasedHTTPRoute(t *testing.T) {
	feature := features.New("Update header based HTTPRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupUpdateHeaderBasedHTTPRouteEnv,
	).Assess(
		"Testing update header based HTTPRoute feature",
		testUpdateHeaderBasedHTTPRoute,
	).Teardown(
		teardownUpdateHeaderBasedHTTPRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupUpdateHeaderBasedHTTPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var httpRoute gatewayv1.HTTPRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)

	// Use the same HTTPRoute as the single test
	firstHTTPRouteFile, err := os.Open(HTTP_ROUTE_HEADER_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", HTTP_ROUTE_HEADER_PATH, err)
		t.Error()
		return ctx
	}
	defer firstHTTPRouteFile.Close()

	rolloutFile, err := os.Open(HTTP_ROUTE_HEADER_UPDATE_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", HTTP_ROUTE_HEADER_UPDATE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()

	err = decoder.Decode(firstHTTPRouteFile, &httpRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_HEADER_PATH, err)
		t.Error()
		return ctx
	}

	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_HEADER_UPDATE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}

	httpRouteObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&httpRoute)
	if err != nil {
		logrus.Errorf("httpRoute %q converting to unstructured was failed: %s", httpRoute.GetName(), err)
		t.Error()
		return ctx
	}
	resourcesMap[HTTP_ROUTE_KEY] = &unstructured.Unstructured{
		Object: httpRouteObject,
	}

	rolloutObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&rollout)
	if err != nil {
		logrus.Errorf("rollout %q converting to unstructured was failed: %s", rollout.GetName(), err)
		t.Error()
		return ctx
	}
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

	err = clusterResources.Create(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("rollout %q creation was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}

	waitCondition := conditions.New(clusterResources)
	// Initial weight is 0 because rollout starts
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[HTTP_ROUTE_KEY],
			getMatchHTTPRouteFetcher(t, 0),
		),
		wait.WithTimeout(MEDIUM_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("checking httpRoute %q connection with rollout %q was failed: %s", resourcesMap[HTTP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	return ctx
}

func testUpdateHeaderBasedHTTPRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	clusterResources := config.Client().Resources()
	resourcesMap, ok := ctx.Value(RESOURCES_MAP_KEY).(map[string]*unstructured.Unstructured)
	if !ok {
		logrus.Errorf("%q type assertion was failed", RESOURCES_MAP_KEY)
		t.Error()
		return ctx
	}

	// Update rollout image to trigger update
	containersObject, _, _ := unstructured.NestedFieldNoCopy(resourcesMap[ROLLOUT_KEY].Object, strings.Split(ROLLOUT_TEMPLATE_CONTAINERS_FIELD, ".")...)
	unstructuredContainerList, _ := containersObject.([]interface{})
	unstructuredContainer, _ := unstructuredContainerList[0].(map[string]interface{})
	unstructured.RemoveNestedField(resourcesMap[ROLLOUT_KEY].Object, "metadata", "resourceVersion")
	unstructuredContainer["image"] = NEW_IMAGE_FIELD_VALUE

	serializedRollout, err := json.Marshal(resourcesMap[ROLLOUT_KEY].Object)
	if err != nil {
		t.Error(err)
		return ctx
	}

	rolloutPatch := k8s.Patch{
		PatchType: types.MergePatchType,
		Data:      serializedRollout,
	}
	err = clusterResources.Patch(ctx, resourcesMap[ROLLOUT_KEY], rolloutPatch)
	if err != nil {
		t.Error(err)
		return ctx
	}

	waitCondition := conditions.New(clusterResources)
	newRouteName := "canary-route1"
	newHttpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newRouteName,
			Namespace: "default",
		},
	}

	// 1. Verify first step: weight 10, header X-Canary-start: ten-per-cent
	headerType := gatewayv1.HeaderMatchExact
	firstHeaderMatch := gatewayv1.HTTPHeaderMatch{
		Name:  "X-Canary-start",
		Type:  &headerType,
		Value: "ten-per-cent",
	}

	logrus.Infof("waiting for new httpRoute %q to be created with first header", newRouteName)
	err = wait.For(
		waitCondition.ResourceMatch(
			newHttpRoute,
			getMatchNewHeaderBasedHTTPRouteFetcher(
				t,
				firstHeaderMatch,
			),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("first step verification failed: %s", err)
		t.Error()
		return ctx
	}

	// Promote to next step
	unstructured.RemoveNestedField(resourcesMap[ROLLOUT_KEY].Object, "metadata", "resourceVersion")

	// Remove the first pause
	steps, _, _ := unstructured.NestedSlice(resourcesMap[ROLLOUT_KEY].Object, "spec", "strategy", "canary", "steps")
	var stepsAfterFirstPause []interface{}
	pauseCount := 0
	for _, step := range steps {
		stepMap, _ := step.(map[string]interface{})
		if _, hasPause := stepMap["pause"]; hasPause {
			pauseCount++
			if pauseCount == 1 {
				continue // Skip the first pause
			}
		}
		stepsAfterFirstPause = append(stepsAfterFirstPause, step)
	}

	err = unstructured.SetNestedSlice(resourcesMap[ROLLOUT_KEY].Object, stepsAfterFirstPause, "spec", "strategy", "canary", "steps")
	if err != nil {
		t.Error(err)
		return ctx
	}

	serializedRolloutPromotion, _ := json.Marshal(resourcesMap[ROLLOUT_KEY].Object)
	rolloutPromotionPatch := k8s.Patch{
		PatchType: types.MergePatchType,
		Data:      serializedRolloutPromotion,
	}
	err = clusterResources.Patch(ctx, resourcesMap[ROLLOUT_KEY], rolloutPromotionPatch)
	if err != nil {
		t.Error(err)
		return ctx
	}

	// 2. Verify second step: weight 50, header X-Canary-middle: half-traffic
	// The route name is the same, so it should be updated.
	secondHeaderMatch := gatewayv1.HTTPHeaderMatch{
		Name:  "X-Canary-middle",
		Type:  &headerType,
		Value: "half-traffic",
	}

	logrus.Infof("waiting for httpRoute %q to be updated with second header", newRouteName)
	err = wait.For(
		waitCondition.ResourceMatch(
			newHttpRoute,
			getMatchNewHeaderBasedHTTPRouteFetcher(
				t,
				secondHeaderMatch,
			),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("second step verification failed: %s", err)
		t.Error()
		return ctx
	}

	// Promote to finish
	unstructured.RemoveNestedField(resourcesMap[ROLLOUT_KEY].Object, "metadata", "resourceVersion")
	// Remove all pauses to finish
	var stepsNoPause []interface{}
	for _, step := range steps {
		stepMap, _ := step.(map[string]interface{})
		if _, hasPause := stepMap["pause"]; !hasPause {
			stepsNoPause = append(stepsNoPause, step)
		}
	}
	unstructured.SetNestedSlice(resourcesMap[ROLLOUT_KEY].Object, stepsNoPause, "spec", "strategy", "canary", "steps")

	serializedRolloutFinish, _ := json.Marshal(resourcesMap[ROLLOUT_KEY].Object)
	rolloutFinishPatch := k8s.Patch{
		PatchType: types.MergePatchType,
		Data:      serializedRolloutFinish,
	}
	err = clusterResources.Patch(ctx, resourcesMap[ROLLOUT_KEY], rolloutFinishPatch)
	if err != nil {
		t.Error(err)
		return ctx
	}

	// Wait for rollout healthy
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[ROLLOUT_KEY],
			getRolloutHealthyFetcher(t),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		t.Error(err)
		return ctx
	}

	// 3. Verify route deletion
	logrus.Infof("waiting for new httpRoute %q to be deleted", newRouteName)
	err = wait.For(
		waitCondition.ResourceDeleted(newHttpRoute),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("route deletion verification failed: %s", err)
		t.Error()
		return ctx
	}

	return ctx
}

func teardownUpdateHeaderBasedHTTPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	clusterResources := config.Client().Resources()
	resourcesMap, ok := ctx.Value(RESOURCES_MAP_KEY).(map[string]*unstructured.Unstructured)
	if !ok {
		return ctx
	}
	clusterResources.Delete(ctx, resourcesMap[ROLLOUT_KEY])
	clusterResources.Delete(ctx, resourcesMap[HTTP_ROUTE_KEY])
	return ctx
}
