package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (r *RpcPlugin) setGRPCRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	grpcRouteClient := r.GatewayAPIClientset.GatewayV1().GRPCRoutes(gatewayAPIConfig.Namespace)

	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	restWeight := 100 - desiredWeight
	managedNames := managedRouteNamesSet(rollout)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}

		canaryFound, stableFound := false, false
		for i := range grpcRoute.Spec.Rules {
			// Skip plugin-injected header-routing rules: rule carries a Name matching a known managed route.
			rule := grpcRoute.Spec.Rules[i]
			if rule.Name != nil && isManagedRuleName(string(*rule.Name), managedNames) {
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
			return errors.New(BackendRefWasNotFoundInGRPCRouteError)
		}

		ensureInProgressLabel(grpcRoute, desiredWeight, gatewayAPIConfig)

		_, err = grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
		return err
	})

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
	grpcRouteClient := r.GatewayAPIClientset.GatewayV1().GRPCRoutes(gatewayAPIConfig.Namespace)
	grpcHeaderRouteRuleList, rpcError := getGRPCHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}

	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	managedName := gatewayv1.SectionName(headerRouting.Name)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}

		canaryServiceKind := gatewayv1.Kind("Service")
		canaryServiceGroup := gatewayv1.Group("")
		grpcRouteRuleList := GRPCRouteRuleList(grpcRoute.Spec.Rules)
		backendRefNameList := []string{string(canaryServiceName), stableServiceName}
		sourceRules, err := getAllRouteRules(grpcRouteRuleList, backendRefNameList...)
		if err != nil {
			return err
		}

		// Build one managed header rule per source rule so that the canary header
		// applies to every rule on a multi-rule GRPCRoute (issue #207).
		// Each rule needs a unique name within the route (Gateway API constraint).
		// Index 0 keeps the bare managedName for backward compatibility with single-rule routes;
		// subsequent rules are named managedName-1, managedName-2, etc.
		newManagedRules := make([]gatewayv1.GRPCRouteRule, 0, len(sourceRules))
		for idx, grpcRouteRule := range sourceRules {
			var canaryBackendRef *GRPCBackendRef
			for i := 0; i < len(grpcRouteRule.BackendRefs); i++ {
				backendRef := grpcRouteRule.BackendRefs[i]
				if canaryServiceName == backendRef.Name {
					canaryBackendRef = (*GRPCBackendRef)(&backendRef)
					break
				}
			}
			ruleName := managedName
			if idx > 0 {
				ruleName = gatewayv1.SectionName(fmt.Sprintf("%s-%d", managedName, idx))
			}
			grpcHeaderRouteRule := gatewayv1.GRPCRouteRule{
				Name:    &ruleName,
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

			// Copy matches from original route and merge headers
			if len(grpcRouteRule.Matches) == 0 {
				grpcHeaderRouteRule.Matches = []gatewayv1.GRPCRouteMatch{
					{Headers: grpcHeaderRouteRuleList},
				}
			} else {
				for i := range len(grpcRouteRule.Matches) {
					mergedHeaders := make([]gatewayv1.GRPCHeaderMatch, 0)
					if grpcRouteRule.Matches[i].Headers != nil {
						mergedHeaders = append(mergedHeaders, grpcRouteRule.Matches[i].Headers...)
					}
					mergedHeaders = append(mergedHeaders, grpcHeaderRouteRuleList...)
					grpcHeaderRouteRule.Matches = append(grpcHeaderRouteRule.Matches, gatewayv1.GRPCRouteMatch{
						Method:  grpcRouteRule.Matches[i].Method,
						Headers: mergedHeaders,
					})
				}
			}

			newManagedRules = append(newManagedRules, grpcHeaderRouteRule)
		}

		// Upsert: remove all existing managed rules for this name, then append the new set.
		// Match by rule Name only, so routes sharing a header name but with a different
		// managed route Name are left untouched.
		cleanedRules := make(GRPCRouteRuleList, 0, len(grpcRouteRuleList))
		for _, rule := range grpcRouteRuleList {
			if rule.Name != nil && isManagedRuleName(string(*rule.Name), map[string]bool{string(managedName): true}) {
				continue
			}
			cleanedRules = append(cleanedRules, rule)
		}
		grpcRoute.Spec.Rules = append(cleanedRules, newManagedRules...)

		_, err = grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
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
	grpcRouteClient := r.GatewayAPIClientset.GatewayV1().GRPCRoutes(gatewayAPIConfig.Namespace)

	managedNames := managedRouteNamesSet(rollout)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		grpcRoute, err := grpcRouteClient.Get(ctx, gatewayAPIConfig.GRPCRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}

		newRules := make([]gatewayv1.GRPCRouteRule, 0, len(grpcRoute.Spec.Rules))
		changed := false
		for _, rule := range grpcRoute.Spec.Rules {
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
		grpcRoute.Spec.Rules = newRules

		_, err = grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
		return err
	})

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
