package plugin

import (
	"encoding/json"
	"fmt"

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
	GatewayAPIManifestError = "No routes configured. One of 'routes', 'tcpRoute' or 'httpRoute' should be set"
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

	if gatewayAPIConfig.HTTPRoute != "" {
		gatewayAPIConfig.HTTPRoutes = append(gatewayAPIConfig.HTTPRoutes, gatewayAPIConfig.HTTPRoute)
	}

	if gatewayAPIConfig.TCPRoute != "" {
		gatewayAPIConfig.TCPRoutes = append(gatewayAPIConfig.TCPRoutes, gatewayAPIConfig.TCPRoute)
	}

	if len(gatewayAPIConfig.HTTPRoutes) == 0 && len(gatewayAPIConfig.TCPRoutes) == 0 {
		return pluginTypes.RpcError{
			ErrorString: GatewayAPIManifestError,
		}
	}

	for _, name := range gatewayAPIConfig.HTTPRoutes {
		err := r.setHTTPRouteWeight(rollout, desiredWeight, additionalDestinations, GatewayAPITrafficRouting{
			Namespace: gatewayAPIConfig.Namespace,
			HTTPRoute: name,
		})
		if err.HasError() {
			return pluginTypes.RpcError{
				ErrorString: fmt.Sprintf("set weight for HTTPRoute %q: %s", name, err),
			}
		}
	}
	for _, name := range gatewayAPIConfig.TCPRoutes {
		err := r.setTCPRouteWeight(rollout, desiredWeight, additionalDestinations, GatewayAPITrafficRouting{
			Namespace: gatewayAPIConfig.Namespace,
			TCPRoute:  name,
		})
		if err.HasError() {
			return pluginTypes.RpcError{
				ErrorString: fmt.Sprintf("set weight for TCPRoute %q: %s", name, err),
			}
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
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
