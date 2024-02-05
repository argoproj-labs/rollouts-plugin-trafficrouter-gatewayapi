package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RpcPlugin) setTCPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting, routeName string) pluginTypes.RpcError {
	ctx := context.TODO()
	tcpRouteClient := r.TCPRouteClient
	if !r.IsTest {
		gatewayV1alpha2 := r.Client.GatewayV1alpha2()
		tcpRouteClient = gatewayV1alpha2.TCPRoutes(gatewayAPIConfig.Namespace)
	}
	tcpRoute, err := tcpRouteClient.Get(ctx, routeName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	routeRuleList := TCPRouteRuleList(tcpRoute.Spec.Rules)
	backendRefList, err := getBackendRefList[TCPBackendRefList](routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef, err := getBackendRef[*TCPBackendRef](canaryServiceName, backendRefList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getBackendRef[*TCPBackendRef](stableServiceName, backendRefList)
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

func (r TCPRouteRuleList) Iterator() (GatewayAPIRouteRuleIterator[TCPBackendRefList], bool) {
	ruleList := r
	index := 0
	next := func() (TCPBackendRefList, bool) {
		if len(ruleList) == index {
			return nil, false
		}
		backendRefList := TCPBackendRefList(ruleList[index].BackendRefs)
		index = index + 1
		return backendRefList, len(ruleList) > index
	}
	return next, len(ruleList) > index
}

func (r TCPRouteRuleList) Error() error {
	return errors.New("backendRefs was not found in tcpRoute")
}

func (r TCPBackendRefList) Iterator() (GatewayAPIBackendRefIterator[*TCPBackendRef], bool) {
	backendRefList := r
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

func (r TCPBackendRefList) Error() error {
	return errors.New("backendRef was not found in tcpRoute")
}

func (r *TCPBackendRef) GetName() string {
	return string(r.Name)
}
