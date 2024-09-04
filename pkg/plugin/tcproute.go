package plugin

import (
	"context"
	"errors"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RpcPlugin) setTCPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	tcpRouteClient := r.TCPRouteClient
	if !r.IsTest {
		gatewayClientV1alpha2 := r.GatewayAPIClientset.GatewayV1alpha2()
		tcpRouteClient = gatewayClientV1alpha2.TCPRoutes(gatewayAPIConfig.Namespace)
	}
	tcpRoute, err := tcpRouteClient.Get(ctx, gatewayAPIConfig.TCPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	routeRuleList := TCPRouteRuleList(tcpRoute.Spec.Rules)
	canaryBackendRefs, err := getBackendRefs(canaryServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	for _, ref := range canaryBackendRefs {
		ref.Weight = &desiredWeight
	}
	stableBackendRefs, err := getBackendRefs(stableServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	for _, ref := range stableBackendRefs {
		ref.Weight = &restWeight
	}
	updatedTCPRoute, err := tcpRouteClient.Update(ctx, tcpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedTCPRouteMock = updatedTCPRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *TCPRouteRule) Iterator() (GatewayAPIRouteRuleIterator[*TCPBackendRef], bool) {
	backendRefList := r.BackendRefs
	index := 0
	next := func() (*TCPBackendRef, bool) {
		if len(backendRefList) == index {
			return nil, false
		}
		backendRef := (*TCPBackendRef)(&backendRefList[index])
		index = index + 1
		return backendRef, len(backendRefList) > index
	}
	return next, len(backendRefList) > index
}

func (r TCPRouteRuleList) Iterator() (GatewayAPIRouteRuleListIterator[*TCPBackendRef, *TCPRouteRule], bool) {
	routeRuleList := r
	index := 0
	next := func() (*TCPRouteRule, bool) {
		if len(routeRuleList) == index {
			return nil, false
		}
		routeRule := (*TCPRouteRule)(&routeRuleList[index])
		index = index + 1
		return routeRule, len(routeRuleList) > index
	}
	return next, len(routeRuleList) > index
}

func (r TCPRouteRuleList) Error() error {
	return errors.New(BackendRefListWasNotFoundInTCPRouteError)
}

func (r *TCPBackendRef) GetName() string {
	return string(r.Name)
}

func (r TCPRoute) GetName() string {
	return r.Name
}
