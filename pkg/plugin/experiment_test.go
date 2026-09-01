package plugin

import (
	"context"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHandleExperiment_ExperimentStatusChecking(t *testing.T) {
	stableService := "stable-svc"
	canaryService := "canary-svc"

	rollout := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout-test",
			Namespace: "default",
		},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: stableService,
					CanaryService: canaryService,
				},
			},
		},
		Status: v1alpha1.RolloutStatus{
			Canary: v1alpha1.CanaryStatus{
				CurrentExperiment: "active-experiment",
			},
		},
	}

	stableWeight := int32(100)
	canaryWeight := int32(0)
	httpRoute := &gatewayv1.HTTPRoute{
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(stableService),
								},
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(canaryService),
								},
								Weight: &canaryWeight,
							},
						},
					},
				},
			},
		},
	}

	isExperimentActive := rollout.Spec.Strategy.Canary != nil && rollout.Status.Canary.CurrentExperiment != ""
	assert.True(t, isExperimentActive, "Experiment should be detected as active")

	hasExperimentServices := false
	ruleIdx := 0
	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		serviceName := string(backendRef.Name)
		if serviceName != stableService && serviceName != canaryService {
			hasExperimentServices = true
			break
		}
	}
	assert.False(t, hasExperimentServices, "HTTPRoute should not have experiment services initially")

	// Test dynamic weight calculation
	additionalDestinations := []v1alpha1.WeightDestination{
		{
			ServiceName: "exp-svc-1",
			Weight:      25,
		},
		{
			ServiceName: "exp-svc-2",
			Weight:      30,
		},
	}

	// Calculate total experiment weight
	var totalExperimentWeight int32
	for _, dest := range additionalDestinations {
		totalExperimentWeight += dest.Weight
	}
	expectedStableWeight := int32(100) - totalExperimentWeight

	// Update stable weight
	for i, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		if string(backendRef.Name) == stableService {
			httpRoute.Spec.Rules[ruleIdx].BackendRefs[i].Weight = &expectedStableWeight
			break
		}
	}

	assert.Equal(t, int32(45), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight, "Stable weight should be 45% (100% - 55% experiment weight)")
}

func TestHandleExperiment_RemoveExperimentServices(t *testing.T) {
	stableService := "stable-svc"
	canaryService := "canary-svc"
	experimentSvc := "exp-svc"

	rollout := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout-test",
			Namespace: "default",
		},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: stableService,
					CanaryService: canaryService,
				},
			},
		},
		Status: v1alpha1.RolloutStatus{
			Canary: v1alpha1.CanaryStatus{
				CurrentExperiment: "",
			},
		},
	}

	stableWeight := int32(45)
	canaryWeight := int32(0)
	experimentWeight := int32(15)
	port := gatewayv1.PortNumber(8080)
	namespace := gatewayv1.Namespace("default")

	httpRoute := &gatewayv1.HTTPRoute{
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(stableService),
									Port: &port,
								},
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(canaryService),
									Port: &port,
								},
								Weight: &canaryWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name:      gatewayv1.ObjectName(experimentSvc),
									Namespace: &namespace,
									Port:      &port,
								},
								Weight: &experimentWeight,
							},
						},
					},
				},
			},
		},
	}

	isExperimentActive := rollout.Spec.Strategy.Canary != nil && rollout.Status.Canary.CurrentExperiment != ""
	assert.False(t, isExperimentActive, "Experiment should be detected as inactive")

	hasExperimentServices := false
	ruleIdx := 0
	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		serviceName := string(backendRef.Name)
		if serviceName != stableService && serviceName != canaryService {
			hasExperimentServices = true
			break
		}
	}
	assert.True(t, hasExperimentServices, "HTTPRoute should have experiment services initially")

	stableWeight = int32(100)
	canaryWeight = int32(0)
	filteredBackendRefs := []gatewayv1.HTTPBackendRef{}

	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		serviceName := string(backendRef.Name)

		switch serviceName {
		case stableService:
			backendRef.Weight = &stableWeight
			filteredBackendRefs = append(filteredBackendRefs, backendRef)
		case canaryService:
			backendRef.Weight = &canaryWeight
			filteredBackendRefs = append(filteredBackendRefs, backendRef)
		}
	}

	httpRoute.Spec.Rules[ruleIdx].BackendRefs = filteredBackendRefs

	assert.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 2, "Should only have stable and canary services after cleanup")
	assert.Equal(t, int32(100), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight, "Stable weight should be reset to 100%")
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight, "Canary weight should remain at 0%")

	hasExperimentServices = false
	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		serviceName := string(backendRef.Name)
		if serviceName != stableService && serviceName != canaryService {
			hasExperimentServices = true
			break
		}
	}
	assert.False(t, hasExperimentServices, "HTTPRoute should not have experiment services after cleanup")
}

