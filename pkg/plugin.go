package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayV1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// Type holds this controller type
const Type = "GatewayAPI"

const httpRoutes = "httproutes"
const GatewayAPIUpdateError = "GatewayAPIUpdateError"

type RpcPlugin struct {
	LogCtx *logrus.Entry
	Client *gatewayApiClientset.Clientset
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute" protobuf:"bytes,1,name=httpRoute"`
}

func (r *RpcPlugin) NewTrafficRouterPlugin() pluginTypes.RpcError {
	kubeConfig, err := utils.GetKubeConfig()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	r.Client, err = gatewayApiClientset.NewForConfig(kubeConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	// TODO: Remove this line when Zach will push the changes in argo-rollouts
	trafficRouting := (interface{}(rollout.Spec.Strategy.Canary.TrafficRouting)).(struct{ Plugin map[string]json.RawMessage })
	err := json.Unmarshal(trafficRouting.Plugin["gatewayAPI"], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	gatewayV1beta1 := r.Client.GatewayV1beta1()
	httpRouteName := gatewayAPIConfig.HTTPRoute
	httpRouteClientset := gatewayV1beta1.HTTPRoutes(metav1.NamespaceAll)
	httpRoute, err := httpRouteClientset.Get(ctx, httpRouteName, metav1.GetOptions{})
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
	rules, err = mergeBackendRefs(rules, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRoute.Spec.Rules = rules
	_, err = httpRouteClientset.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
		r.LogCtx.Error(msg)
	}
	return pluginTypes.RpcError{
		ErrorString: err.Error(),
	}
}

func getBackendRef(serviceName string, backendRefs []gatewayV1beta1.HTTPBackendRef) (*gatewayV1beta1.HTTPBackendRef, error) {
	var selectedService *gatewayV1beta1.HTTPBackendRef
	for _, service := range backendRefs {
		nameOfCurrentService := string(service.Name)
		if nameOfCurrentService == serviceName {
			selectedService = &service
			break
		}
	}
	if selectedService == nil {
		return nil, errors.New("service was not found in httpRoute")
	}
	return selectedService, nil
}

func getBackendRefList(rules []gatewayV1beta1.HTTPRouteRule) ([]gatewayV1beta1.HTTPBackendRef, error) {
	for _, rule := range rules {
		backendRefs := rule.BackendRefs
		return backendRefs, nil
	}
	return nil, errors.New("backendRefs was not found in httpRoute")
}

func mergeBackendRefs(rules []gatewayV1beta1.HTTPRouteRule, backendRefs []gatewayV1beta1.HTTPBackendRef) ([]gatewayV1beta1.HTTPRouteRule, error) {
	for _, rule := range rules {
		if rule.BackendRefs == nil {
			continue
		}
		rule.BackendRefs = backendRefs
		return rules, nil
	}
	return rules, errors.New("backendRefs was not found and merged in rules")
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (*bool, pluginTypes.RpcError) {
	return nil, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}
