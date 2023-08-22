package plugin

import (
	"encoding/json"
	"errors"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
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
	if gatewayAPIConfig.HTTPRoute != "" {
		return r.setHTTPRouteWeight(rollout, desiredWeight, additionalDestinations, &gatewayAPIConfig)
	}
	if gatewayAPIConfig.TCPRoute != "" {
		return r.setTCPRouteWeight(rollout, desiredWeight, additionalDestinations, &gatewayAPIConfig)
	}
	return pluginTypes.RpcError{
		ErrorString: GatewayAPIManifestError,
	}
}

func getBackendRef(serviceName string, backendRefs []v1beta1.HTTPBackendRef) (*v1beta1.HTTPBackendRef, error) {
	var selectedService *v1beta1.HTTPBackendRef
	for i := 0; i < len(backendRefs); i++ {
		service := &backendRefs[i]
		nameOfCurrentService := string(service.Name)
		if nameOfCurrentService == serviceName {
			selectedService = service
			break
		}
	}
	if selectedService == nil {
		return nil, errors.New("service was not found in httpRoute")
	}
	return selectedService, nil
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
