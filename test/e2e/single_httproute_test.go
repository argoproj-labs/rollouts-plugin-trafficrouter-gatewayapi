//go:build !flaky

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestSingleHTTPRoute(t *testing.T) {
	feature := features.New("Single HTTPRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupSingleHTTPRouteEnv,
	).Assess(
		"Testing single HTTPRoute feature",
		testSingleHTTPRoute,
	).Teardown(
		teardownSingleHTTPRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupSingleHTTPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var httpRoute gatewayv1.HTTPRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)
	firstHTTPRouteFile, err := os.Open(HTTP_ROUTE_BASIC_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", HTTP_ROUTE_BASIC_PATH, err)
		t.Error()
		return ctx
	}
	defer firstHTTPRouteFile.Close()
	logrus.Infof("file %q was opened", HTTP_ROUTE_BASIC_PATH)
	rolloutFile, err := os.Open(HTTP_ROUTE_BASIC_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", HTTP_ROUTE_BASIC_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", HTTP_ROUTE_BASIC_ROLLOUT_PATH)
	err = decoder.Decode(firstHTTPRouteFile, &httpRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_BASIC_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", HTTP_ROUTE_BASIC_PATH)
	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", HTTP_ROUTE_BASIC_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", HTTP_ROUTE_BASIC_ROLLOUT_PATH)
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

func testSingleHTTPRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
	logrus.Infof("waiting for httpRoute %q to update canary weight to %d after rollout image change", resourcesMap[HTTP_ROUTE_KEY].GetName(), LAST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[HTTP_ROUTE_KEY],
			getMatchHTTPRouteFetcher(t, LAST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("httpRoute %q updating failed: %s", resourcesMap[HTTP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("httpRoute %q was updated", resourcesMap[HTTP_ROUTE_KEY].GetName())
	return ctx
}

func teardownSingleHTTPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
