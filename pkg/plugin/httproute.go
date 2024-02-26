package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	HTTPConfigMapKey = "httpManagedRoutes"
)

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRouteClient := r.HTTPRouteClient
	if !r.IsTest {
		gatewayV1beta1 := r.GatewayAPIClientset.GatewayV1beta1()
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
	canaryBackendRef, err := getBackendRef[*HTTPBackendRef, *HTTPRouteRule](canaryServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getBackendRef[*HTTPBackendRef, *HTTPRouteRule](stableServiceName, routeRuleList)
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
		msg := fmt.Sprintf(GatewayAPIUpdateError, httpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	}
	return pluginTypes.RpcError{}
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
		gatewayV1beta1 := r.GatewayAPIClientset.GatewayV1beta1()
		httpRouteClient = gatewayV1beta1.HTTPRoutes(gatewayAPIConfig.Namespace)
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
	canaryServiceName := v1beta1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceKind := v1beta1.Kind("Service")
	canaryServiceGroup := v1beta1.Group("")
	httpHeaderRouteRuleList, rpcError := getHTTPHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}
	httpRouteRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	backendRefNameList := []string{string(canaryServiceName), stableServiceName}
	httpRouteRule, err := getRouteRule[*HTTPBackendRef, *HTTPRouteRule](httpRouteRuleList, backendRefNameList...)
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
	httpHeaderRouteRule := v1beta1.HTTPRouteRule{
		Matches:     []v1beta1.HTTPRouteMatch{},
		BackendRefs: []v1beta1.HTTPBackendRef{},
	}
	for i := 0; i < len(httpRouteRule.Matches); i++ {
		httpHeaderRouteRule.Matches = append(httpHeaderRouteRule.Matches, v1beta1.HTTPRouteMatch{
			Path:        httpRouteRule.Matches[i].Path,
			Headers:     httpHeaderRouteRuleList,
			QueryParams: httpRouteRule.Matches[i].QueryParams,
		})
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

func getHTTPHeaderRouteRuleList(headerRouting *v1alpha1.SetHeaderRoute) ([]v1beta1.HTTPHeaderMatch, pluginTypes.RpcError) {
	httpHeaderRouteRuleList := []v1beta1.HTTPHeaderMatch{}
	for _, headerRule := range headerRouting.Match {
		httpHeaderRouteRule := v1beta1.HTTPHeaderMatch{
			Name: v1beta1.HTTPHeaderName(headerRule.HeaderName),
		}
		switch {
		case headerRule.HeaderValue.Exact != "":
			headerMatchType := v1beta1.HeaderMatchExact
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Exact
		case headerRule.HeaderValue.Prefix != "":
			headerMatchType := v1beta1.HeaderMatchRegularExpression
			httpHeaderRouteRule.Type = &headerMatchType
			httpHeaderRouteRule.Value = headerRule.HeaderValue.Prefix + ".*"
		case headerRule.HeaderValue.Regex != "":
			headerMatchType := v1beta1.HeaderMatchRegularExpression
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
		gatewayV1beta1 := r.GatewayAPIClientset.GatewayV1beta1()
		httpRouteClient = gatewayV1beta1.HTTPRoutes(gatewayAPIConfig.Namespace)
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
		httpRouteRuleList, err = removeManagedRouteEntry(managedRouteMap, httpRouteRuleList, managedRouteName, httpRouteName)
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
