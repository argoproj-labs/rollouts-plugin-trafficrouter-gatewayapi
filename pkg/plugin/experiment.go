package plugin

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	hclog "github.com/hashicorp/go-hclog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

func HandleExperiment(ctx context.Context, clientset *kubernetes.Clientset, gatewayClient gatewayApiClientset.Interface, logger hclog.Logger, rollout *v1alpha1.Rollout, httpRoute *gatewayv1.HTTPRoute, additionalDestinations []v1alpha1.WeightDestination) error {
	ruleIdx := -1
	stableService := rollout.Spec.Strategy.Canary.StableService
	canaryService := rollout.Spec.Strategy.Canary.CanaryService

	for i, rule := range httpRoute.Spec.Rules {
		if ruleIdx != -1 {
			break
		}
		for _, backendRef := range rule.BackendRefs {
			if string(backendRef.Name) == stableService || string(backendRef.Name) == canaryService {
				ruleIdx = i
				break
			}
		}
	}

	if ruleIdx == -1 {
		return fmt.Errorf("no matching rule found for rollout %s", rollout.Name)
	}

	isExperimentActive := rollout.Spec.Strategy.Canary != nil && rollout.Status.Canary.CurrentExperiment != ""

	// previousServices are the experiment services the controller told the plugin to add
	// on the previous reconcile, recorded in the rollout status. These are the only
	// backends the plugin owns and may remove; any other backend in the route is managed
	// externally and must be left untouched (issue #203).
	previousServices := make(map[string]bool)
	if rollout.Status.Canary.Weights != nil {
		for _, dest := range rollout.Status.Canary.Weights.Additional {
			previousServices[dest.ServiceName] = true
		}
	}

	hasExperimentServices := false
	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		if previousServices[string(backendRef.Name)] {
			hasExperimentServices = true
			break
		}
	}

	if isExperimentActive {
		logger.Info("Found active experiment", "experiment", rollout.Status.Canary.CurrentExperiment)

		if len(additionalDestinations) == 0 {
			logger.Info("No experiment services found in additionalDestinations, skipping experiment service addition")
			return nil
		}

		// Compute total experiment weight
		var totalExperimentWeight int32
		for _, dest := range additionalDestinations {
			totalExperimentWeight += dest.Weight
		}

		// Sanity cap: don't allow overflow
		if totalExperimentWeight > 100 {
			logger.Warn("Total experiment weight exceeds 100, capping at 100", "weight", totalExperimentWeight)
			totalExperimentWeight = 100
		}

		stableWeight := int32(100) - totalExperimentWeight

		for i, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
			if string(backendRef.Name) == stableService {
				httpRoute.Spec.Rules[ruleIdx].BackendRefs[i].Weight = &stableWeight
				break
			}
		}

		for _, additionalDest := range additionalDestinations {
			serviceName := additionalDest.ServiceName
			weight := additionalDest.Weight

			exists := false
			for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
				if string(backendRef.Name) == serviceName {
					exists = true
					break
				}
			}

			if !exists {
				logger.Info("Adding experiment service to HTTPRoute", "service", serviceName, "weight", weight)

				service, err := clientset.CoreV1().Services(rollout.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
				if err != nil {
					logger.Warn("Failed to get service", "service", serviceName, "error", err)
					continue
				}

				port := gatewayv1.PortNumber(8080)
				portName := "http"
				for _, servicePort := range service.Spec.Ports {
					if servicePort.Name == portName {
						port = servicePort.Port
						break
					}
				}

				if len(service.Spec.Ports) > 0 && port == 8080 {
					port = service.Spec.Ports[0].Port
				}

				namespace := gatewayv1.Namespace(rollout.Namespace)
				httpRoute.Spec.Rules[ruleIdx].BackendRefs = append(httpRoute.Spec.Rules[ruleIdx].BackendRefs, gatewayv1.HTTPBackendRef{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      gatewayv1.ObjectName(serviceName),
							Namespace: &namespace,
							Port:      &port,
						},
						Weight: &weight,
					},
				})
			}
		}
		return nil
	}

	if !isExperimentActive && hasExperimentServices {
		logger.Info("Experiment is no longer active, removing experiment services from HTTPRoute")

		stableWeight := int32(100)
		filteredBackendRefs := []gatewayv1.HTTPBackendRef{}

		for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
			serviceName := string(backendRef.Name)

			if previousServices[serviceName] {
				logger.Info("Removing experiment service from HTTPRoute", "service", serviceName)
				continue
			}

			switch serviceName {
			case stableService:
				backendRef.Weight = &stableWeight
			case canaryService:
				zeroWeight := int32(0)
				backendRef.Weight = &zeroWeight
			}
			filteredBackendRefs = append(filteredBackendRefs, backendRef)
		}

		httpRoute.Spec.Rules[ruleIdx].BackendRefs = filteredBackendRefs
		logger.Info("Experiment services removed from HTTPRoute")
	}

	return nil
}
