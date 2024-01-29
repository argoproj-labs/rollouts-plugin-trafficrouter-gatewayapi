package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	httpHeaderRoute = HTTPHeaderRoute{
		mu:                        sync.Mutex{},
		httpHeaderManagedRouteMap: make(map[string]int),
		httpHeaderRouteRule: v1beta1.HTTPRouteRule{
			Matches:     []v1beta1.HTTPRouteMatch{},
			BackendRefs: []v1beta1.HTTPBackendRef{},
		},
	}
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
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

// TODO: Add tests
func (r *RpcPlugin) setHTTPHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	if headerRouting.Match == nil {
		managedRouteList := []v1alpha1.MangedRoutes{
			{
				Name: headerRouting.Name,
			},
		}
		return r.removeHTTPManagedRoutes(managedRouteList, gatewayAPIConfig)
	}
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
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
	canaryServiceName := v1beta1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	canaryServiceKind := v1beta1.Kind("Service")
	canaryServiceGroup := v1beta1.Group("")
	httpHeaderRouteRuleList := []v1beta1.HTTPHeaderMatch{}
	for _, headerRule := range headerRouting.Match {
		httpHeaderRouteRule := v1beta1.HTTPHeaderMatch{
			Name: v1beta1.HTTPHeaderName(headerRule.HeaderName),
		}
		switch {
		case headerRule.HeaderValue.Exact != "":
			headerMatchType := v1beta1.HeaderMatchType(v1beta1.HeaderMatchExact)
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Exact
		case headerRule.HeaderValue.Prefix != "":
			headerMatchType := v1beta1.HeaderMatchType(v1beta1.HeaderMatchRegularExpression)
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Prefix + "*"
		case headerRule.HeaderValue.Regex != "":
			headerMatchType := v1beta1.HeaderMatchType(v1beta1.HeaderMatchRegularExpression)
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Regex
		default:
			return pluginTypes.RpcError{
				ErrorString: "Not found header match type",
			}
		}
		httpHeaderRouteRuleList = append(httpHeaderRouteRuleList, httpHeaderRouteRule)
	}
	routeRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	backendRefList, err := getBackendRefList[HTTPBackendRefList](routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef, err := getBackendRef[*HTTPBackendRef](string(canaryServiceName), backendRefList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpHeaderRouteRule := &httpHeaderRoute.httpHeaderRouteRule
	httpHeaderRouteRule.Matches = []v1beta1.HTTPRouteMatch{
		{
			Path:    routeRuleList[0].Matches[0].Path,
			Headers: httpHeaderRouteRuleList,
		},
	}
	httpHeaderRouteRule.BackendRefs = []v1beta1.HTTPBackendRef{
		{
			BackendRef: v1beta1.BackendRef{
				BackendObjectReference: v1beta1.BackendObjectReference{
					Group: &canaryServiceGroup,
					Kind:  &canaryServiceKind,
					Name:  canaryServiceName,
					Port:  canaryBackendRef.Port,
				},
			},
		},
	}
	routeRuleList = append(routeRuleList, *httpHeaderRouteRule)
	httpRoute.Spec.Rules = routeRuleList
	updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedHTTPRouteMock = updatedHTTPRoute
	}
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	} else {
		httpHeaderRoute.httpHeaderManagedRouteMap[headerRouting.Name] = len(routeRuleList) - 1
	}
	return pluginTypes.RpcError{}
}

// TODO: Add tests
func (r *RpcPlugin) removeHTTPManagedRoutes(managedRouteNameList []v1alpha1.MangedRoutes, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
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
	httpRouteRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	httpHeaderManagedRouteMap := httpHeaderRoute.httpHeaderManagedRouteMap
	for _, managedRoute := range managedRouteNameList {
		managedRouteName := managedRoute.Name
		httpRouteRuleListIndex, isOk := httpHeaderManagedRouteMap[managedRouteName]
		if !isOk {
			r.LogCtx.Logger.Info(fmt.Sprintf("%s is not in httpHeaderManagedRouteMap", managedRouteName))
			return pluginTypes.RpcError{}
		}
		updatedHTTPRouteRuleList := httpRouteRuleList[:httpRouteRuleListIndex]
		if httpRouteRuleListIndex+1 < len(httpRouteRuleList) {
			updatedHTTPRouteRuleList = append(updatedHTTPRouteRuleList, httpRouteRuleList[httpRouteRuleListIndex+1:]...)
		}
		httpRoute.Spec.Rules = updatedHTTPRouteRuleList
		updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
		if r.IsTest {
			r.UpdatedHTTPRouteMock = updatedHTTPRoute
		}
		if err != nil {
			msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
			r.LogCtx.Error(msg)
		} else {
			delete(httpHeaderManagedRouteMap, managedRouteName)
		}
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