func TestHandleExperimentUsesMaxTrafficWeight(t *testing.T) {
	const maxWeight = int32(100000)
	rollout := experimentRollout(maxWeight, "active-experiment")
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 100000),
		backendRef("canary-svc", 0),
		backendRef("exp-svc-1", 20000),
		backendRef("exp-svc-2", 30000),
	)
	destinations := []v1alpha1.WeightDestination{
		{ServiceName: "exp-svc-1", Weight: 20000},
		{ServiceName: "exp-svc-2", Weight: 30000},
	}

	err := HandleExperiment(context.Background(), nil, nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	assert.Equal(t, int32(50000), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
}

func TestHandleExperimentPreservesCanaryWeight(t *testing.T) {
	const maxWeight = int32(100)
	rollout := experimentRollout(maxWeight, "active-experiment")
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 80),
		backendRef("canary-svc", 20),
		backendRef("exp-svc-1", 10),
		backendRef("exp-svc-2", 10),
	)
	destinations := []v1alpha1.WeightDestination{
		{ServiceName: "exp-svc-1", Weight: 10},
		{ServiceName: "exp-svc-2", Weight: 10},
	}

	err := HandleExperiment(context.Background(), nil, nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	assert.Equal(t, int32(60), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, int32(20), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
	assert.Equal(t, int32(10), *httpRoute.Spec.Rules[0].BackendRefs[2].Weight)
	assert.Equal(t, int32(10), *httpRoute.Spec.Rules[0].BackendRefs[3].Weight)
}

func TestHandleExperimentFloorsStableWeightWhenExperimentWeightsExceedMaxTrafficWeight(t *testing.T) {
	const maxWeight = int32(100000)
	rollout := experimentRollout(maxWeight, "active-experiment")
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 100000),
		backendRef("canary-svc", 0),
		backendRef("exp-svc-1", 60000),
		backendRef("exp-svc-2", 50000),
	)
	destinations := []v1alpha1.WeightDestination{
		{ServiceName: "exp-svc-1", Weight: 60000},
		{ServiceName: "exp-svc-2", Weight: 50000},
	}

	err := HandleExperiment(context.Background(), nil, nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 4)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
	assert.Equal(t, int32(60000), *httpRoute.Spec.Rules[0].BackendRefs[2].Weight)
	assert.Equal(t, int32(50000), *httpRoute.Spec.Rules[0].BackendRefs[3].Weight)
}

func TestHandleExperimentCleanupRestoresMaxTrafficWeight(t *testing.T) {
	const maxWeight = int32(100000)
	rollout := experimentRollout(maxWeight, "")
	rollout.Status.Canary.Weights = &v1alpha1.TrafficWeights{
		Additional: []v1alpha1.WeightDestination{{ServiceName: "exp-svc", Weight: 30000}},
	}
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 70000),
		backendRef("canary-svc", 0),
		backendRef("exp-svc", 30000),
	)

	err := HandleExperiment(context.Background(), nil, nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 2)
	assert.Equal(t, int32(100000), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
}

func TestHandleExperimentCleansUpBeforeCurrentExperimentIsCleared(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	rollout.Status.Canary.Weights = &v1alpha1.TrafficWeights{
		Additional: []v1alpha1.WeightDestination{
			{ServiceName: "exp-svc-1", Weight: 10},
			{ServiceName: "exp-svc-2", Weight: 10},
		},
	}
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 60),
		backendRef("canary-svc", 40),
		backendRef("exp-svc-1", 10),
		backendRef("unmanaged-svc", 5),
		backendRef("exp-svc-2", 10),
	)

	err := HandleExperiment(context.Background(), nil, nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 3)
	assert.Equal(t, gatewayv1.ObjectName("stable-svc"), httpRoute.Spec.Rules[0].BackendRefs[0].Name)
	assert.Equal(t, int32(100), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, gatewayv1.ObjectName("canary-svc"), httpRoute.Spec.Rules[0].BackendRefs[1].Name)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
	assert.Equal(t, gatewayv1.ObjectName("unmanaged-svc"), httpRoute.Spec.Rules[0].BackendRefs[2].Name)
}

func experimentRollout(maxWeight int32, currentExperiment string) *v1alpha1.Rollout {
	return &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "rollout-test", Namespace: "default"},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: "stable-svc",
					CanaryService: "canary-svc",
					TrafficRouting: &v1alpha1.RolloutTrafficRouting{
						MaxTrafficWeight: &maxWeight,
					},
				},
			},
		},
		Status: v1alpha1.RolloutStatus{
			Canary: v1alpha1.CanaryStatus{CurrentExperiment: currentExperiment},
		},
	}
}

func experimentHTTPRoute(refs ...gatewayv1.HTTPBackendRef) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{BackendRefs: refs}},
		},
	}
}

func backendRef(name string, weight int32) gatewayv1.HTTPBackendRef {
	return gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(name)},
			Weight:                 &weight,
		},
	}
}

func testLogger() *logrus.Entry {
	return logrus.New().WithField("test", "experiment")
}
