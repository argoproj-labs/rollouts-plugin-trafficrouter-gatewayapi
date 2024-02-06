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
	gatewayAPIConfig := &GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	if gatewayAPIConfig.HTTPRoute != "" {
		gatewayAPIConfig.Routes = append(gatewayAPIConfig.Routes, GatewayAPIRouteReference{
			Kind: HTTPRouteKind,
			Name: gatewayAPIConfig.HTTPRoute,
		})
	}

	if gatewayAPIConfig.TCPRoute != "" {
		gatewayAPIConfig.Routes = append(gatewayAPIConfig.Routes, GatewayAPIRouteReference{
			Kind: TCPRouteKind,
			Name: gatewayAPIConfig.TCPRoute,
		})
	}

	if len(gatewayAPIConfig.Routes) == 0 {
		return pluginTypes.RpcError{
			ErrorString: GatewayAPIManifestError,
		}
	}

	var rpcErr pluginTypes.RpcError
	for _, ref := range gatewayAPIConfig.Routes {
		if ref.Kind == "" {
			// Assume HTTPRoute by default
			ref.Kind = HTTPRouteKind
		}

		switch ref.Kind {
		case HTTPRouteKind:
			rpcErr = r.setHTTPRouteWeight(rollout, desiredWeight, additionalDestinations, GatewayAPITrafficRouting{
				Namespace: gatewayAPIConfig.Namespace,
				HTTPRoute: ref.Name,
			})
		case TCPRouteKind:
			rpcErr = r.setTCPRouteWeight(rollout, desiredWeight, additionalDestinations, GatewayAPITrafficRouting{
				Namespace: gatewayAPIConfig.Namespace,
				TCPRoute:  ref.Name,
			})
		default:
			r.LogCtx.Warnf("unsupported kind %q for route %q, supported values: %q, %q", ref.Kind, ref.Name, HTTPRouteKind, TCPRouteKind)
		}
		if rpcErr.HasError() {
			return pluginTypes.RpcError{
				ErrorString: fmt.Sprintf("set weight for %s %q: %s", ref.Kind, ref.Name, err),
			}
		}
	}

	return rpcErr
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
