package plugin

import (
	"context"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHandleExperimentUsesDefaultMaxTrafficWeightWhenUnset(t *testing.T) {
	rollout := experimentRollout(0, "active-experiment")
	rollout.Spec.Strategy.Canary.TrafficRouting.MaxTrafficWeight = nil
	httpRoute := experimentHTTPRoute(
		backendRef("stable-svc", 100),
		backendRef("canary-svc", 0),
		backendRef("exp-svc-1", 25),
		backendRef("exp-svc-2", 30),
	)
	destinations := []v1alpha1.WeightDestination{
		{ServiceName: "exp-svc-1", Weight: 25},
		{ServiceName: "exp-svc-2", Weight: 30},
	}

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	assert.Equal(t, int32(45), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
}

func TestHandleExperimentCleanupPreservesRemainingBackendRefFields(t *testing.T) {
	rollout := experimentRollout(100, "")
	rollout.Status.Canary.Weights = &v1alpha1.TrafficWeights{
		Additional: []v1alpha1.WeightDestination{{ServiceName: "exp-svc", Weight: 15}},
	}
	httpRoute := experimentHTTPRoute(
		backendRefWithPort("stable-svc", 45, 8080),
		backendRefWithPort("canary-svc", 0, 8080),
		backendRefWithPort("exp-svc", 15, 8080),
	)

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 2)
	assert.Equal(t, int32(100), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, gatewayv1.PortNumber(8080), *httpRoute.Spec.Rules[0].BackendRefs[0].Port)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
	assert.Equal(t, gatewayv1.PortNumber(8080), *httpRoute.Spec.Rules[0].BackendRefs[1].Port)
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

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

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

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

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

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

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

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

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

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 3)
	assert.Equal(t, gatewayv1.ObjectName("stable-svc"), httpRoute.Spec.Rules[0].BackendRefs[0].Name)
	assert.Equal(t, int32(100), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Equal(t, gatewayv1.ObjectName("canary-svc"), httpRoute.Spec.Rules[0].BackendRefs[1].Name)
	assert.Equal(t, int32(0), *httpRoute.Spec.Rules[0].BackendRefs[1].Weight)
	assert.Equal(t, gatewayv1.ObjectName("unmanaged-svc"), httpRoute.Spec.Rules[0].BackendRefs[2].Name)
}

// TestHandleExperimentAddsBackendsToAllWeightedRules locks in that experiment
// backends land on every rule SetWeight splits, not just the first one.
func TestHandleExperimentAddsBackendsToAllWeightedRules(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	httpRoute := multiRuleHTTPRoute(
		[]gatewayv1.HTTPBackendRef{backendRef("stable-svc", 80), backendRef("canary-svc", 20)},
		[]gatewayv1.HTTPBackendRef{backendRef("stable-svc", 80), backendRef("canary-svc", 20)},
	)
	destinations := []v1alpha1.WeightDestination{{ServiceName: "exp-svc", Weight: 10}}
	clientset := fake.NewClientset(experimentService("exp-svc", corev1.ServicePort{Name: "http", Port: 8080}))

	err := HandleExperiment(context.Background(), clientset, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	for i := range httpRoute.Spec.Rules {
		require.Len(t, httpRoute.Spec.Rules[i].BackendRefs, 3, "rule %d", i)
		assert.Equal(t, int32(70), *httpRoute.Spec.Rules[i].BackendRefs[0].Weight, "rule %d", i)
		assert.Equal(t, gatewayv1.ObjectName("exp-svc"), httpRoute.Spec.Rules[i].BackendRefs[2].Name, "rule %d", i)
		assert.Equal(t, int32(10), *httpRoute.Spec.Rules[i].BackendRefs[2].Weight, "rule %d", i)
	}
}

// TestHandleExperimentLeavesRuleWithoutBothServicesUntouched locks in the ownership
// rule: only rules holding both the canary and the stable backendRef belong to the
// plugin, so a user-managed rule referencing just the canary is never mutated, even
// when it is ordered first.
func TestHandleExperimentLeavesRuleWithoutBothServicesUntouched(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	httpRoute := multiRuleHTTPRoute(
		[]gatewayv1.HTTPBackendRef{backendRef("canary-svc", 5)},
		[]gatewayv1.HTTPBackendRef{backendRef("stable-svc", 80), backendRef("canary-svc", 20)},
	)
	destinations := []v1alpha1.WeightDestination{{ServiceName: "exp-svc", Weight: 10}}
	clientset := fake.NewClientset(experimentService("exp-svc", corev1.ServicePort{Name: "http", Port: 8080}))

	err := HandleExperiment(context.Background(), clientset, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 1)
	assert.Equal(t, int32(5), *httpRoute.Spec.Rules[0].BackendRefs[0].Weight)
	require.Len(t, httpRoute.Spec.Rules[1].BackendRefs, 3)
	assert.Equal(t, int32(70), *httpRoute.Spec.Rules[1].BackendRefs[0].Weight)
	assert.Equal(t, gatewayv1.ObjectName("exp-svc"), httpRoute.Spec.Rules[1].BackendRefs[2].Name)
}

// TestHandleExperimentPrefersHTTPNamedServicePort covers a service whose "http" port
// is the same number the lookup used to use as its "not found" default.
func TestHandleExperimentPrefersHTTPNamedServicePort(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	httpRoute := experimentHTTPRoute(backendRef("stable-svc", 80), backendRef("canary-svc", 20))
	destinations := []v1alpha1.WeightDestination{{ServiceName: "exp-svc", Weight: 10}}
	clientset := fake.NewClientset(experimentService("exp-svc",
		corev1.ServicePort{Name: "metrics", Port: 9090},
		corev1.ServicePort{Name: "http", Port: 8080},
	))

	err := HandleExperiment(context.Background(), clientset, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 3)
	addedRef := httpRoute.Spec.Rules[0].BackendRefs[2]
	assert.Equal(t, gatewayv1.ObjectName("exp-svc"), addedRef.Name)
	assert.Equal(t, gatewayv1.PortNumber(8080), *addedRef.Port)
	assert.Equal(t, gatewayv1.Namespace("default"), *addedRef.Namespace)
}

func TestHandleExperimentSkipsDestinationWithoutService(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	httpRoute := experimentHTTPRoute(backendRef("stable-svc", 80), backendRef("canary-svc", 20))
	destinations := []v1alpha1.WeightDestination{{ServiceName: "missing-svc", Weight: 10}}

	err := HandleExperiment(context.Background(), fake.NewClientset(), testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, destinations)

	require.NoError(t, err)
	require.Len(t, httpRoute.Spec.Rules[0].BackendRefs, 2)
}

func TestHandleExperimentErrorsWhenNoWeightedRuleExists(t *testing.T) {
	rollout := experimentRollout(100, "active-experiment")
	httpRoute := experimentHTTPRoute(backendRef("other-svc", 100))

	err := HandleExperiment(context.Background(), nil, testLogger(), rollout, "stable-svc", "canary-svc", httpRoute, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching rule found for rollout rollout-test")
}

func TestExperimentServicePort(t *testing.T) {
	tests := []struct {
		name     string
		ports    []corev1.ServicePort
		wantPort gatewayv1.PortNumber
		wantOK   bool
	}{
		{
			name:     "prefers the port named http",
			ports:    []corev1.ServicePort{{Name: "http", Port: 80}},
			wantPort: 80,
			wantOK:   true,
		},
		{
			name:     "falls back to the first port",
			ports:    []corev1.ServicePort{{Name: "grpc", Port: 9000}},
			wantPort: 9000,
			wantOK:   true,
		},
		{
			name:     "falls back for an unnamed single port",
			ports:    []corev1.ServicePort{{Port: 8080}},
			wantPort: 8080,
			wantOK:   true,
		},
		{
			name:     "keeps an http port that is not listed first",
			ports:    []corev1.ServicePort{{Name: "metrics", Port: 9090}, {Name: "http", Port: 8080}},
			wantPort: 8080,
			wantOK:   true,
		},
		{
			name:   "reports no port when the service exposes none",
			ports:  nil,
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			port, ok := experimentServicePort(&corev1.Service{Spec: corev1.ServiceSpec{Ports: test.ports}})

			assert.Equal(t, test.wantOK, ok)
			assert.Equal(t, test.wantPort, port)
		})
	}
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

func multiRuleHTTPRoute(ruleRefs ...[]gatewayv1.HTTPBackendRef) *gatewayv1.HTTPRoute {
	rules := make([]gatewayv1.HTTPRouteRule, 0, len(ruleRefs))
	for _, refs := range ruleRefs {
		rules = append(rules, gatewayv1.HTTPRouteRule{BackendRefs: refs})
	}
	return &gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: rules}}
}

func backendRef(name string, weight int32) gatewayv1.HTTPBackendRef {
	return gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(name)},
			Weight:                 &weight,
		},
	}
}

func backendRefWithPort(name string, weight int32, port gatewayv1.PortNumber) gatewayv1.HTTPBackendRef {
	namespace := gatewayv1.Namespace("default")
	return gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name:      gatewayv1.ObjectName(name),
				Namespace: &namespace,
				Port:      &port,
			},
			Weight: &weight,
		},
	}
}

func experimentService(name string, ports ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: ports},
	}
}

func testLogger() *logrus.Entry {
	return logrus.New().WithField("test", "experiment")
}
