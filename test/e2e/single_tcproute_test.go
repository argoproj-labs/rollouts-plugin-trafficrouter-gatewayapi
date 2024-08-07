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

func TestSingleTCPRoute(t *testing.T) {
	feature := features.New("Single TCPRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupSingleTCPRouteEnv,
	).Assess(
		"Testing single TCPRoute feature",
		testSingleTCPRoute,
	).Teardown(
		teardownSingleTCPRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupSingleTCPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var tcpRoute v1alpha2.TCPRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)
	firstTCPRouteFile, err := os.Open(FIRST_TCP_ROUTE_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", FIRST_TCP_ROUTE_PATH, err)
		t.Error()
		return ctx
	}
	defer firstTCPRouteFile.Close()
	logrus.Infof("file %q was opened", FIRST_TCP_ROUTE_PATH)
	rolloutFile, err := os.Open(SINGLE_TCP_ROUTE_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", SINGLE_TCP_ROUTE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", SINGLE_TCP_ROUTE_ROLLOUT_PATH)
	err = decoder.Decode(firstTCPRouteFile, &tcpRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", FIRST_TCP_ROUTE_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", FIRST_TCP_ROUTE_PATH)
	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", SINGLE_TCP_ROUTE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", SINGLE_TCP_ROUTE_ROLLOUT_PATH)
	tcpRouteObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tcpRoute)
	if err != nil {
		logrus.Errorf("tcpRoute %q converting to unstructured was failed: %s", tcpRoute.GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tcpRoute %q was converted to unstructured", tcpRoute.GetName())
	resourcesMap[TCP_ROUTE_KEY] = &unstructured.Unstructured{
		Object: tcpRouteObject,
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
	err = clusterResources.Create(ctx, resourcesMap[TCP_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("tcpRoute %q creation was failed: %s", resourcesMap[TCP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tcpRoute %q was created", resourcesMap[TCP_ROUTE_KEY].GetName())
	err = clusterResources.Create(ctx, resourcesMap[ROLLOUT_KEY])
	if err != nil {
		logrus.Errorf("rollout %q creation was failed: %s", resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("rollout %q was created", resourcesMap[ROLLOUT_KEY].GetName())
	waitCondition := conditions.New(clusterResources)
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[TCP_ROUTE_KEY],
			getMatchTCPRouteFetcher(t, FIRST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(MEDIUM_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("checking tcpRoute %q connection with rollout %q was failed: %s", resourcesMap[TCP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tcpRoute %q connected with rollout %q", resourcesMap[TCP_ROUTE_KEY].GetName(), resourcesMap[ROLLOUT_KEY].GetName())
	return ctx
}

func testSingleTCPRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
	err = wait.For(
		waitCondition.ResourceMatch(
			resourcesMap[TCP_ROUTE_KEY],
			getMatchTCPRouteFetcher(t, LAST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("tcpRoute %q updation was failed: %s", resourcesMap[TCP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tcpRoute %q was updated", resourcesMap[TCP_ROUTE_KEY].GetName())
	return ctx
}

func teardownSingleTCPRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
	err = clusterResources.Delete(ctx, resourcesMap[TCP_ROUTE_KEY])
	if err != nil {
		logrus.Errorf("deleting tcpRoute %q was failed: %s", resourcesMap[TCP_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("tcpRoute %q was deleted", resourcesMap[TCP_ROUTE_KEY].GetName())
	return ctx
}

func getMatchTCPRouteFetcher(t *testing.T, targetWeight int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var tcpRoute v1alpha2.TCPRoute
		unstructuredTCPRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		logrus.Info("k8s object was type asserted")
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredTCPRoute.Object, &tcpRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured tcpRoute %q to the typed tcpRoute was failed", unstructuredTCPRoute.GetName())
			t.Error()
			return false
		}
		logrus.Infof("unstructured tcpRoute %q was converted to the typed tcpRoute", tcpRoute.GetName())
		return *tcpRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}
