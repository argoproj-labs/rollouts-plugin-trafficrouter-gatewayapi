package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/rollout/trafficrouting"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/argoproj/argo-rollouts/utils/weightutil"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.GatewayAPIClientset.GatewayV1().HTTPRoutes(gatewayAPIConfig.Namespace)

	stableServiceName, canaryServiceName := trafficrouting.GetStableAndCanaryServices(rollout, true)
	restWeight := weightutil.MaxTrafficWeight(rollout) - desiredWeight

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}
		originalSpec := httpRoute.Spec.DeepCopy()

		// Only rules containing BOTH the canary and stable BackendRefs are weight-split
		// rules under this plugin's control. A rule referencing just one of them (e.g. a
		// user-defined rule that happens to route solely to the canary service) is left
		// untouched, regardless of its Name.
		weightedRules, err := getAllRouteRules(HTTPRouteRuleList(httpRoute.Spec.Rules), canaryServiceName, stableServiceName)
		if err != nil {
			return err
		}
		for _, rule := range weightedRules {
			for j := range rule.BackendRefs {
				switch string(rule.BackendRefs[j].Name) {
				case canaryServiceName:
					rule.BackendRefs[j].Weight = &desiredWeight
				case stableServiceName:
					rule.BackendRefs[j].Weight = &restWeight
				}
			}
		}

		err = HandleExperiment(ctx, r.Clientset, r.GatewayAPIClientset, r.LogCtx, rollout, stableServiceName, canaryServiceName, httpRoute, additionalDestinations)
		if err != nil {
			r.LogCtx.Error(err, "Failed to handle experiment services")
		}

		labelModified := ensureInProgressLabel(httpRoute, desiredWeight, gatewayAPIConfig)

		if apiequality.Semantic.DeepEqual(originalSpec, &httpRoute.Spec) && !labelModified {
			return nil
		}

		_, err = httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
		return err
	})

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
	httpRouteClient := r.GatewayAPIClientset.GatewayV1().HTTPRoutes(gatewayAPIConfig.Namespace)
	httpHeaderRouteRuleList, rpcError := getHTTPHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}

	stableServiceName, canaryService := trafficrouting.GetStableAndCanaryServices(rollout, true)
	canaryServiceName := gatewayv1.ObjectName(canaryService)
	managedName := gatewayv1.SectionName(headerRouting.Name)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}

		canaryServiceKind := gatewayv1.Kind("Service")
		canaryServiceGroup := gatewayv1.Group("")
		httpRouteRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
		backendRefNameList := []string{string(canaryServiceName), stableServiceName}
		sourceRules, err := getAllRouteRules(httpRouteRuleList, backendRefNameList...)
		if err != nil {
			return err
		}

		// Build one managed header rule per source rule so that the canary header
		// applies to every rule on a multi-rule HTTPRoute (issue #207).
		// Each rule needs a unique name within the route (Gateway API constraint).
		// Index 0 keeps the bare managedName for backward compatibility with single-rule routes;
		// subsequent rules are named managedName-1, managedName-2, etc.
		newManagedRules := make([]gatewayv1.HTTPRouteRule, 0, len(sourceRules))
		for idx, httpRouteRule := range sourceRules {
			var canaryBackendRef *HTTPBackendRef
			for i := 0; i < len(httpRouteRule.BackendRefs); i++ {
				backendRef := httpRouteRule.BackendRefs[i]
				if canaryServiceName == backendRef.Name {
					canaryBackendRef = (*HTTPBackendRef)(&backendRef)
					break
				}
			}
			ruleName := managedName
			if idx > 0 {
				ruleName = gatewayv1.SectionName(fmt.Sprintf("%s-%d", managedName, idx))
			}
			httpHeaderRouteRule := gatewayv1.HTTPRouteRule{
				Name:    &ruleName,
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
				httpHeaderRouteRule.Matches = []gatewayv1.HTTPRouteMatch{
					{Headers: httpHeaderRouteRuleList},
				}
			} else {
				for i := 0; i < len(httpRouteRule.Matches); i++ {
					mergedHeaders := make([]gatewayv1.HTTPHeaderMatch, 0)
					if httpRouteRule.Matches[i].Headers != nil {
						mergedHeaders = append(mergedHeaders, httpRouteRule.Matches[i].Headers...)
					}
					mergedHeaders = append(mergedHeaders, httpHeaderRouteRuleList...)
					httpHeaderRouteRule.Matches = append(httpHeaderRouteRule.Matches, gatewayv1.HTTPRouteMatch{
						Path:        httpRouteRule.Matches[i].Path,
						Headers:     mergedHeaders,
						QueryParams: httpRouteRule.Matches[i].QueryParams,
						Method:      httpRouteRule.Matches[i].Method,
					})
				}
			}

			newManagedRules = append(newManagedRules, httpHeaderRouteRule)
		}

		// Upsert: remove all existing managed rules for this name, then append the new set.
		// Match by rule Name only, so routes sharing a header name but with a different
		// managed route Name are left untouched.
		cleanedRules := make(HTTPRouteRuleList, 0, len(httpRouteRuleList))
		for _, rule := range httpRouteRuleList {
			if rule.Name != nil && isManagedRuleName(string(*rule.Name), map[string]bool{string(managedName): true}) {
				continue
			}
			cleanedRules = append(cleanedRules, rule)
		}
		httpRoute.Spec.Rules = append(cleanedRules, newManagedRules...)

		_, err = httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
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
	httpRouteClient := r.GatewayAPIClientset.GatewayV1().HTTPRoutes(gatewayAPIConfig.Namespace)

	managedNames := managedRouteNamesSet(rollout)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		httpRoute, err := httpRouteClient.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}

		newRules := make([]gatewayv1.HTTPRouteRule, 0, len(httpRoute.Spec.Rules))
		changed := false
		for _, rule := range httpRoute.Spec.Rules {
			// Remove by Name.
			if rule.Name != nil && isManagedRuleName(string(*rule.Name), managedNames) {
				changed = true
				continue
			}
			newRules = append(newRules, rule)
		}
		if !changed {
			return nil
		}
		httpRoute.Spec.Rules = newRules

		_, err = httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
		return err
	})

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
