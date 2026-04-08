package plugin

import (
	"context"
	"errors"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (r *RpcPlugin) setGRPCRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	grpcRouteClient := r.GRPCRouteClient
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		grpcRouteClient = gatewayClientv1.GRPCRoutes(gatewayAPIConfig.Namespace)
	}
	grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
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
	for i := range grpcRoute.Spec.Rules {
		// Skip plugin-injected header-routing rules.
		// Primary: rule carries a Name matching a known managed route.
		// Fallback: structural check (single canary-only BackendRef) for rules injected
		// by older plugin versions that did not set the Name field.
		rule := grpcRoute.Spec.Rules[i]
		if (rule.Name != nil && managedNames[string(*rule.Name)]) || isGRPCManagedRule(rule, canaryServiceObjName, nil) {
			continue
		}
		for j := range grpcRoute.Spec.Rules[i].BackendRefs {
			switch string(grpcRoute.Spec.Rules[i].BackendRefs[j].Name) {
			case canaryServiceName:
				grpcRoute.Spec.Rules[i].BackendRefs[j].Weight = &desiredWeight
				canaryFound = true
			case stableServiceName:
				grpcRoute.Spec.Rules[i].BackendRefs[j].Weight = &restWeight
				stableFound = true
			}
		}
	}
	if !canaryFound || !stableFound {
		return pluginTypes.RpcError{
			ErrorString: BackendRefWasNotFoundInGRPCRouteError,
		}
	}
	ensureInProgressLabel(grpcRoute, desiredWeight, gatewayAPIConfig)
	updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedGRPCRouteMock = updatedGRPCRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) setGRPCHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	if headerRouting.Match == nil {
		return r.removeGRPCManagedRoutes(rollout, gatewayAPIConfig)
	}
	ctx := context.TODO()
	grpcRouteClient := r.GRPCRouteClient
	if !r.IsTest {
		gatewayClientV1 := r.GatewayAPIClientset.GatewayV1()
		grpcRouteClient = gatewayClientV1.GRPCRoutes(gatewayAPIConfig.Namespace)
	}
	grpcHeaderRouteRuleList, rpcError := getGRPCHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}
	grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceKind := gatewayv1.Kind("Service")
	canaryServiceGroup := gatewayv1.Group("")
	grpcRouteRuleList := GRPCRouteRuleList(grpcRoute.Spec.Rules)
	backendRefNameList := []string{string(canaryServiceName), stableServiceName}
	grpcRouteRule, err := getRouteRule(grpcRouteRuleList, backendRefNameList...)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	var canaryBackendRef *GRPCBackendRef
	for i := 0; i < len(grpcRouteRule.BackendRefs); i++ {
		backendRef := grpcRouteRule.BackendRefs[i]
		if canaryServiceName == backendRef.Name {
			canaryBackendRef = (*GRPCBackendRef)(&backendRef)
			break
		}
	}
	managedName := gatewayv1.SectionName(headerRouting.Name)
	grpcHeaderRouteRule := gatewayv1.GRPCRouteRule{
		Name:    &managedName,
		Matches: []gatewayv1.GRPCRouteMatch{},
		Filters: []gatewayv1.GRPCRouteFilter{},
		BackendRefs: []gatewayv1.GRPCBackendRef{
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
	if grpcRouteRule.Filters != nil {
		for i := 0; i < len(grpcRouteRule.Filters); i++ {
			grpcHeaderRouteRule.Filters = append(grpcHeaderRouteRule.Filters, *grpcRouteRule.Filters[i].DeepCopy())
		}
	}
	matchLength := len(grpcRouteRule.Matches)
	if matchLength == 0 {
		grpcHeaderRouteRule.Matches = []gatewayv1.GRPCRouteMatch{
			{
				Headers: grpcHeaderRouteRuleList,
			},
		}
	} else {
		// Copy matches from original route and merge headers
		for i := 0; i < matchLength; i++ {
			// Merge existing headers with new canary headers
			mergedHeaders := make([]gatewayv1.GRPCHeaderMatch, 0)
			// First, add existing headers from the original match
			if grpcRouteRule.Matches[i].Headers != nil {
				mergedHeaders = append(mergedHeaders, grpcRouteRule.Matches[i].Headers...)
			}
			// Then, add the new canary headers
			mergedHeaders = append(mergedHeaders, grpcHeaderRouteRuleList...)

			grpcHeaderRouteRule.Matches = append(grpcHeaderRouteRule.Matches, gatewayv1.GRPCRouteMatch{
				Method:  grpcRouteRule.Matches[i].Method,
				Headers: mergedHeaders,
			})
		}
	}

	// Upsert: find an existing managed rule to replace in-place.
	// Primary: match by rule Name (set by this plugin on injection).
	// Fallback: structural check for rules injected by older plugin versions without a Name.
	foundIndex := -1
	for i, rule := range grpcRouteRuleList {
		if (rule.Name != nil && *rule.Name == managedName) || isGRPCManagedRule(rule, canaryServiceName, grpcHeaderRouteRuleList) {
			foundIndex = i
			break
		}
	}
	if foundIndex >= 0 {
		grpcRouteRuleList[foundIndex] = grpcHeaderRouteRule
	} else {
		grpcRouteRuleList = append(grpcRouteRuleList, grpcHeaderRouteRule)
	}
	grpcRoute.Spec.Rules = grpcRouteRuleList
	updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedGRPCRouteMock = updatedGRPCRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

// isGRPCManagedRule reports whether the given rule was injected by this plugin.
// A plugin-injected rule always has exactly one BackendRef pointing to the canary service.
// If canaryHeaders is non-nil, the rule must also have at least one match whose header list
// contains all of the specified canary header names — this distinguishes between multiple
// managed routes that each inject rules with different header sets.
func isGRPCManagedRule(rule gatewayv1.GRPCRouteRule, canaryService gatewayv1.ObjectName, canaryHeaders []gatewayv1.GRPCHeaderMatch) bool {
	if len(rule.BackendRefs) != 1 || rule.BackendRefs[0].Name != canaryService {
		return false
	}
	if canaryHeaders == nil {
		return true
	}
	for _, match := range rule.Matches {
		if grpcMatchContainsAllCanaryHeaders(match.Headers, canaryHeaders) {
			return true
		}
	}
	return false
}

// grpcMatchContainsAllCanaryHeaders returns true if matchHeaders contains a header entry
// for every name present in canaryHeaders.
func grpcMatchContainsAllCanaryHeaders(matchHeaders []gatewayv1.GRPCHeaderMatch, canaryHeaders []gatewayv1.GRPCHeaderMatch) bool {
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

func getGRPCHeaderRouteRuleList(headerRouting *v1alpha1.SetHeaderRoute) ([]gatewayv1.GRPCHeaderMatch, pluginTypes.RpcError) {
	grpcHeaderRouteRuleList := []gatewayv1.GRPCHeaderMatch{}
	for _, headerRule := range headerRouting.Match {
		grpcHeaderRouteRule := gatewayv1.GRPCHeaderMatch{
			Name: gatewayv1.GRPCHeaderName(headerRule.HeaderName),
		}
		switch {
		case headerRule.HeaderValue.Exact != "":
			headerMatchType := gatewayv1.GRPCHeaderMatchExact
			grpcHeaderRouteRule.Type = &headerMatchType
			grpcHeaderRouteRule.Value = headerRule.HeaderValue.Exact
		case headerRule.HeaderValue.Prefix != "":
			headerMatchType := gatewayv1.GRPCHeaderMatchRegularExpression
			grpcHeaderRouteRule.Type = &headerMatchType
			grpcHeaderRouteRule.Value = headerRule.HeaderValue.Prefix + ".*"
		case headerRule.HeaderValue.Regex != "":
			headerMatchType := gatewayv1.GRPCHeaderMatchRegularExpression
			grpcHeaderRouteRule.Type = &headerMatchType
			grpcHeaderRouteRule.Value = headerRule.HeaderValue.Regex
		default:
			return nil, pluginTypes.RpcError{
				ErrorString: InvalidHeaderMatchTypeError,
			}
		}
		grpcHeaderRouteRuleList = append(grpcHeaderRouteRuleList, grpcHeaderRouteRule)
	}
	return grpcHeaderRouteRuleList, pluginTypes.RpcError{}
}

func (r *RpcPlugin) removeGRPCManagedRoutes(rollout *v1alpha1.Rollout, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	grpcRouteClient := r.GRPCRouteClient
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		grpcRouteClient = gatewayClientv1.GRPCRoutes(gatewayAPIConfig.Namespace)
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	managedNames := managedRouteNamesSet(rollout)
	grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	newRules := make([]gatewayv1.GRPCRouteRule, 0, len(grpcRoute.Spec.Rules))
	changed := false
	for _, rule := range grpcRoute.Spec.Rules {
		// Primary: remove by Name. Fallback: structural check for unnamed legacy rules.
		if (rule.Name != nil && managedNames[string(*rule.Name)]) || isGRPCManagedRule(rule, canaryServiceName, nil) {
			changed = true
			continue
		}
		newRules = append(newRules, rule)
	}
	if !changed {
		return pluginTypes.RpcError{}
	}
	grpcRoute.Spec.Rules = newRules
	updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedGRPCRouteMock = updatedGRPCRoute
	}
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *GRPCRouteRule) Iterator() (GatewayAPIRouteRuleIterator[*GRPCBackendRef], bool) {
	backendRefList := r.BackendRefs
	index := 0
	next := func() (*GRPCBackendRef, bool) {
		if len(backendRefList) == index {
			return nil, false
		}
		backendRef := (*GRPCBackendRef)(&backendRefList[index])
		index = index + 1
		return backendRef, len(backendRefList) > index
	}
	return next, len(backendRefList) > index
}

func (r GRPCRouteRuleList) Iterator() (GatewayAPIRouteRuleListIterator[*GRPCBackendRef, *GRPCRouteRule], bool) {
	routeRuleList := r
	index := 0
	next := func() (*GRPCRouteRule, bool) {
		if len(routeRuleList) == index {
			return nil, false
		}
		routeRule := (*GRPCRouteRule)(&routeRuleList[index])
		index++
		return routeRule, len(routeRuleList) > index
	}
	return next, len(routeRuleList) != index
}

func (r GRPCRouteRuleList) Error() error {
	return errors.New(BackendRefWasNotFoundInGRPCRouteError)
}

func (r *GRPCBackendRef) GetName() string {
	return string(r.Name)
}

func (r GRPCRoute) GetName() string {
	return r.Name
}
