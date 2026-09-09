package plugin

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/weightutil"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// experimentPortName is the service port the plugin prefers when adding an
// experiment backendRef.
const experimentPortName = "http"

func HandleExperiment(ctx context.Context, clientset kubernetes.Interface, logger *logrus.Entry, rollout *v1alpha1.Rollout, stableService string, canaryService string, httpRoute *gatewayv1.HTTPRoute, additionalDestinations []v1alpha1.WeightDestination) error {
	// Experiment backends belong on the same rules SetWeight splits, i.e. rules
	// holding both the canary and the stable backendRef. A rule referencing only
	// one of them is user-managed and must be left alone.
	weightedRules, err := getAllRouteRules(HTTPRouteRuleList(httpRoute.Spec.Rules), canaryService, stableService)
	if err != nil {
		return fmt.Errorf("no matching rule found for rollout %s: %w", rollout.Name, err)
	}

	if isExperimentActive(rollout, additionalDestinations) {
		logger.Infof("Found active experiment %s", rollout.Status.Canary.CurrentExperiment)
		for _, rule := range weightedRules {
			addExperimentBackends(ctx, clientset, logger, rollout, rule, stableService, canaryService, additionalDestinations)
		}
		return nil
	}

	ownedServices := ownedExperimentServices(rollout)
	for _, rule := range weightedRules {
		removeExperimentBackends(logger, rollout, rule, stableService, canaryService, ownedServices)
	}
	return nil
}

// isExperimentActive reports whether the controller wants experiment destinations
// on this reconcile. CurrentExperiment alone is not enough: it is still set while
// the experiment is being torn down, so the plugin must also have been given
// destinations to add (issue #203).
func isExperimentActive(rollout *v1alpha1.Rollout, additionalDestinations []v1alpha1.WeightDestination) bool {
	return rollout.Spec.Strategy.Canary != nil &&
		rollout.Status.Canary.CurrentExperiment != "" &&
		len(additionalDestinations) > 0
}

// ownedExperimentServices are the experiment services the controller told the
// plugin to add on the previous reconcile, recorded in the rollout status. These
// are the only backends the plugin owns and may remove; any other backend in the
// route is managed externally and must be left untouched (issue #203).
func ownedExperimentServices(rollout *v1alpha1.Rollout) map[string]bool {
	owned := make(map[string]bool)
	if rollout.Status.Canary.Weights == nil {
		return owned
	}
	for _, dest := range rollout.Status.Canary.Weights.Additional {
		owned[dest.ServiceName] = true
	}
	return owned
}

// addExperimentBackends re-points stable at the traffic left over by canary and the
// experiment, then appends a backendRef for every destination not already present.
func addExperimentBackends(ctx context.Context, clientset kubernetes.Interface, logger *logrus.Entry, rollout *v1alpha1.Rollout, rule *HTTPRouteRule, stableService string, canaryService string, additionalDestinations []v1alpha1.WeightDestination) {
	setBackendWeight(rule, stableService, stableWeightForExperiment(logger, rollout, rule, canaryService, additionalDestinations))

	for _, dest := range additionalDestinations {
		if findBackendRef(rule, dest.ServiceName) != nil {
			continue
		}
		backendRef, ok := experimentBackendRef(ctx, clientset, logger, rollout.Namespace, dest)
		if !ok {
			continue
		}
		logger.Infof("Adding experiment service to HTTPRoute: %s with weight %d", dest.ServiceName, dest.Weight)
		rule.BackendRefs = append(rule.BackendRefs, backendRef)
	}
}

// stableWeightForExperiment preserves the canary allocation established by
// SetWeight. Experiment destinations consume traffic in addition to canary, so
// only the remainder belongs to stable.
func stableWeightForExperiment(logger *logrus.Entry, rollout *v1alpha1.Rollout, rule *HTTPRouteRule, canaryService string, additionalDestinations []v1alpha1.WeightDestination) int32 {
	var canaryWeight int32
	if canaryRef := findBackendRef(rule, canaryService); canaryRef != nil && canaryRef.Weight != nil {
		canaryWeight = *canaryRef.Weight
	}

	var totalExperimentWeight int64
	for _, dest := range additionalDestinations {
		totalExperimentWeight += int64(dest.Weight)
	}

	maxWeight := weightutil.MaxTrafficWeight(rollout)
	totalAllocatedWeight := int64(canaryWeight) + totalExperimentWeight
	if totalAllocatedWeight > int64(maxWeight) {
		logger.Warnf("Combined canary and experiment weight exceeds maxTrafficWeight %d (got %d), setting stable weight to 0", maxWeight, totalAllocatedWeight)
		return 0
	}
	return maxWeight - int32(totalAllocatedWeight)
}

