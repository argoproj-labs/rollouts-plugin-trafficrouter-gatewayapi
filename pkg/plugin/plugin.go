package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// Type holds this controller type
const Type = "GatewayAPI"

const GatewayAPIUpdateError = "GatewayAPIUpdateError"

type RpcPlugin struct {
	LogCtx *logrus.Entry
	Client RpcPluginClient
}

type RpcPluginClient interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1beta1.HTTPRoute, error)
	Update(ctx context.Context, hTTPRoute *v1beta1.HTTPRoute, opts metav1.UpdateOptions) (*v1beta1.HTTPRoute, error)
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute" protobuf:"bytes,1,name=httpRoute"`
	Namespace string `json:"namespace" protobuf:"bytes,2,name=namespace"`
}

func (r *RpcPlugin) InitPlugin(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
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
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err = json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugin["argoproj-labs/gatewayAPI"], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	gatewayV1beta1 := clientset.GatewayV1beta1()
	r.Client = gatewayV1beta1.HTTPRoutes(gatewayAPIConfig.Namespace)
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugin["argoproj-labs/gatewayAPI"], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRoute, err := r.Client.Get(ctx, gatewayAPIConfig.HTTPRoute, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	rules := httpRoute.Spec.Rules
	backendRefs, err := getBackendRefList(rules)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef, err := getBackendRef(canaryServiceName, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryBackendRef.Weight = &desiredWeight
	stableBackendRef, err := getBackendRef(stableServiceName, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	restWeight := 100 - desiredWeight
	stableBackendRef.Weight = &restWeight
	_, err = r.Client.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	}
	return pluginTypes.RpcError{}
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

func getBackendRefList(rules []v1beta1.HTTPRouteRule) ([]v1beta1.HTTPBackendRef, error) {
	for _, rule := range rules {
		if rule.BackendRefs == nil {
			continue
		}
		backendRefs := rule.BackendRefs
		return backendRefs, nil
	}
	return nil, errors.New("backendRefs was not found in httpRoute")
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (*bool, pluginTypes.RpcError) {
	verified := true
	return &verified, pluginTypes.RpcError{ErrorString: plugin.ErrNotImplemented}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}
