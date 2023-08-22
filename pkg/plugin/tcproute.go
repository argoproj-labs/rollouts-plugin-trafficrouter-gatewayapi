package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

// TODO: Refactor
// Think about generics
func (r *RpcPlugin) setTCPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	tcpRouteClient := r.TcpRouteClient
	if !r.IsTest {
		gatewayV1alpha2 := r.Client.GatewayV1alpha2()
		tcpRouteClient = gatewayV1alpha2.TCPRoutes(gatewayAPIConfig.Namespace)
	}
	tcpRoute, err := tcpRouteClient.Get(ctx, gatewayAPIConfig.TCPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	rules := tcpRoute.Spec.Rules
	backendRefs, err := getTCPRouteBackendRefList(rules)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef, err := getTCPRouteBackendRef(canaryServiceName, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getTCPRouteBackendRef(stableServiceName, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	stableBackendRef.Weight = &restWeight
	updatedTCPRoute, err := tcpRouteClient.Update(ctx, tcpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedTCPRouteMock = updatedTCPRoute
	}
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", tcpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	}
	return pluginTypes.RpcError{}
}

func getTCPRouteBackendRefList(rules []v1alpha2.TCPRouteRule) ([]v1beta1.BackendRef, error) {
	for _, rule := range rules {
		if rule.BackendRefs == nil {
			continue
		}
		backendRefs := rule.BackendRefs
		return backendRefs, nil
	}
	return nil, errors.New("backendRefs was not found in tcpRoute")
}

func getTCPRouteBackendRef(serviceName string, backendRefs []v1beta1.BackendRef) (*v1beta1.BackendRef, error) {
	var selectedService *v1beta1.BackendRef
	for i := 0; i < len(backendRefs); i++ {
		service := &backendRefs[i]
		nameOfCurrentService := string(service.Name)
		if nameOfCurrentService == serviceName {
			selectedService = service
			break
		}
	}
	if selectedService == nil {
		return nil, errors.New("service was not found in tcpRoute")
	}
	return selectedService, nil
}
