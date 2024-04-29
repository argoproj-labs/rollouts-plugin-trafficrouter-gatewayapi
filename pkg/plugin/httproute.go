package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	HTTPConfigMapKey = "httpManagedRoutes"
)

func httpRouteV1Alpha2ToV1(obj *gatewayv1alpha2.HTTPRoute) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		TypeMeta:   obj.TypeMeta,
		ObjectMeta: obj.ObjectMeta,
		Spec:       obj.Spec,
		Status:     obj.Status,
	}
}

func httpRouteV1ToV1Alpha2(obj *gatewayv1.HTTPRoute) *gatewayv1alpha2.HTTPRoute {
	return &gatewayv1alpha2.HTTPRoute{
		TypeMeta:   obj.TypeMeta,
		ObjectMeta: obj.ObjectMeta,
		Spec:       obj.Spec,
		Status:     obj.Status,
	}
}

func httpRouteV1Beta1ToV1(obj *gatewayv1beta1.HTTPRoute) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		TypeMeta:   obj.TypeMeta,
		ObjectMeta: obj.ObjectMeta,
		Spec:       obj.Spec,
		Status:     obj.Status,
	}
}

func httpRouteV1ToV1Beta1(obj *gatewayv1.HTTPRoute) *gatewayv1beta1.HTTPRoute {
	return &gatewayv1beta1.HTTPRoute{
		TypeMeta:   obj.TypeMeta,
		ObjectMeta: obj.ObjectMeta,
		Spec:       obj.Spec,
		Status:     obj.Status,
	}
}

func (r *RpcPlugin) getHTTPRoute(ctx context.Context, namespace, name string) (*gatewayv1.HTTPRoute, error) {
	if r.IsTest {
		return r.HTTPRouteClient.Get(ctx, name, metav1.GetOptions{})
	}
	switch r.HTTPRouteAPIVersion {
	case "v1alpha2":
		route, err := r.GatewayAPIClientset.GatewayV1alpha2().HTTPRoutes(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return httpRouteV1Alpha2ToV1(route), nil
	case "v1beta1":
		route, err := r.GatewayAPIClientset.GatewayV1beta1().HTTPRoutes(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return httpRouteV1Beta1ToV1(route), nil
	default:
		return r.GatewayAPIClientset.GatewayV1().HTTPRoutes(namespace).Get(ctx, name, metav1.GetOptions{})
	}
}

func (r *RpcPlugin) updateHTTPRoute(ctx context.Context, input *gatewayv1.HTTPRoute) (*gatewayv1.HTTPRoute, error) {
	if r.IsTest {
		return r.HTTPRouteClient.Update(ctx, input, metav1.UpdateOptions{})
	}
	switch r.HTTPRouteAPIVersion {
	case "v1alpha2":
		output, err := r.GatewayAPIClientset.GatewayV1alpha2().HTTPRoutes(input.Namespace).Update(ctx, httpRouteV1ToV1Alpha2(input), metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return httpRouteV1Alpha2ToV1(output), nil
	case "v1beta1":
		output, err := r.GatewayAPIClientset.GatewayV1beta1().HTTPRoutes(input.Namespace).Update(ctx, httpRouteV1ToV1Beta1(input), metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return httpRouteV1Beta1ToV1(output), nil
	default:
		return r.GatewayAPIClientset.GatewayV1().HTTPRoutes(input.Namespace).Update(ctx, input, metav1.UpdateOptions{})
	}
}

func (r *RpcPlugin) setHTTPRouteWeight(rollout *v1alpha1.Rollout, desiredWeight int32, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	httpRoute, err := r.getHTTPRoute(ctx, gatewayAPIConfig.Namespace, gatewayAPIConfig.HTTPRoute)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	routeRuleList := HTTPRouteRuleList(httpRoute.Spec.Rules)
	canaryBackendRef, err := getBackendRef(canaryServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getBackendRef(stableServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	stableBackendRef.Weight = &restWeight
	updatedHTTPRoute, err := r.updateHTTPRoute(ctx, httpRoute)
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
	managedRouteMap := make(ManagedRouteMap)
	httpRouteName := gatewayAPIConfig.HTTPRoute
	clientset := r.TestClientset
	if !r.IsTest {
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
	httpRoute, err := r.getHTTPRoute(ctx, gatewayAPIConfig.Namespace, httpRouteName)
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
		Matches:     []gatewayv1.HTTPRouteMatch{},
		BackendRefs: []gatewayv1.HTTPBackendRef{},
	}
	for i := 0; i < len(httpRouteRule.Matches); i++ {
		httpHeaderRouteRule.Matches = append(httpHeaderRouteRule.Matches, gatewayv1.HTTPRouteMatch{
			Path:        httpRouteRule.Matches[i].Path,
			Headers:     httpHeaderRouteRuleList,
			QueryParams: httpRouteRule.Matches[i].QueryParams,
		})
	}
	httpHeaderRouteRule.BackendRefs = []gatewayv1.HTTPBackendRef{
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
				updatedHTTPRoute, err := r.updateHTTPRoute(ctx, httpRoute)
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
				updatedHTTPRoute, err := r.updateHTTPRoute(ctx, httpRoute)
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
	clientset := r.TestClientset
	httpRouteName := gatewayAPIConfig.HTTPRoute
	managedRouteMap := make(ManagedRouteMap)
	if !r.IsTest {
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
	httpRoute, err := r.getHTTPRoute(ctx, gatewayAPIConfig.Namespace, httpRouteName)
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
				updatedHTTPRoute, err := r.updateHTTPRoute(ctx, httpRoute)
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
				updatedHTTPRoute, err := r.updateHTTPRoute(ctx, httpRoute)
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
