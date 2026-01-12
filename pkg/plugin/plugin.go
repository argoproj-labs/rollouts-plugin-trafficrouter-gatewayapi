package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/go-playground/validator/v10"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	gatewayAPIConfig, err := r.getGatewayAPIConfigWithDiscovery(rollout)
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
		return r.setHTTPRouteWeight(rollout, desiredWeight, additionalDestinations, gatewayAPIConfig)
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
	if rpcError.HasError() {
		return rpcError
	}
	r.LogCtx.Info(fmt.Sprintf("[SetWeight] plugin %q controls TLSRoutes: %v", PluginName, getGatewayAPIRouteNameList(gatewayAPIConfig.TLSRoutes)))
	rpcError = forEachGatewayAPIRoute(gatewayAPIConfig.TLSRoutes, func(route TLSRoute) pluginTypes.RpcError {
		gatewayAPIConfig.TLSRoute = route.Name
		return r.setTLSRouteWeight(rollout, desiredWeight, gatewayAPIConfig)
	})
	return rpcError
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	gatewayAPIConfig, err := r.getGatewayAPIConfigWithDiscovery(rollout)
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
	gatewayAPIConfig, err := r.getGatewayAPIConfigWithDiscovery(rollout)
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

func (r *RpcPlugin) getGatewayAPIConfigWithDiscovery(rollout *v1alpha1.Rollout) (*GatewayAPITrafficRouting, error) {
	gatewayAPIConfig, err := getGatewayAPITrafficRoutingConfig(rollout)
	if err != nil {
		return nil, err
	}

	if gatewayAPIConfig.HTTPRouteSelector != nil ||
		gatewayAPIConfig.GRPCRouteSelector != nil ||
		gatewayAPIConfig.TCPRouteSelector != nil ||
		gatewayAPIConfig.TLSRouteSelector != nil {
		if err := r.discoverRoutesBySelector(rollout, gatewayAPIConfig); err != nil {
			return nil, err
		}
	}

	return gatewayAPIConfig, nil
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

func (r *RpcPlugin) discoverRoutesBySelector(rollout *v1alpha1.Rollout, gatewayAPIConfig *GatewayAPITrafficRouting) error {
	namespace := gatewayAPIConfig.Namespace
	if namespace == "" {
		namespace = rollout.Namespace
	}

	if gatewayAPIConfig.HTTPRouteSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(gatewayAPIConfig.HTTPRouteSelector)
		if err != nil {
			return err
		}

		httpRouteList, err := r.GatewayAPIClientset.GatewayV1().HTTPRoutes(namespace).List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: selector.String()},
		)
		if err != nil {
			return err
		}

		for _, route := range httpRouteList.Items {
			gatewayAPIConfig.HTTPRoutes = append(gatewayAPIConfig.HTTPRoutes, HTTPRoute{
				Name:            route.Name,
				UseHeaderRoutes: false,
			})
		}

		if len(httpRouteList.Items) > 0 {
			r.LogCtx.Info(fmt.Sprintf("[discoverRoutesBySelector] discovered %d HTTPRoutes via selector", len(httpRouteList.Items)))
		}
	}

	if gatewayAPIConfig.GRPCRouteSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(gatewayAPIConfig.GRPCRouteSelector)
		if err != nil {
			return err
		}

		grpcRouteList, err := r.GatewayAPIClientset.GatewayV1().GRPCRoutes(namespace).List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: selector.String()},
		)
		if err != nil {
			return err
		}

		for _, route := range grpcRouteList.Items {
			gatewayAPIConfig.GRPCRoutes = append(gatewayAPIConfig.GRPCRoutes, GRPCRoute{
				Name:            route.Name,
				UseHeaderRoutes: false,
			})
		}

		if len(grpcRouteList.Items) > 0 {
			r.LogCtx.Info(fmt.Sprintf("[discoverRoutesBySelector] discovered %d GRPCRoutes via selector", len(grpcRouteList.Items)))
		}
	}

	if gatewayAPIConfig.TCPRouteSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(gatewayAPIConfig.TCPRouteSelector)
		if err != nil {
			return err
		}

		tcpRouteList, err := r.GatewayAPIClientset.GatewayV1alpha2().TCPRoutes(namespace).List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: selector.String()},
		)
		if err != nil {
			return err
		}

		for _, route := range tcpRouteList.Items {
			gatewayAPIConfig.TCPRoutes = append(gatewayAPIConfig.TCPRoutes, TCPRoute{
				Name:            route.Name,
				UseHeaderRoutes: false,
			})
		}

		if len(tcpRouteList.Items) > 0 {
			r.LogCtx.Info(fmt.Sprintf("[discoverRoutesBySelector] discovered %d TCPRoutes via selector", len(tcpRouteList.Items)))
		}
	}

	if gatewayAPIConfig.TLSRouteSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(gatewayAPIConfig.TLSRouteSelector)
		if err != nil {
			return err
		}

		tlsRouteList, err := r.GatewayAPIClientset.GatewayV1alpha2().TLSRoutes(namespace).List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: selector.String()},
		)
		if err != nil {
			return err
		}

		for _, route := range tlsRouteList.Items {
			gatewayAPIConfig.TLSRoutes = append(gatewayAPIConfig.TLSRoutes, TLSRoute{
				Name:            route.Name,
				UseHeaderRoutes: false,
			})
		}

		if len(tlsRouteList.Items) > 0 {
			r.LogCtx.Info(fmt.Sprintf("[discoverRoutesBySelector] discovered %d TLSRoutes via selector", len(tlsRouteList.Items)))
		}
	}

	return nil
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
	if gatewayAPIConfig.TLSRoute != "" {
		gatewayAPIConfig.TLSRoutes = append(gatewayAPIConfig.TLSRoutes, TLSRoute{
			Name:            gatewayAPIConfig.TLSRoute,
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

func getBackendRefsWithSkipIndexes[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1], T3 GatewayAPIRouteRuleList[T1, T2]](backendRefName string, routeRuleList T3, skipIndexes map[int]bool) ([]T1, error) {
	var backendRef T1
	var routeRule T2
	var matchedRefs []T1
	index := 0
	for next, hasNext := routeRuleList.Iterator(); hasNext; {
		routeRule, hasNext = next()
		if skipIndexes[index] {
			index++
			continue
		}
		for next, hasNext := routeRule.Iterator(); hasNext; {
			backendRef, hasNext = next()
			if backendRefName == backendRef.GetName() {
				matchedRefs = append(matchedRefs, backendRef)
			}
		}
		index++
	}
	if len(matchedRefs) > 0 {
		return matchedRefs, nil
	}
	return nil, routeRuleList.Error()
}

func isConfigHasRoutes(config *GatewayAPITrafficRouting) bool {
	return len(config.HTTPRoutes) > 0 || len(config.TCPRoutes) > 0 || len(config.GRPCRoutes) > 0 || len(config.TLSRoutes) > 0
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