// removeExperimentBackends drops the backends the plugin added for a finished
// experiment and hands their traffic back to stable. Weights are only reset if
// something was actually removed, so a route the plugin never touched is left as is.
func removeExperimentBackends(logger *logrus.Entry, rollout *v1alpha1.Rollout, rule *HTTPRouteRule, stableService string, canaryService string, ownedServices map[string]bool) {
	if !hasOwnedBackend(rule, ownedServices) {
		return
	}
	logger.Info("Experiment is no longer active, removing experiment services from HTTPRoute")

	keptBackendRefs := make([]gatewayv1.HTTPBackendRef, 0, len(rule.BackendRefs))
	for _, backendRef := range rule.BackendRefs {
		if ownedServices[string(backendRef.Name)] {
			logger.Infof("Removing experiment service from HTTPRoute: %s", backendRef.Name)
			continue
		}
		keptBackendRefs = append(keptBackendRefs, backendRef)
	}
	rule.BackendRefs = keptBackendRefs

	setBackendWeight(rule, stableService, weightutil.MaxTrafficWeight(rollout))
	setBackendWeight(rule, canaryService, 0)
	logger.Info("Experiment services removed from HTTPRoute")
}

// hasOwnedBackend reports whether the rule still carries a backend the plugin
// added for an experiment, i.e. whether there is anything to clean up.
func hasOwnedBackend(rule *HTTPRouteRule, ownedServices map[string]bool) bool {
	for _, backendRef := range rule.BackendRefs {
		if ownedServices[string(backendRef.Name)] {
			return true
		}
	}
	return false
}

// experimentBackendRef builds the backendRef for an experiment destination, or
// reports false if the service cannot be resolved.
func experimentBackendRef(ctx context.Context, clientset kubernetes.Interface, logger *logrus.Entry, namespace string, dest v1alpha1.WeightDestination) (gatewayv1.HTTPBackendRef, bool) {
	service, err := clientset.CoreV1().Services(namespace).Get(ctx, dest.ServiceName, metav1.GetOptions{})
	if err != nil {
		logger.Warnf("Failed to get service %s: %v", dest.ServiceName, err)
		return gatewayv1.HTTPBackendRef{}, false
	}

	port, ok := experimentServicePort(service)
	if !ok {
		logger.Warnf("Service %s exposes no ports, skipping experiment backendRef", dest.ServiceName)
		return gatewayv1.HTTPBackendRef{}, false
	}

	backendNamespace := gatewayv1.Namespace(namespace)
	weight := dest.Weight
	return gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name:      gatewayv1.ObjectName(dest.ServiceName),
				Namespace: &backendNamespace,
				Port:      &port,
			},
			Weight: &weight,
		},
	}, true
}

// experimentServicePort prefers the port named "http" and falls back to the
// service's first port.
func experimentServicePort(service *corev1.Service) (gatewayv1.PortNumber, bool) {
	for _, servicePort := range service.Spec.Ports {
		if servicePort.Name == experimentPortName {
			return servicePort.Port, true
		}
	}
	if len(service.Spec.Ports) > 0 {
		return service.Spec.Ports[0].Port, true
	}
	return 0, false
}

// findBackendRef returns the backendRef for serviceName within a single rule, or
// nil when the rule does not reference it.
func findBackendRef(rule *HTTPRouteRule, serviceName string) *gatewayv1.HTTPBackendRef {
	for i := range rule.BackendRefs {
		if string(rule.BackendRefs[i].Name) == serviceName {
			return &rule.BackendRefs[i]
		}
	}
	return nil
}

func setBackendWeight(rule *HTTPRouteRule, serviceName string, weight int32) {
	if backendRef := findBackendRef(rule, serviceName); backendRef != nil {
		backendRef.Weight = &weight
	}
}
