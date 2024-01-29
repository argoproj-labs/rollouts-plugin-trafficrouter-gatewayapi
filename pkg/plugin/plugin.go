package plugin

import (
	"encoding/json"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

const (
	// Type holds this controller type
	Type       = "GatewayAPI"
	PluginName = "argoproj-labs/gatewayAPI"

	GatewayAPIUpdateError   = "GatewayAPIUpdateError"
	GatewayAPIManifestError = "httpRoute and tcpRoute fields are empty. tcpRoute or httpRoute should be set"
)

func (r *RpcPlugin) InitPlugin() pluginTypes.RpcError {
	if r.IsTest {
		return pluginTypes.RpcError{}
	}
	kubeConfig, err := utils.GetKubeConfig()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	clientset, err := gatewayApiClientset.NewForConfig(kubeConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	r.Client = clientset

	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	switch {
	case gatewayAPIConfig.HTTPRoute != "":
		rpcError := r.setHTTPRouteWeight(rollout, desiredWeight, additionalDestinations, &gatewayAPIConfig)
		if rpcError.HasError() {
			return rpcError
		}
	case gatewayAPIConfig.TCPRoute != "":
		rpcError := r.setTCPRouteWeight(rollout, desiredWeight, additionalDestinations, &gatewayAPIConfig)
		if rpcError.HasError() {
			return rpcError
		}
	default:
		return pluginTypes.RpcError{
			ErrorString: GatewayAPIManifestError,
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	switch {
	case gatewayAPIConfig.HTTPRoute != "":
		httpHeaderRoute.mu.Lock()
		rpcError := r.setHTTPHeaderRoute(rollout, headerRouting, &gatewayAPIConfig)
		if rpcError.HasError() {
			httpHeaderRoute.mu.Unlock()
			return rpcError
		}
		httpHeaderRoute.mu.Unlock()
	default:
		return pluginTypes.RpcError{
			ErrorString: "httpRoute field is empty. It has to be set to use setHeadRoute functionality",
		}
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
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	switch {
	case gatewayAPIConfig.HTTPRoute != "":
		httpHeaderRoute.mu.Lock()
		rpcError := r.removeHTTPManagedRoutes(rollout.Spec.Strategy.Canary.TrafficRouting.ManagedRoutes, &gatewayAPIConfig)
		if rpcError.HasError() {
			httpHeaderRoute.mu.Unlock()
			return rpcError
		}
		httpHeaderRoute.mu.Unlock()
	default:
		return pluginTypes.RpcError{
			ErrorString: "httpRoute field is empty. It has to be set to remove managed routes",
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}

func getBackendRefList[T GatewayAPIBackendRefList](ruleList GatewayAPIRouteRuleCollection[T]) (T, error) {
	var backendRefList T
	for next, hasNext := ruleList.Iterator(); hasNext; {
		backendRefList, hasNext = next()
		if backendRefList == nil {
			continue
		}
		return backendRefList, nil
	}
	return nil, ruleList.Error()
}

func getBackendRef[T GatewayAPIBackendRef](backendRefName string, backendRefList GatewayAPIBackendRefCollection[T]) (T, error) {
	var selectedService, backendRef T
	for next, hasNext := backendRefList.Iterator(); hasNext; {
		backendRef, hasNext = next()
		nameOfCurrentService := backendRef.GetName()
		if nameOfCurrentService == backendRefName {
			selectedService = backendRef
			break
		}
	}
	if selectedService == nil {
		return nil, backendRefList.Error()
	}
	return selectedService, nil
}
