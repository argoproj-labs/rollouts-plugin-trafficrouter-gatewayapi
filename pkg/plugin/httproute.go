package plugin

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	HTTPConfigMapKey = "httpManagedRoutes"
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
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

	// Get experiment services if experiment step is active
	experimentCanaryServiceName := ""
	experimentBaselineServiceName := ""
	experimentWeight := int32(0)

	// Check if this is an experiment step by examining the current step
	if rollout.Spec.Strategy.Canary.Steps != nil && rollout.Status.CurrentStepIndex != nil {
		currentStepIndex := *rollout.Status.CurrentStepIndex
		if currentStepIndex < int32(len(rollout.Spec.Strategy.Canary.Steps)) {
			currentStep := rollout.Spec.Strategy.Canary.Steps[currentStepIndex]
			if currentStep.Experiment != nil {
				// This is an experiment step
				r.LogCtx.Logger.Info("Found experiment step")

				// Check if templates are defined and extract weights from templates
				if currentStep.Experiment.Templates != nil && len(currentStep.Experiment.Templates) > 0 {
					for _, template := range currentStep.Experiment.Templates {
						if template.Name == "experiment-canary" && template.Weight != nil {
							experimentCanaryServiceName = fmt.Sprintf("%s-%s", rollout.Name, template.Name)
							experimentWeight += *template.Weight
							r.LogCtx.Logger.Info(fmt.Sprintf("Found experiment template: %s with weight: %d", template.Name, *template.Weight))
						}
						if template.Name == "experiment-baseline" && template.Weight != nil {
							experimentBaselineServiceName = fmt.Sprintf("%s-%s", rollout.Name, template.Name)
							experimentWeight += *template.Weight
							r.LogCtx.Logger.Info(fmt.Sprintf("Found experiment template: %s with weight: %d", template.Name, *template.Weight))
						}
					}
				}
			}
		}
	}

	routeRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)

	canaryBackendRefs, err := getBackendRefs(canaryServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	stableBackendRefs, err := getBackendRefs(stableServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	// If experiment is active, get experiment backend refs
	var experimentCanaryBackendRefs []*HTTPBackendRef
	var experimentBaselineBackendRefs []*HTTPBackendRef

	if experimentCanaryServiceName != "" {
		experimentCanaryBackendRefs, err = getBackendRefs(experimentCanaryServiceName, routeRuleList)
		if err != nil {
			// If experiment service backend refs not found, we'll need to add them
			r.LogCtx.Logger.Info(fmt.Sprintf("Experiment canary service %s not found in HTTP route, will create it", experimentCanaryServiceName))
		}
	}

	if experimentBaselineServiceName != "" {
		experimentBaselineBackendRefs, err = getBackendRefs(experimentBaselineServiceName, routeRuleList)
		if err != nil {
			// If experiment service backend refs not found, we'll need to add them
			r.LogCtx.Logger.Info(fmt.Sprintf("Experiment baseline service %s not found in HTTP route, will create it", experimentBaselineServiceName))
		}
	}

	// Calculate weights based on whether experiment is active
	stableWeight := int32(100)
	canaryWeight := desiredWeight

	if experimentWeight > 0 {
		// Since we're handling each template with its own weight, we don't need to divide evenly
		experimentCanaryWeight := int32(0)
		experimentBaselineWeight := int32(0)

		// Get individual weights from the templates
		if rollout.Status.CurrentStepIndex != nil {
			currentStepIndex := *rollout.Status.CurrentStepIndex
			if currentStepIndex < int32(len(rollout.Spec.Strategy.Canary.Steps)) {
				currentStep := rollout.Spec.Strategy.Canary.Steps[currentStepIndex]
				if currentStep.Experiment != nil && currentStep.Experiment.Templates != nil {
					for _, template := range currentStep.Experiment.Templates {
						if template.Name == "experiment-canary" && template.Weight != nil {
							experimentCanaryWeight = *template.Weight
						}
						if template.Name == "experiment-baseline" && template.Weight != nil {
							experimentBaselineWeight = *template.Weight
						}
					}
				}
			}
		}

		// Adjust canary weight if needed
		if desiredWeight > experimentWeight {
			canaryWeight = desiredWeight - experimentWeight
		} else {
			canaryWeight = 0
		}

		// Remaining weight goes to stable
		stableWeight = 100 - canaryWeight - experimentWeight

		// Set weights for experiment services
		if len(experimentCanaryBackendRefs) > 0 {
			for _, ref := range experimentCanaryBackendRefs {
				ref.Weight = &experimentCanaryWeight
			}
		} else if experimentCanaryServiceName != "" {
			// Need to add experiment canary service to the route rule
			addExperimentServiceToRoute(httpRoute, experimentCanaryServiceName, experimentCanaryWeight)
		}

		if len(experimentBaselineBackendRefs) > 0 {
			for _, ref := range experimentBaselineBackendRefs {
				ref.Weight = &experimentBaselineWeight
			}
		} else if experimentBaselineServiceName != "" {
			// Need to add experiment baseline service to the route rule
			addExperimentServiceToRoute(httpRoute, experimentBaselineServiceName, experimentBaselineWeight)
		}
	} else {
		// No experiment, use original weight calculation
		stableWeight = 100 - desiredWeight
	}

	// Set weights for canary and stable services
	for _, ref := range canaryBackendRefs {
		ref.Weight = &canaryWeight
	}
	for _, ref := range stableBackendRefs {
		ref.Weight = &stableWeight
	}

	// Update the HTTP route
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

// Helper function to add experiment service to HTTP route
func addExperimentServiceToRoute(httpRoute *gatewayv1.HTTPRoute, serviceName string, weight int32) {
	// Add experiment service as a backend to each rule
	for i := range httpRoute.Spec.Rules {
		serviceKind := gatewayv1.Kind("Service")
		serviceGroup := gatewayv1.Group("")
		port := gatewayv1.PortNumber(80) // Default port, might need adjustment

		// Create new backend reference for experiment service
		experimentBackendRef := gatewayv1.HTTPBackendRef{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Group: &serviceGroup,
					Kind:  &serviceKind,
					Name:  gatewayv1.ObjectName(serviceName),
					Port:  &port,
				},
				Weight: &weight,
			},
		}

		// Add the experiment backend to the rule
		httpRoute.Spec.Rules[i].BackendRefs = append(httpRoute.Spec.Rules[i].BackendRefs, experimentBackendRef)
	}
}

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
	managedRouteMap := make(ManagedRouteMap)
	httpRouteName := gatewayAPIConfig.HTTPRoute
	clientset := r.TestClientset
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		httpRouteClient = gatewayClientv1.HTTPRoutes(gatewayAPIConfig.Namespace)
		clientset = r.Clientset.CoreV1().ConfigMaps(gatewayAPIConfig.Namespace)
	}
	configMap, err := utils.GetOrCreateConfigMap(gatewayAPIConfig.ConfigMap, utils.CreateConfigMapOptions{
		Clientset: clientset,
		Ctx:       ctx,
	})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	err = utils.GetConfigMapData(configMap, HTTPConfigMapKey, &managedRouteMap)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRoute, err := httpRouteClient.Get(ctx, httpRouteName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceKind := gatewayv1.Kind("Service")
	canaryServiceGroup := gatewayv1.Group("")
	httpHeaderRouteRuleList, rpcError := getHTTPHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}
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
	httpHeaderRouteRule := gatewayv1.HTTPRouteRule{
		Matches: []gatewayv1.HTTPRouteMatch{},
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
	for i := 0; i < len(httpRouteRule.Matches); i++ {
		httpHeaderRouteRule.Matches = append(httpHeaderRouteRule.Matches, gatewayv1.HTTPRouteMatch{
			Path:        httpRouteRule.Matches[i].Path,
			Headers:     httpHeaderRouteRuleList,
			QueryParams: httpRouteRule.Matches[i].QueryParams,
		})
	}
	httpRouteRuleList = append(httpRouteRuleList, httpHeaderRouteRule)
	oldHTTPRuleList := httpRoute.Spec.Rules
	httpRoute.Spec.Rules = httpRouteRuleList
	oldConfigMapData := make(ManagedRouteMap)
	err = utils.GetConfigMapData(configMap, HTTPConfigMapKey, &oldConfigMapData)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	taskList := []utils.Task{
		{
			Action: func() error {
				updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedHTTPRouteMock = updatedHTTPRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				httpRoute.Spec.Rules = oldHTTPRuleList
				updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedHTTPRouteMock = updatedHTTPRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			Action: func() error {
				if managedRouteMap[headerRouting.Name] == nil {
					managedRouteMap[headerRouting.Name] = make(map[string]int)
				}
				managedRouteMap[headerRouting.Name][httpRouteName] = len(httpRouteRuleList) - 1
				err = utils.UpdateConfigMapData(configMap, managedRouteMap, utils.UpdateConfigMapOptions{
					Clientset:    clientset,
					ConfigMapKey: HTTPConfigMapKey,
					Ctx:          ctx,
				})
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				err = utils.UpdateConfigMapData(configMap, oldConfigMapData, utils.UpdateConfigMapOptions{
					Clientset:    clientset,
					ConfigMapKey: HTTPConfigMapKey,
					Ctx:          ctx,
				})
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	err = utils.DoTransaction(r.LogCtx, taskList...)
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

func (r *RpcPlugin) removeHTTPManagedRoutes(managedRouteNameList []v1alpha1.MangedRoutes, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
	clientset := r.TestClientset
	httpRouteName := gatewayAPIConfig.HTTPRoute
	managedRouteMap := make(ManagedRouteMap)
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		httpRouteClient = gatewayClientv1.HTTPRoutes(gatewayAPIConfig.Namespace)
		clientset = r.Clientset.CoreV1().ConfigMaps(gatewayAPIConfig.Namespace)
	}
	configMap, err := utils.GetOrCreateConfigMap(gatewayAPIConfig.ConfigMap, utils.CreateConfigMapOptions{
		Clientset: clientset,
		Ctx:       ctx,
	})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	err = utils.GetConfigMapData(configMap, HTTPConfigMapKey, &managedRouteMap)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRoute, err := httpRouteClient.Get(ctx, httpRouteName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRouteRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	isHTTPRouteRuleListChanged := false
	for _, managedRoute := range managedRouteNameList {
		managedRouteName := managedRoute.Name
		_, isOk := managedRouteMap[managedRouteName]
		if !isOk {
			r.LogCtx.Logger.Info(fmt.Sprintf("%s is not in httpHeaderManagedRouteMap", managedRouteName))
			continue
		}
		isHTTPRouteRuleListChanged = true
		httpRouteRuleList, err = removeManagedHTTPRouteEntry(managedRouteMap, httpRouteRuleList, managedRouteName, httpRouteName)
		if err != nil {
			return pluginTypes.RpcError{
				ErrorString: err.Error(),
			}
		}
	}
	if !isHTTPRouteRuleListChanged {
		return pluginTypes.RpcError{}
	}
	oldHTTPRuleList := httpRoute.Spec.Rules
	httpRoute.Spec.Rules = httpRouteRuleList
	oldConfigMapData := make(ManagedRouteMap)
	err = utils.GetConfigMapData(configMap, HTTPConfigMapKey, &oldConfigMapData)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	taskList := []utils.Task{
		{
			Action: func() error {
				updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedHTTPRouteMock = updatedHTTPRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				httpRoute.Spec.Rules = oldHTTPRuleList
				updatedHTTPRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedHTTPRouteMock = updatedHTTPRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			Action: func() error {
				err = utils.UpdateConfigMapData(configMap, managedRouteMap, utils.UpdateConfigMapOptions{
					Clientset:    clientset,
					ConfigMapKey: HTTPConfigMapKey,
					Ctx:          ctx,
				})
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				err = utils.UpdateConfigMapData(configMap, oldConfigMapData, utils.UpdateConfigMapOptions{
					Clientset:    clientset,
					ConfigMapKey: HTTPConfigMapKey,
					Ctx:          ctx,
				})
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	err = utils.DoTransaction(r.LogCtx, taskList...)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func removeManagedHTTPRouteEntry(managedRouteMap ManagedRouteMap, routeRuleList HTTPRouteRuleList, managedRouteName string, httpRouteName string) (HTTPRouteRuleList, error) {
	routeManagedRouteMap, isOk := managedRouteMap[managedRouteName]
	if !isOk {
		return nil, fmt.Errorf(ManagedRouteMapEntryDeleteError, managedRouteName, managedRouteName)
	}
	managedRouteIndex, isOk := routeManagedRouteMap[httpRouteName]
	if !isOk {
		managedRouteMapKey := managedRouteName + "." + httpRouteName
		return nil, fmt.Errorf(ManagedRouteMapEntryDeleteError, managedRouteMapKey, managedRouteMapKey)
	}
	delete(routeManagedRouteMap, httpRouteName)
	if len(managedRouteMap[managedRouteName]) == 0 {
		delete(managedRouteMap, managedRouteName)
	}
	for _, currentRouteManagedRouteMap := range managedRouteMap {
		value := currentRouteManagedRouteMap[httpRouteName]
		if value > managedRouteIndex {
			currentRouteManagedRouteMap[httpRouteName]--
		}
	}
	routeRuleList = slices.Delete(routeRuleList, managedRouteIndex, managedRouteIndex+1)
	return routeRuleList, nil
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
