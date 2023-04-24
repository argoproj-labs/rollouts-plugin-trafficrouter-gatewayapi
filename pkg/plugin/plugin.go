package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayApiv1beta1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

// Type holds this controller type
const Type = "GatewayAPI"

const GatewayAPIUpdateError = "GatewayAPIUpdateError"

type RpcPlugin struct {
	IsTest               bool
	LogCtx               *logrus.Entry
	Client               *gatewayApiClientset.Clientset
	UpdatedMockHttpRoute *v1beta1.HTTPRoute
	HttpRouteClient      gatewayApiv1beta1.HTTPRouteInterface
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute" protobuf:"bytes,1,name=httpRoute"`
	Namespace string `json:"namespace" protobuf:"bytes,2,name=namespace"`
}

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
	ctx := context.TODO()
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/gatewayAPI"], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRouteClient := r.HttpRouteClient
	if !r.IsTest {
		gatewayV1beta1 := r.Client.GatewayV1beta1()
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
	updatedHttpRoute, err := httpRouteClient.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if r.IsTest {
		r.UpdatedMockHttpRoute = updatedHttpRoute
	}
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

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}
