package plugin

import (
	"context"
	"errors"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
	if !r.IsTest {
		gatewayClientV1 := r.GatewayAPIClientset.GatewayV1()
		httpRouteClient = gatewayClientV1.HTTPRoutes(gatewayAPIConfig.Namespace)
	}
	httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceObjName := gatewayv1.ObjectName(canaryServiceName)
	restWeight := 100 - desiredWeight
	managedNames := managedRouteNamesSet(rollout)
	canaryFound, stableFound := false, false
	for i := range httpRoute.Spec.Rules {
		// Skip plugin-injected header-routing rules.
		// Primary: rule carries a Name matching a known managed route.
		// Fallback: structural check (single canary-only BackendRef) for rules injected
		// by older plugin versions that did not set the Name field.
		rule := httpRoute.Spec.Rules[i]
		if (rule.Name != nil && managedNames[string(*rule.Name)]) || isHTTPManagedRule(rule, canaryServiceObjName, nil) {
			continue
		}
		for j := range httpRoute.Spec.Rules[i].BackendRefs {
			switch string(httpRoute.Spec.Rules[i].BackendRefs[j].Name) {
			case canaryServiceName:
				httpRoute.Spec.Rules[i].BackendRefs[j].Weight = &desiredWeight
				canaryFound = true
			case stableServiceName:
				httpRoute.Spec.Rules[i].BackendRefs[j].Weight = &restWeight
				stableFound = true
			}
		}
	}
	if !canaryFound || !stableFound {
		return pluginTypes.RpcError{
			ErrorString: BackendRefWasNotFoundInHTTPRouteError,
		}
	}
	err = HandleExperiment(ctx, r.Clientset, r.GatewayAPIClientset, r.LogCtx, rollout, httpRoute, additionalDestinations)
	if err != nil {
		r.LogCtx.Error(err, "Failed to handle experiment services")
	}
	ensureInProgressLabel(httpRoute, desiredWeight, gatewayAPIConfig)
	updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedHTTPRouteMock = updatedHTTPRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) setHTTPHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	if headerRouting.Match == nil {
		return r.removeHTTPManagedRoutes(rollout, gatewayAPIConfig)
	}
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		httpRouteClient = gatewayClientv1.HTTPRoutes(gatewayAPIConfig.Namespace)
	}
	httpHeaderRouteRuleList, rpcError := getHTTPHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}
	httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceKind := gatewayv1.Kind("Service")
	canaryServiceGroup := gatewayv1.Group("")
	httpRouteRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	backendRefNameList := []string{string(canaryServiceName), stableServiceName}
	httpRouteRule, err := getRouteRule(httpRouteRuleList, backendRefNameList...)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	var canaryBackendRef *HTTPBackendRef
	for i := 0; i < len(httpRouteRule.BackendRefs); i++ {
		backendRef := httpRouteRule.BackendRefs[i]
		if canaryServiceName == backendRef.Name {
			canaryBackendRef = (*HTTPBackendRef)(&backendRef)
			break
		}
	}
	managedName := gatewayv1.SectionName(headerRouting.Name)
	httpHeaderRouteRule := gatewayv1.HTTPRouteRule{
		Name:    &managedName,
		Matches: []gatewayv1.HTTPRouteMatch{},
		Filters: []gatewayv1.HTTPRouteFilter{},
		BackendRefs: []gatewayv1.HTTPBackendRef{
			{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Group: &canaryServiceGroup,
						Kind:  &canaryServiceKind,
						Name:  canaryServiceName,
						Port:  canaryBackendRef.Port,
					},
				},
			},
		},
	}

	// Copy filters from original route
	if httpRouteRule.Filters != nil {
		for i := 0; i < len(httpRouteRule.Filters); i++ {
			httpHeaderRouteRule.Filters = append(httpHeaderRouteRule.Filters, *httpRouteRule.Filters[i].DeepCopy())
		}
	}

	// Copy matches from original route and merge headers
	if len(httpRouteRule.Matches) == 0 {
		// Original rule has no matches - create a match with just the canary headers
		httpHeaderRouteRule.Matches = []gatewayv1.HTTPRouteMatch{
			{
				Headers: httpHeaderRouteRuleList,
			},
		}
	} else {
		// Copy existing matches and merge headers
		for i := 0; i < len(httpRouteRule.Matches); i++ {
			// Merge existing headers with new canary headers
			mergedHeaders := make([]gatewayv1.HTTPHeaderMatch, 0)
			// First, add existing headers from the original match
			if httpRouteRule.Matches[i].Headers != nil {
				mergedHeaders = append(mergedHeaders, httpRouteRule.Matches[i].Headers...)
			}
			// Then, add the new canary headers
			mergedHeaders = append(mergedHeaders, httpHeaderRouteRuleList...)

			httpHeaderRouteRule.Matches = append(httpHeaderRouteRule.Matches, gatewayv1.HTTPRouteMatch{
				Path:        httpRouteRule.Matches[i].Path,
				Headers:     mergedHeaders,
				QueryParams: httpRouteRule.Matches[i].QueryParams,
				Method:      httpRouteRule.Matches[i].Method,
			})
		}
	}

	// Upsert: find an existing managed rule to replace in-place.
	// Primary: match by rule Name (set by this plugin on injection).
	// Fallback: structural check for rules injected by older plugin versions without a Name.
	foundIndex := -1
	for i, rule := range httpRouteRuleList {
		if (rule.Name != nil && *rule.Name == managedName) || isHTTPManagedRule(rule, canaryServiceName, httpHeaderRouteRuleList) {
			foundIndex = i
			break
		}
	}
	if foundIndex >= 0 {
		httpRouteRuleList[foundIndex] = httpHeaderRouteRule
	} else {
		httpRouteRuleList = append(httpRouteRuleList, httpHeaderRouteRule)
	}
	httpRoute.Spec.Rules = httpRouteRuleList
	updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedHTTPRouteMock = updatedHTTPRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

