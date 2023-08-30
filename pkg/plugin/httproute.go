package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HttpRouteClient
	if !r.IsTest {
		gatewayV1beta1 := r.Client.GatewayV1beta1()
		httpRouteClient = gatewayV1beta1.HTTPRoutes(gatewayAPIConfig.Namespace)
	}
	httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	routeRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	backendRefList, err := getBackendRefList[HTTPBackendRefList](routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef, err := getBackendRef[*HTTPBackendRef](canaryServiceName, backendRefList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getBackendRef[*HTTPBackendRef](stableServiceName, backendRefList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	stableBackendRef.Weight = &restWeight
	updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedHTTPRouteMock = updatedHTTPRoute
	}
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	}
	return pluginTypes.RpcError{}
}

func (r HTTPRouteRuleList) Iterator() (GatewayAPIRouteRuleIterator[HTTPBackendRefList], bool) {
	ruleList := r
	index := 0
	next := func() (HTTPBackendRefList, bool) {
		if len(ruleList) == index {
			return nil, false
		}
		backendRefList := HTTPBackendRefList(ruleList[index].BackendRefs)
		index = index + 1
		return backendRefList, len(ruleList) > index
	}
	return next, len(ruleList) != index
}

func (r HTTPRouteRuleList) Error() error {
	return errors.New("backendRefs was not found in httpRoute")
}

func (r HTTPBackendRefList) Iterator() (GatewayAPIBackendRefIterator[*HTTPBackendRef], bool) {
	backendRefList := r
	index := 0
	next := func() (*HTTPBackendRef, bool) {
		if len(backendRefList) == index {
			return nil, false
		}
		backendRef := (*HTTPBackendRef)(&backendRefList[index])
		index = index + 1
		return backendRef, len(backendRefList) > index
	}
	return next, len(backendRefList) > index
}

func (r HTTPBackendRefList) Error() error {
	return errors.New("backendRef was not found in httpRoute")
}

func (r *HTTPBackendRef) GetName() string {
	return string(r.Name)
}
