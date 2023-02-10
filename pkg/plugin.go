package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-trafficrouter-gatewayapi-plugin/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// Type holds this controller type
const Type = "GatewayAPI"

const httpRoutes = "httproutes"
const GatewayAPIUpdateError = "GatewayAPIUpdateError"

type RpcPlugin struct {
	LogCtx *logrus.Entry
	Client ClientInterface
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute" protobuf:"bytes,1,name=httpRoute"`
}

type ClientInterface interface {
	Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
}

func (r *RpcPlugin) NewTrafficRouterPlugin() pluginTypes.RpcError {
	kubeConfig, err := utils.GetKubeConfig()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	gatewayAPIConfig := GatewayAPITrafficRouting{}
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugin["gatewayAPI"], &gatewayAPIConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	httpRouteName := gatewayAPIConfig.HTTPRoute
	httpRoute, err := r.Client.Get(ctx, httpRouteName, metav1.GetOptions{})
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	canaryServiceName := rollout.Spec.Strategy.Canary.CanaryService
	stableServiceName := rollout.Spec.Strategy.Canary.StableService
	rules, isFound, err := unstructured.NestedSlice(httpRoute.Object, "spec", "rules")
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	if !isFound {
		return pluginTypes.RpcError{
			ErrorString: errors.New("spec.rules field was not found in httpRoute").Error(),
		}
	}
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
	err = unstructured.SetNestedField(canaryBackendRef, int64(desiredWeight), "weight")
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	stableBackendRef, err := getBackendRef(stableServiceName, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	err = unstructured.SetNestedField(stableBackendRef, int64(100-desiredWeight), "weight")
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	rules, err = mergeBackendRefs(rules, backendRefs)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	err = unstructured.SetNestedSlice(httpRoute.Object, rules, "spec", "rules")
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	_, err = r.Client.Update(ctx, httpRoute, metav1.UpdateOptions{})
	if err != nil {
		msg := fmt.Sprintf("Error updating Gateway API %q: %s", httpRoute.GetName(), err)
		r.sendWarningEvent(GatewayAPIUpdateError, msg)
	}
	return pluginTypes.RpcError{
		ErrorString: err.Error(),
	}
}

func getBackendRef(serviceName string, backendRefs []interface{}) (map[string]interface{}, error) {
	var selectedService map[string]interface{}
	for _, service := range backendRefs {
		typedService, ok := service.(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed type assertion for gateway api service")
		}
		nameOfCurrentService, isFound, err := unstructured.NestedString(typedService, "name")
		if err != nil {
			return nil, err
		}
		if !isFound {
			continue
		}
		if nameOfCurrentService == serviceName {
			selectedService = typedService
			break
		}
	}
	if selectedService == nil {
		return nil, errors.New("service was not found in httpRoute")
	}
	return selectedService, nil
}

func getBackendRefList(rules []interface{}) ([]interface{}, error) {
	for _, rule := range rules {
		typedRule, ok := rule.(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed type assertion setting rule for http route")
		}
		backendRefs, isFound, err := unstructured.NestedSlice(typedRule, "backendRefs")
		if err != nil {
			return nil, err
		}
		if !isFound {
			continue
		}
		return backendRefs, nil
	}
	return nil, errors.New("backendRefs was not found in httpRoute")
}

func mergeBackendRefs(rules, backendRefs []interface{}) ([]interface{}, error) {
	for _, rule := range rules {
		typedRule, ok := rule.(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed type assertion setting rule for http route")
		}
		isFound, err := hasBackendRefs(typedRule)
		if err != nil {
			return nil, err
		}
		if !isFound {
			continue
		}
		err = unstructured.SetNestedSlice(typedRule, backendRefs, "backendRefs")
		if err != nil {
			return nil, err
		}
		return rules, nil
	}
	return rules, errors.New("backendRefs was not found and merged in rules")
}

func hasBackendRefs(typedRule map[string]interface{}) (bool, error) {
	_, isFound, err := unstructured.NestedSlice(typedRule, "backendRefs")
	return isFound, err
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