// isHTTPManagedRule reports whether the given rule was injected by this plugin.
// A plugin-injected rule always has exactly one BackendRef pointing to the canary service.
// If canaryHeaders is non-nil, the rule must also have at least one match whose header list
// contains all of the specified canary header names — this distinguishes between multiple
// managed routes that each inject rules with different header sets.
func isHTTPManagedRule(rule gatewayv1.HTTPRouteRule, canaryService gatewayv1.ObjectName, canaryHeaders []gatewayv1.HTTPHeaderMatch) bool {
	if len(rule.BackendRefs) != 1 || rule.BackendRefs[0].Name != canaryService {
		return false
	}
	if canaryHeaders == nil {
		return true
	}
	for _, match := range rule.Matches {
		if httpMatchContainsAllCanaryHeaders(match.Headers, canaryHeaders) {
			return true
		}
	}
	return false
}

// httpMatchContainsAllCanaryHeaders returns true if matchHeaders contains a header entry
// for every name present in canaryHeaders.
func httpMatchContainsAllCanaryHeaders(matchHeaders []gatewayv1.HTTPHeaderMatch, canaryHeaders []gatewayv1.HTTPHeaderMatch) bool {
	for _, canary := range canaryHeaders {
		found := false
		for _, h := range matchHeaders {
			if h.Name == canary.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func getHTTPHeaderRouteRuleList(headerRouting *v1alpha1.SetHeaderRoute) ([]gatewayv1.HTTPHeaderMatch, pluginTypes.RpcError) {
	httpHeaderRouteRuleList := []gatewayv1.HTTPHeaderMatch{}
	for _, headerRule := range headerRouting.Match {
		httpHeaderRouteRule := gatewayv1.HTTPHeaderMatch{
			Name: gatewayv1.HTTPHeaderName(headerRule.HeaderName),
		}
		switch {
		case headerRule.HeaderValue.Exact != "":
			headerMatchType := gatewayv1.HeaderMatchExact
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Exact
		case headerRule.HeaderValue.Prefix != "":
			headerMatchType := gatewayv1.HeaderMatchRegularExpression
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Prefix + ".*"
		case headerRule.HeaderValue.Regex != "":
			headerMatchType := gatewayv1.HeaderMatchRegularExpression
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Regex
		default:
			return nil, pluginTypes.RpcError{
				ErrorString: InvalidHeaderMatchTypeError,
			}
		}
		httpHeaderRouteRuleList = append(httpHeaderRouteRuleList, httpHeaderRouteRule)
	}
	return httpHeaderRouteRuleList, pluginTypes.RpcError{}
}

func (r *RpcPlugin) removeHTTPManagedRoutes(rollout *v1alpha1.Rollout, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		httpRouteClient = gatewayClientv1.HTTPRoutes(gatewayAPIConfig.Namespace)
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	managedNames := managedRouteNamesSet(rollout)
	httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	newRules := make([]gatewayv1.HTTPRouteRule, 0, len(httpRoute.Spec.Rules))
	changed := false
	for _, rule := range httpRoute.Spec.Rules {
		// Primary: remove by Name. Fallback: structural check for unnamed legacy rules.
		if (rule.Name != nil && managedNames[string(*rule.Name)]) || isHTTPManagedRule(rule, canaryServiceName, nil) {
			changed = true
			continue
		}
		newRules = append(newRules, rule)
	}
	if !changed {
		return pluginTypes.RpcError{}
	}
	httpRoute.Spec.Rules = newRules
	updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedHTTPRouteMock = updatedHTTPRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *HTTPRouteRule) Iterator() (GatewayAPIRouteRuleIterator[*HTTPBackendRef], bool) {
	backendRefList := r.BackendRefs
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

func (r HTTPRouteRuleList) Iterator() (GatewayAPIRouteRuleListIterator[*HTTPBackendRef, *HTTPRouteRule], bool) {
	routeRuleList := r
	index := 0
	next := func() (*HTTPRouteRule, bool) {
		if len(routeRuleList) == index {
			return nil, false
		}
		routeRule := (*HTTPRouteRule)(&routeRuleList[index])
		index++
		return routeRule, len(routeRuleList) > index
	}
	return next, len(routeRuleList) != index
}

func (r HTTPRouteRuleList) Error() error {
	return errors.New(BackendRefWasNotFoundInHTTPRouteError)
}

func (r *HTTPBackendRef) GetName() string {
	return string(r.Name)
}

func (r HTTPRoute) GetName() string {
	return r.Name
}
