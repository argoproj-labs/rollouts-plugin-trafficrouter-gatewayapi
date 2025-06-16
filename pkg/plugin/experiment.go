package plugin

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

func HandleExperiment(ctx context.Context, clientset *kubernetes.Clientset, gatewayClient *gatewayApiClientset.Clientset, logger *logrus.Entry, rollout *v1alpha1.Rollout, httpRoute *gatewayv1.HTTPRoute, additionalDestinations []v1alpha1.WeightDestination) error {
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

	hasExperimentServices := false
	for _, backendRef := range httpRoute.Spec.Rules[ruleIdx].BackendRefs {
		serviceName := string(backendRef.Name)
		if serviceName != stableService && serviceName != canaryService {
			hasExperimentServices = true
			break
		}
	}

	if isExperimentActive {
		logger.Info(fmt.Sprintf("Found active experiment %s", rollout.Status.Canary.CurrentExperiment))

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
			logger.Warnf("Total experiment weight exceeds 100 (got %d), capping at 100", totalExperimentWeight)
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
				logger.Info(fmt.Sprintf("Adding experiment service to HTTPRoute: %s with weight %d", serviceName, weight))

				service, err := clientset.CoreV1().Services(rollout.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
				if err != nil {
					logger.Warn(fmt.Sprintf("Failed to get service %s: %v", serviceName, err))
					continue
				}

				port := gatewayv1.PortNumber(8080)
				portName := "http"
				for _, servicePort := range service.Spec.Ports {
					if servicePort.Name == portName {
						port = gatewayv1.PortNumber(servicePort.Port)
						break
					}
				}

				if len(service.Spec.Ports) > 0 && port == 8080 {
					port = gatewayv1.PortNumber(service.Spec.Ports[0].Port)
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

			if serviceName == stableService {
				backendRef.Weight = &stableWeight
				filteredBackendRefs = append(filteredBackendRefs, backendRef)
			} else if serviceName == canaryService {
				zeroWeight := int32(0)
				backendRef.Weight = &zeroWeight
				filteredBackendRefs = append(filteredBackendRefs, backendRef)
			} else {
				logger.Info(fmt.Sprintf("Removing experiment service from HTTPRoute: %s", serviceName))
			}
		}

		httpRoute.Spec.Rules[ruleIdx].BackendRefs = filteredBackendRefs
		logger.Info("Experiment services removed from HTTPRoute")
	}

	return nil
}
