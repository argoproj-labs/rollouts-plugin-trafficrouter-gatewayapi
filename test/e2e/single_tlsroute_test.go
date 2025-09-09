//go:build !flaky

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
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestSingleTLSRoute(t *testing.T) {
	feature := features.New("Single TLSRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupSingleTLSRouteEnv,
	).Assess(
		"Testing single TLSRoute feature",
		testSingleTLSRoute,
	).Teardown(
		teardownSingleTLSRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupSingleTLSRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var tlsRoute v1alpha2.TLSRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)
	firstTLSRouteFile, err := os.Open(TLS_ROUTE_BASIC_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", TLS_ROUTE_BASIC_PATH, err)
		t.Error()
		return ctx
	}
	defer firstTLSRouteFile.Close()
	logrus.Infof("file %q was opened", TLS_ROUTE_BASIC_PATH)
	rolloutFile, err := os.Open(TLS_ROUTE_BASIC_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", TLS_ROUTE_BASIC_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", TLS_ROUTE_BASIC_ROLLOUT_PATH)
	err = decoder.Decode(firstTLSRouteFile, &tlsRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", TLS_ROUTE_BASIC_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", TLS_ROUTE_BASIC_PATH)
	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", TLS_ROUTE_BASIC_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", TLS_ROUTE_BASIC_ROLLOUT_PATH)
	tlsRouteObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tlsRoute)
	if err != nil {
		logrus.Errorf("tlsRoute %q converting to unstructured was failed: %s", tlsRoute.GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tlsRoute %q was converted to unstructured", tlsRoute.GetName())
	resourcesMap[TLS_ROUTE_KEY] = &unstructured.Unstructured{
		Object: tlsRouteObject,
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
	err = clusterResources.Create(ctx, resourcesMap[TLS_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("tlsRoute %q creation was failed: %s", resourcesMap[TLS_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tlsRoute %q was created", resourcesMap[TLS_ROUTE_KEY].GetName())
	err = clusterResources.Create(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("rollout %q creation was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was created", resourcesMap[ROLLOUT_KEY].GetName())
	waitCondition := conditions.New(clusterResources)
	logrus.Infof("waiting for tlsRoute %q to connect with rollout %q (expecting canary weight: %d)", resourcesMap[TLS_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), FIRST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[TLS_ROUTE_KEY],
			getMatchTLSRouteFetcher(t, FIRST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(MEDIUM_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("checking tlsRoute %q connection with rollout %q was failed: %s", resourcesMap[TLS_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tlsRoute %q connected with rollout %q", resourcesMap[TLS_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName())
	return ctx
}

func testSingleTLSRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
	logrus.Infof("waiting for tlsRoute %q to update canary weight to %d after rollout image change", resourcesMap[TLS_ROUTE_KEY].GetName(), LAST_CANARY_ROUTE_WEIGHT)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[TLS_ROUTE_KEY],
			getMatchTLSRouteFetcher(t, LAST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("tlsRoute %q updating failed: %s", resourcesMap[TLS_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tlsRoute %q was updated", resourcesMap[TLS_ROUTE_KEY].GetName())
	return ctx
}

func teardownSingleTLSRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
	err = clusterResources.Delete(ctx, resourcesMap[TLS_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("deleting tlsRoute %q was failed: %s", resourcesMap[TLS_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tlsRoute %q was deleted", resourcesMap[TLS_ROUTE_KEY].GetName())
	return ctx
}

func getMatchTLSRouteFetcher(t *testing.T, targetWeight int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var tlsRoute v1alpha2.TLSRoute
		unstructuredTLSRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		// logrus.Info("k8s object was type asserted")
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredTLSRoute.Object, &tlsRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured tlsRoute %q to the typed tlsRoute was failed", unstructuredTLSRoute.GetName())
			t.Error()
			return false
		}
		// logrus.Infof("unstructured tlsRoute %q was converted to the typed tlsRoute", tlsRoute.GetName())
		return *tlsRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}
