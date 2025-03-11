package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/go-playground/validator/v10"
	"k8s.io/client-go/kubernetes"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
)

const (
	Type       = "GatewayAPI"
	PluginName = "argoproj-labs/gatewayAPI"
)

func (r *RpcPlugin) InitPlugin() pluginTypes.RpcError {
	log := utils.SetupLog()

	if r.IsTest {
		return pluginTypes.RpcError{}
	}
	kubeConfig, err := utils.GetKubeConfig()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	// Configure command-line overrides for the Kubernetes client:
	if r.CommandLineOpts.KubeClientQPS != 0 {
		log.Infof("KubeClientQPS set to: %f", r.CommandLineOpts.KubeClientQPS)
		kubeConfig.QPS = r.CommandLineOpts.KubeClientQPS
	}
	if r.CommandLineOpts.KubeClientBurst != 0 {
		log.Infof("KubeClientBurst set to: %d", r.CommandLineOpts.KubeClientBurst)
		kubeConfig.Burst = r.CommandLineOpts.KubeClientBurst
	}

	gatewayAPIClientset, err := gatewayApiClientset.NewForConfig(kubeConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	r.GatewayAPIClientset = gatewayAPIClientset
	r.Clientset = clientset
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	gatewayAPIConfig, err := getGatewayAPITrafficRoutingConfig(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	if !isConfigHasRoutes(gatewayAPIConfig) {
		return pluginTypes.RpcError{
			ErrorString: GatewayAPIManifestError,
		}
	}
	r.LogCtx.Info(fmt.Sprintf("[SetWeight] plugin %q controls HTTPRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.HTTPRoutes)))
	rpcError := forEachGatewayAPIRoute(gatewayAPIConfig.HTTPRoutes, func(route HTTPRoute) pluginTypes.RpcError {
		gatewayAPIConfig.HTTPRoute = route.Name
		return r.setHTTPRouteWeight(rollout, desiredWeight, gatewayAPIConfig)
	})
	if rpcError.HasError() {
		return rpcError
	}
	r.LogCtx.Info(fmt.Sprintf("[SetWeight] plugin %q controls GRPCRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.GRPCRoutes)))
	rpcError = forEachGatewayAPIRoute(gatewayAPIConfig.GRPCRoutes, func(route GRPCRoute) pluginTypes.RpcError {
		gatewayAPIConfig.GRPCRoute = route.Name
		return r.setGRPCRouteWeight(rollout, desiredWeight, gatewayAPIConfig)
	})
	if rpcError.HasError() {
		return rpcError
	}
	r.LogCtx.Info(fmt.Sprintf("[SetWeight] plugin %q controls TCPRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.TCPRoutes)))
	rpcError = forEachGatewayAPIRoute(gatewayAPIConfig.TCPRoutes, func(route TCPRoute) pluginTypes.RpcError {
		gatewayAPIConfig.TCPRoute = route.Name
		return r.setTCPRouteWeight(rollout, desiredWeight, gatewayAPIConfig)
	})
	return rpcError
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	gatewayAPIConfig, err := getGatewayAPITrafficRoutingConfig(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	if gatewayAPIConfig.HTTPRoutes != nil {
		gatewayAPIConfig.ConfigMapRWMutex.Lock()
		r.LogCtx.Info(fmt.Sprintf("[SetHeaderRoute] plugin %q controls HTTPRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.HTTPRoutes)))
		rpcError := forEachGatewayAPIRoute(gatewayAPIConfig.HTTPRoutes, func(route HTTPRoute) pluginTypes.RpcError {
			if !route.UseHeaderRoutes {
				return pluginTypes.RpcError{}
			}
			gatewayAPIConfig.HTTPRoute = route.Name
			return r.setHTTPHeaderRoute(rollout, headerRouting, gatewayAPIConfig)
		})
		if rpcError.HasError() {
			gatewayAPIConfig.ConfigMapRWMutex.Unlock()
			return rpcError
		}
		gatewayAPIConfig.ConfigMapRWMutex.Unlock()
	}
	if gatewayAPIConfig.GRPCRoutes != nil {
		gatewayAPIConfig.ConfigMapRWMutex.Lock()
		r.LogCtx.Info(fmt.Sprintf("[SetHeaderRoute] plugin %q controls GRPCRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.GRPCRoutes)))
		rpcError := forEachGatewayAPIRoute(gatewayAPIConfig.GRPCRoutes, func(route GRPCRoute) pluginTypes.RpcError {
			if !route.UseHeaderRoutes {
				return pluginTypes.RpcError{}
			}
			gatewayAPIConfig.GRPCRoute = route.Name
			return r.setGRPCHeaderRoute(rollout, headerRouting, gatewayAPIConfig)
		})
		if rpcError.HasError() {
			gatewayAPIConfig.ConfigMapRWMutex.Unlock()
			return rpcError
		}
		gatewayAPIConfig.ConfigMapRWMutex.Unlock()
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	gatewayAPIConfig, err := getGatewayAPITrafficRoutingConfig(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	if gatewayAPIConfig.HTTPRoutes != nil {
		gatewayAPIConfig.ConfigMapRWMutex.Lock()
		r.LogCtx.Info(fmt.Sprintf("[RemoveManagedRoutes] plugin %q controls HTTPRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.HTTPRoutes)))
		rpcError := forEachGatewayAPIRoute(gatewayAPIConfig.HTTPRoutes, func(route HTTPRoute) pluginTypes.RpcError {
			if !route.UseHeaderRoutes {
				return pluginTypes.RpcError{}
			}
			gatewayAPIConfig.HTTPRoute = route.Name
			return r.removeHTTPManagedRoutes(rollout.Spec.Strategy.Canary.TrafficRouting.ManagedRoutes, gatewayAPIConfig)
		})
		if rpcError.HasError() {
			gatewayAPIConfig.ConfigMapRWMutex.Unlock()
			return rpcError
		}
		gatewayAPIConfig.ConfigMapRWMutex.Unlock()
	}
	if gatewayAPIConfig.GRPCRoutes != nil {
		gatewayAPIConfig.ConfigMapRWMutex.Lock()
		r.LogCtx.Info(fmt.Sprintf("[RemoveManagedRoutes] plugin %q controls GRPCRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.GRPCRoutes)))
		rpcError := forEachGatewayAPIRoute(gatewayAPIConfig.GRPCRoutes, func(route GRPCRoute) pluginTypes.RpcError {
			if !route.UseHeaderRoutes {
				return pluginTypes.RpcError{}
			}
			gatewayAPIConfig.GRPCRoute = route.Name
			return r.removeGRPCManagedRoutes(rollout.Spec.Strategy.Canary.TrafficRouting.ManagedRoutes, gatewayAPIConfig)
		})
		if rpcError.HasError() {
			gatewayAPIConfig.ConfigMapRWMutex.Unlock()
			return rpcError
		}
		gatewayAPIConfig.ConfigMapRWMutex.Unlock()
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}

func getGatewayAPITrafficRoutingConfig(rollout *v1alpha1.Rollout) (*GatewayAPITrafficRouting, error) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	gatewayAPIConfig := &GatewayAPITrafficRouting{
		ConfigMap: defaults.ConfigMap,
	}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &gatewayAPIConfig)
	if err != nil {
		return gatewayAPIConfig, err
	}
	insertGatewayAPIRouteLists(gatewayAPIConfig)
	err = validate.Struct(gatewayAPIConfig)
	if err != nil {
		return gatewayAPIConfig, err
	}
	return gatewayAPIConfig, err
}

func insertGatewayAPIRouteLists(gatewayAPIConfig *GatewayAPITrafficRouting) {
	if gatewayAPIConfig.HTTPRoute != "" {
		gatewayAPIConfig.HTTPRoutes = append(gatewayAPIConfig.HTTPRoutes, HTTPRoute{
			Name:            gatewayAPIConfig.HTTPRoute,
			UseHeaderRoutes: true,
		})
	}
	if gatewayAPIConfig.GRPCRoute != "" {
		gatewayAPIConfig.GRPCRoutes = append(gatewayAPIConfig.GRPCRoutes, GRPCRoute{
			Name:            gatewayAPIConfig.GRPCRoute,
			UseHeaderRoutes: true,
		})
	}
	if gatewayAPIConfig.TCPRoute != "" {
		gatewayAPIConfig.TCPRoutes = append(gatewayAPIConfig.TCPRoutes, TCPRoute{
			Name:            gatewayAPIConfig.TCPRoute,
			UseHeaderRoutes: true,
		})
	}
}

func getRouteRule[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1], T3 GatewayAPIRouteRuleList[T1, T2]](routeRuleList T3, backendRefNameList ...string) (T2, error) {
	var backendRef T1
	var routeRule T2
	isFound := false
	for next, hasNext := routeRuleList.Iterator(); hasNext; {
		routeRule, hasNext = next()
		_, hasNext := routeRule.Iterator()
		if !hasNext {
			continue
		}
		for _, backendRefName := range backendRefNameList {
			isFound = false
			for next, hasNext := routeRule.Iterator(); hasNext; {
				backendRef, hasNext = next()
				if backendRefName == backendRef.GetName() {
					isFound = true
					continue
				}
			}
			if !isFound {
				break
			}
		}
		return routeRule, nil
	}
	return nil, routeRuleList.Error()
}

func getBackendRefs[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1], T3 GatewayAPIRouteRuleList[T1, T2]](backendRefName string, routeRuleList T3) ([]T1, error) {
	var backendRef T1
	var routeRule T2
	var matchedRefs []T1
	for next, hasNext := routeRuleList.Iterator(); hasNext; {
		routeRule, hasNext = next()
		for next, hasNext := routeRule.Iterator(); hasNext; {
			backendRef, hasNext = next()
			if backendRefName == backendRef.GetName() {
				matchedRefs = append(matchedRefs, backendRef)
			}
		}
	}
	if len(matchedRefs) > 0 {
		return matchedRefs, nil
	}
	return nil, routeRuleList.Error()
}

// This will return the backendRefs slices that matched the backendRefName but split by the routeRule index to be used for updating the weight
// under consideration of the managedRoute rules present from the configmap (identifiable by the index of the rule on an HttpRoute)
func getIndexedBackendRefs[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1], T3 GatewayAPIRouteRuleList[T1, T2]](backendRefName string, routeRuleList T3) ([]IndexedBackendRefs[T1], error) {
	var results []IndexedBackendRefs[T1]
	var backendRef T1
	var routeRule T2
	ruleIndex := 0
	for next, hasNext := routeRuleList.Iterator(); hasNext; {
		// Has to be inside of the for loop since it will be reset to 0 for each routeRule
		var matchedRefs []T1
		routeRule, hasNext = next()
		for nextRef, hasNextRef := routeRule.Iterator(); hasNextRef; {
			backendRef, hasNextRef = nextRef()
			if backendRefName == backendRef.GetName() {
				matchedRefs = append(matchedRefs, backendRef)
			}
		}
		if len(matchedRefs) > 0 {
			results = append(results, IndexedBackendRefs[T1]{
				RuleIndex: ruleIndex,
				Refs:      matchedRefs,
			})
		}
		ruleIndex++
	}
	if len(results) > 0 {
		return results, nil
	}
	return nil, routeRuleList.Error()
}

func isConfigHasRoutes(config *GatewayAPITrafficRouting) bool {
	return len(config.HTTPRoutes) > 0 || len(config.TCPRoutes) > 0 || len(config.GRPCRoutes) > 0
}

func forEachGatewayAPIRoute[T1 GatewayAPIRoute](routeList []T1, fn func(route T1) pluginTypes.RpcError) pluginTypes.RpcError {
	var err pluginTypes.RpcError
	for _, route := range routeList {
		if err = fn(route); err.HasError() {
			return err
		}
	}
	return pluginTypes.RpcError{}
}

func getGatewayAPIRouteNameList[T1 GatewayAPIRoute](gatewayAPIRouteList []T1) []string {
	gatewayAPIRouteNameList := make([]string, len(gatewayAPIRouteList))
	for index, value := range gatewayAPIRouteList {
		gatewayAPIRouteNameList[index] = value.GetName()
	}
	return gatewayAPIRouteNameList
}
