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

func TestSingleGRPCRoute(t *testing.T) {
	feature := features.New("Single GRPCRoute feature").Setup(
		setupEnvironment,
	).Setup(
		setupSingleGRPCRouteEnv,
	).Assess(
		"Testing single GRPCRoute feature",
		testSingleGRPCRoute,
	).Teardown(
		teardownSingleGRPCRouteEnv,
	).Feature()
	_ = global.Test(t, feature)
}

func setupSingleGRPCRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	var grpcRoute v1alpha2.GRPCRoute
	var rollout v1alpha1.Rollout
	clusterResources := config.Client().Resources()
	resourcesMap := map[string]*unstructured.Unstructured{}
	ctx = context.WithValue(ctx, RESOURCES_MAP_KEY, resourcesMap)
	firstGRPCRouteFile, err := os.Open(FIRST_GRPC_ROUTE_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", FIRST_GRPC_ROUTE_PATH, err)
		t.Error()
		return ctx
	}
	defer firstGRPCRouteFile.Close()
	logrus.Infof("file %q was opened", FIRST_GRPC_ROUTE_PATH)
	rolloutFile, err := os.Open(SINGLE_GRPC_ROUTE_ROLLOUT_PATH)
	if err != nil {
		logrus.Errorf("file %q openning was failed: %s", SINGLE_GRPC_ROUTE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	defer rolloutFile.Close()
	logrus.Infof("file %q was opened", SINGLE_GRPC_ROUTE_ROLLOUT_PATH)
	err = decoder.Decode(firstGRPCRouteFile, &grpcRoute)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", FIRST_GRPC_ROUTE_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", FIRST_GRPC_ROUTE_PATH)
	err = decoder.Decode(rolloutFile, &rollout)
	if err != nil {
		logrus.Errorf("file %q decoding was failed: %s", SINGLE_GRPC_ROUTE_ROLLOUT_PATH, err)
		t.Error()
		return ctx
	}
	logrus.Infof("file %q was decoded", SINGLE_GRPC_ROUTE_ROLLOUT_PATH)
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

func testSingleGRPCRoute(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
			resourcesMap[GRPC_ROUTE_KEY],
			getMatchGRPCRouteFetcher(t, LAST_CANARY_ROUTE_WEIGHT),
		),
		wait.WithTimeout(LONG_PERIOD),
		wait.WithInterval(SHORT_PERIOD),
	)
	if err != nil {
		logrus.Errorf("grpcRoute %q updation was failed: %s", resourcesMap[GRPC_ROUTE_KEY].GetName(), err)
		t.Error()
		return ctx
	}
	logrus.Infof("grpcRoute %q was updated", resourcesMap[GRPC_ROUTE_KEY].GetName())
	return ctx
}

func teardownSingleGRPCRouteEnv(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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

func getMatchGRPCRouteFetcher(t *testing.T, targetWeight int32) func(k8s.Object) bool {
	return func(obj k8s.Object) bool {
		var grpcRoute v1alpha2.GRPCRoute
		unstructuredGRPCRoute, ok := obj.(*unstructured.Unstructured)
		if !ok {
			logrus.Error("k8s object type assertion was failed")
			t.Error()
			return false
		}
		logrus.Info("k8s object was type asserted")
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredGRPCRoute.Object, &grpcRoute)
		if err != nil {
			logrus.Errorf("conversation from unstructured grpcRoute %q to the typed grpcRoute was failed", unstructuredGRPCRoute.GetName())
			t.Error()
			return false
		}
		logrus.Infof("unstructured grpcRoute %q was converted to the typed grpcRoute", grpcRoute.GetName())
		return *grpcRoute.Spec.Rules[ROLLOUT_ROUTE_RULE_INDEX].BackendRefs[CANARY_BACKEND_REF_INDEX].Weight == targetWeight
	}
}
