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
	GRPCConfigMapKey = "grpcManagedRoutes"
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
	routeRuleList := GRPCRouteRuleList(grpcRoute.Spec.Rules)
	canaryBackendRefs, err := getBackendRefs(canaryServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	for _, ref := range canaryBackendRefs {
		ref.Weight = &desiredWeight
	}
	stableBackendRefs, err := getBackendRefs(stableServiceName, routeRuleList)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	for _, ref := range stableBackendRefs {
		ref.Weight = &restWeight
	}
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
		managedRouteList := []v1alpha1.MangedRoutes{
			{
				Name: headerRouting.Name,
			},
		}
		return r.removeHTTPManagedRoutes(managedRouteList, gatewayAPIConfig)
	}
	ctx := context.TODO()
	grpcRouteClient := r.GRPCRouteClient
	managedRouteMap := make(ManagedRouteMap)
	grpcRouteName := gatewayAPIConfig.GRPCRoute
	clientset := r.TestClientset
	if !r.IsTest {
		gatewayClientV1 := r.GatewayAPIClientset.GatewayV1()
		grpcRouteClient = gatewayClientV1.GRPCRoutes(gatewayAPIConfig.Namespace)
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
	err = utils.GetConfigMapData(configMap, GRPCConfigMapKey, &managedRouteMap)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	grpcRoute, err := grpcRouteClient.Get(ctx, grpcRouteName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := gatewayv1.ObjectName(rollout.Spec.Strategy.Canary.CanaryService)
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	canaryServiceKind := gatewayv1.Kind("Service")
	canaryServiceGroup := gatewayv1.Group("")
	grpcHeaderRouteRuleList, rpcError := getGRPCHeaderRouteRuleList(headerRouting)
	if rpcError.HasError() {
		return rpcError
	}
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
	grpcHeaderRouteRule := gatewayv1.GRPCRouteRule{
		Matches: []gatewayv1.GRPCRouteMatch{},
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
	matchLength := len(grpcRouteRule.Matches)
	if matchLength == 0 {
		grpcHeaderRouteRule.Matches = []gatewayv1.GRPCRouteMatch{
			{
				Headers: grpcHeaderRouteRuleList,
			},
		}
	} else {
		for i := 0; i < matchLength; i++ {
			grpcHeaderRouteRule.Matches = append(grpcHeaderRouteRule.Matches, gatewayv1.GRPCRouteMatch{
				Method:  grpcRouteRule.Matches[i].Method,
				Headers: grpcHeaderRouteRuleList,
			})
		}
	}
	grpcRouteRuleList = append(grpcRouteRuleList, grpcHeaderRouteRule)
	oldGRPCRuleList := grpcRoute.Spec.Rules
	grpcRoute.Spec.Rules = grpcRouteRuleList
	oldConfigMapData := make(ManagedRouteMap)
	err = utils.GetConfigMapData(configMap, GRPCConfigMapKey, &oldConfigMapData)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	taskList := []utils.Task{
		{
			Action: func() error {
				updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedGRPCRouteMock = updatedGRPCRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				grpcRoute.Spec.Rules = oldGRPCRuleList
				updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedGRPCRouteMock = updatedGRPCRoute
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
				managedRouteMap[headerRouting.Name][grpcRouteName] = len(grpcRouteRuleList) - 1
				err = utils.UpdateConfigMapData(configMap, managedRouteMap, utils.UpdateConfigMapOptions{
					Clientset:    clientset,
					ConfigMapKey: GRPCConfigMapKey,
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
					ConfigMapKey: GRPCConfigMapKey,
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

func getGRPCHeaderRouteRuleList(headerRouting *v1alpha1.SetHeaderRoute) ([]gatewayv1.GRPCHeaderMatch, pluginTypes.RpcError) {
	grpcHeaderRouteRuleList := []gatewayv1.GRPCHeaderMatch{}
	for _, headerRule := range headerRouting.Match {
		grpcHeaderRouteRule := gatewayv1.GRPCHeaderMatch{
			Name: gatewayv1.GRPCHeaderName(headerRule.HeaderName),
		}
		switch {
		case headerRule.HeaderValue.Exact != "":
			headerMatchType := gatewayv1.HeaderMatchExact
			grpcHeaderRouteRule.Type = &headerMatchType
			grpcHeaderRouteRule.Value = headerRule.HeaderValue.Exact
		case headerRule.HeaderValue.Prefix != "":
			headerMatchType := gatewayv1.HeaderMatchRegularExpression
			grpcHeaderRouteRule.Type = &headerMatchType
			grpcHeaderRouteRule.Value = headerRule.HeaderValue.Prefix + ".*"
		case headerRule.HeaderValue.Regex != "":
			headerMatchType := gatewayv1.HeaderMatchRegularExpression
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

func (r *RpcPlugin) removeGRPCManagedRoutes(managedRouteNameList []v1alpha1.MangedRoutes, gatewayAPIConfig *GatewayAPITrafficRouting) pluginTypes.RpcError {
	ctx := context.TODO()
	grpcRouteClient := r.GRPCRouteClient
	clientset := r.TestClientset
	grpcRouteName := gatewayAPIConfig.GRPCRoute
	managedRouteMap := make(ManagedRouteMap)
	if !r.IsTest {
		gatewayClientv1 := r.GatewayAPIClientset.GatewayV1()
		grpcRouteClient = gatewayClientv1.GRPCRoutes(gatewayAPIConfig.Namespace)
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
	err = utils.GetConfigMapData(configMap, GRPCConfigMapKey, &managedRouteMap)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	grpcRoute, err := grpcRouteClient.Get(ctx, grpcRouteName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	grpcRouteRuleList := GRPCRouteRuleList(grpcRoute.Spec.Rules)
	isGRPCRouteRuleListChanged := false
	for _, managedRoute := range managedRouteNameList {
		managedRouteName := managedRoute.Name
		_, isOk := managedRouteMap[managedRouteName]
		if !isOk {
			r.LogCtx.Logger.Infof("%s is not in grpcHeaderManagedRouteMap", managedRouteName)
			continue
		}
		isGRPCRouteRuleListChanged = true
		grpcRouteRuleList, err = removeManagedGRPCRouteEntry(managedRouteMap, grpcRouteRuleList, managedRouteName, grpcRouteName)
		if err != nil {
			return pluginTypes.RpcError{
				ErrorString: err.Error(),
			}
		}
	}
	if !isGRPCRouteRuleListChanged {
		return pluginTypes.RpcError{}
	}
	oldGRPCRuleList := grpcRoute.Spec.Rules
	grpcRoute.Spec.Rules = grpcRouteRuleList
	oldConfigMapData := make(ManagedRouteMap)
	err = utils.GetConfigMapData(configMap, GRPCConfigMapKey, &oldConfigMapData)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	taskList := []utils.Task{
		{
			Action: func() error {
				updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedGRPCRouteMock = updatedGRPCRoute
				}
				if err != nil {
					return err
				}
				return nil
			},
			ReverseAction: func() error {
				grpcRoute.Spec.Rules = oldGRPCRuleList
				updatedGRPCRoute, err := grpcRouteClient.Update(ctx, grpcRoute, metav1.UpdateOptions{})
				if r.IsTest {
					r.UpdatedGRPCRouteMock = updatedGRPCRoute
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
					ConfigMapKey: GRPCConfigMapKey,
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
					ConfigMapKey: GRPCConfigMapKey,
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

func removeManagedGRPCRouteEntry(managedRouteMap ManagedRouteMap, routeRuleList GRPCRouteRuleList, managedRouteName string, grpcRouteName string) (GRPCRouteRuleList, error) {
	routeManagedRouteMap, isOk := managedRouteMap[managedRouteName]
	if !isOk {
		return nil, fmt.Errorf(ManagedRouteMapEntryDeleteError, managedRouteName, managedRouteName)
	}
	managedRouteIndex, isOk := routeManagedRouteMap[grpcRouteName]
	if !isOk {
		managedRouteMapKey := managedRouteName + "." + grpcRouteName
		return nil, fmt.Errorf(ManagedRouteMapEntryDeleteError, managedRouteMapKey, managedRouteMapKey)
	}
	delete(routeManagedRouteMap, grpcRouteName)
	if len(managedRouteMap[managedRouteName]) == 0 {
		delete(managedRouteMap, managedRouteName)
	}
	for _, currentRouteManagedRouteMap := range managedRouteMap {
		value := currentRouteManagedRouteMap[grpcRouteName]
		if value > managedRouteIndex {
			currentRouteManagedRouteMap[grpcRouteName]--
		}
	}
	routeRuleList = slices.Delete(routeRuleList, managedRouteIndex, managedRouteIndex+1)
	return routeRuleList, nil
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
