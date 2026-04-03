package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/defaults"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pkg/mocks"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	log "github.com/sirupsen/logrus"
	gwFake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"

	goPlugin "github.com/hashicorp/go-plugin"
)

var testHandshake = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "trafficrouter",
}

func TestRunSuccessfully(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(&mocks.HTTPRouteObj).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		GRPCRouteClient: gwFake.NewSimpleClientset(&mocks.GRPCRouteObj).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TCPRouteClient:  gwFake.NewSimpleClientset(&mocks.TCPPRouteObj).GatewayV1alpha2().TCPRoutes(mocks.RolloutNamespace),
		TLSRouteClient:  gwFake.NewSimpleClientset(&mocks.TLSRouteObj).GatewayV1alpha2().TLSRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]goPlugin.Plugin{
		"RpcTrafficRouterPlugin": &rolloutsPlugin.RpcTrafficRouterPlugin{Impl: rpcPluginImp},
	}

	ch := make(chan *goPlugin.ReattachConfig, 1)
	closeCh := make(chan struct{})
	go goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: testHandshake,
		Plugins:         pluginMap,
		Test: &goPlugin.ServeTestConfig{
			Context:          ctx,
			ReattachConfigCh: ch,
			CloseCh:          closeCh,
		},
	})

	// We should get a config
	var config *goPlugin.ReattachConfig
	select {
	case config = <-ch:
	case <-time.After(2000 * time.Millisecond):
		t.Fatal("should've received reattach")
	}
	if config == nil {
		t.Fatal("config should not be nil")
	}

	// Connect!
	c := goPlugin.NewClient(&goPlugin.ClientConfig{
		Cmd:             nil,
		HandshakeConfig: testHandshake,
		Plugins:         pluginMap,
		Reattach:        config,
	})
	client, err := c.Client()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Pinging should work
	if err := client.Ping(); err != nil {
		t.Fatalf("should not err: %s", err)
	}

	// Kill which should do nothing
	c.Kill()
	if err := client.Ping(); err != nil {
		t.Fatalf("should not err: %s", err)
	}

	// Request the plugin
	raw, err := client.Dispense("RpcTrafficRouterPlugin")
	if err != nil {
		t.Fail()
	}

	pluginInstance := raw.(*rolloutsPlugin.TrafficRouterPluginRPC)
	err = pluginInstance.InitPlugin()
	if err.Error() != "" {
		t.Fail()
	}
	t.Run("SetHTTPRouteWeight", func(t *testing.T) {
		var desiredWeight int32 = 30
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
		})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
	})
	t.Run("SetHTTPRouteWeightAddsAndRemovesLabel", func(t *testing.T) {
		httpRoute := mocks.CreateHTTPRouteWithLabels(mocks.HTTPRouteName, nil)
		rpcPluginImp.HTTPRouteClient = gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace)
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
		})

		err := pluginInstance.SetWeight(rollout, 25, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels := rpcPluginImp.UpdatedHTTPRouteMock.Labels
		assert.Equal(t, defaults.InProgressLabelValue, labels[defaults.InProgressLabelKey])

		err = pluginInstance.SetWeight(rollout, 0, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels = rpcPluginImp.UpdatedHTTPRouteMock.Labels
		_, exists := labels[defaults.InProgressLabelKey]
		assert.False(t, exists)
	})
	t.Run("SetGRPCRouteWeight", func(t *testing.T) {
		var desiredWeight int32 = 30
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
		})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
	})
	t.Run("SetGRPCRouteWeightAddsAndRemovesLabel", func(t *testing.T) {
		grpcRoute := mocks.CreateGRPCRouteWithLabels(mocks.GRPCRouteName, nil)
		rpcPluginImp.GRPCRouteClient = gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace)
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
		})

		err := pluginInstance.SetWeight(rollout, 40, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels := rpcPluginImp.UpdatedGRPCRouteMock.Labels
		assert.Equal(t, defaults.InProgressLabelValue, labels[defaults.InProgressLabelKey])

		err = pluginInstance.SetWeight(rollout, 0, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels = rpcPluginImp.UpdatedGRPCRouteMock.Labels
		_, exists := labels[defaults.InProgressLabelKey]
		assert.False(t, exists)
	})
	t.Run("SetTCPRouteWeight", func(t *testing.T) {
		var desiredWeight int32 = 30
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName,
			&GatewayAPITrafficRouting{
				Namespace: mocks.RolloutNamespace,
				TCPRoute:  mocks.TCPRouteName,
			})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
	})
	t.Run("SetTCPRouteWeightAddsAndRemovesLabel", func(t *testing.T) {
		tcpRoute := mocks.CreateTCPRouteWithLabels(mocks.TCPRouteName, nil)
		rpcPluginImp.TCPRouteClient = gwFake.NewSimpleClientset(tcpRoute).GatewayV1alpha2().TCPRoutes(mocks.RolloutNamespace)
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName,
			&GatewayAPITrafficRouting{
				Namespace: mocks.RolloutNamespace,
				TCPRoute:  mocks.TCPRouteName,
			})

		err := pluginInstance.SetWeight(rollout, 15, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels := rpcPluginImp.UpdatedTCPRouteMock.Labels
		assert.Equal(t, defaults.InProgressLabelValue, labels[defaults.InProgressLabelKey])

		err = pluginInstance.SetWeight(rollout, 0, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels = rpcPluginImp.UpdatedTCPRouteMock.Labels
		_, exists := labels[defaults.InProgressLabelKey]
		assert.False(t, exists)
	})
	t.Run("SetTLSRouteWeight", func(t *testing.T) {
		var desiredWeight int32 = 30
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName,
			&GatewayAPITrafficRouting{
				Namespace: mocks.RolloutNamespace,
				TLSRoute:  mocks.TLSRouteName,
			})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedTLSRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedTLSRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
	})
	t.Run("SetTLSRouteWeightAddsAndRemovesLabel", func(t *testing.T) {
		tlsRoute := mocks.CreateTLSRouteWithLabels(mocks.TLSRouteName, nil)
		rpcPluginImp.TLSRouteClient = gwFake.NewSimpleClientset(tlsRoute).GatewayV1alpha2().TLSRoutes(mocks.RolloutNamespace)
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName,
			&GatewayAPITrafficRouting{
				Namespace: mocks.RolloutNamespace,
				TLSRoute:  mocks.TLSRouteName,
			})

		err := pluginInstance.SetWeight(rollout, 60, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels := rpcPluginImp.UpdatedTLSRouteMock.Labels
		assert.Equal(t, defaults.InProgressLabelValue, labels[defaults.InProgressLabelKey])

		err = pluginInstance.SetWeight(rollout, 0, []v1alpha1.WeightDestination{})
		assert.Empty(t, err.Error())
		labels = rpcPluginImp.UpdatedTLSRouteMock.Labels
		_, exists := labels[defaults.InProgressLabelKey]
		assert.False(t, exists)
	})
	t.Run("SetWeightViaRoutes", func(t *testing.T) {
		var desiredWeight int32 = 30
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName,
			&GatewayAPITrafficRouting{
				Namespace: mocks.RolloutNamespace,
				HTTPRoutes: []HTTPRoute{
					{
						Name:            mocks.HTTPRouteName,
						UseHeaderRoutes: true,
					},
				},
				TCPRoutes: []TCPRoute{
					{
						Name:            mocks.TCPRouteName,
						UseHeaderRoutes: true,
					},
				},
				TLSRoutes: []TLSRoute{
					{
						Name:            mocks.TLSRouteName,
						UseHeaderRoutes: true,
					},
				},
			})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedTLSRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedTLSRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
	})
	t.Run("SetHTTPHeaderRoute", func(t *testing.T) {
		headerName := "X-Test"
		headerValue := "test"
		headerValueType := gatewayv1.HeaderMatchRegularExpression
		prefixedHeaderValue := headerValue + ".*"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		assert.Equal(t, headerName, string(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches[0].Headers[0].Name))
		assert.Equal(t, prefixedHeaderValue, rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches[0].Headers[0].Value)
		assert.Equal(t, headerValueType, *rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches[0].Headers[0].Type)
	})
	t.Run("SetGRPCHeaderRoute", func(t *testing.T) {
		headerName := "X-Test"
		headerValue := "test"
		headerValueType := gatewayv1.GRPCHeaderMatchRegularExpression
		prefixedHeaderValue := headerValue + ".*"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		assert.Equal(t, headerName, string(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Name))
		assert.Equal(t, prefixedHeaderValue, rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Value)
		assert.Equal(t, headerValueType, *rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Type)
	})
	t.Run("SetGRPCHeaderRouteWithFilters", func(t *testing.T) {
		// Create a GRPCRoute mock with filters
		grpcRouteWithFilters := mocks.GRPCRouteObj
		grpcRouteWithFilters.Spec.Rules[0].Filters = []gatewayv1.GRPCRouteFilter{
			{
				Type: gatewayv1.GRPCRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{
							Name:  "X-Custom-Header",
							Value: "custom-value",
						},
					},
				},
			},
			{
				Type: gatewayv1.GRPCRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{
							Name:  "X-Response-Header",
							Value: "response-value",
						},
					},
				},
			},
		}

		// Update the plugin's GRPCRouteClient with the new mock
		rpcPluginImp.GRPCRouteClient = gwFake.NewSimpleClientset(&grpcRouteWithFilters).GatewayV1().GRPCRoutes(mocks.RolloutNamespace)

		headerName := "X-Test"
		headerValue := "test"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		// Verify that the new header route rule (index 1) has the same filters as the original route rule (index 0)
		originalFilters := grpcRouteWithFilters.Spec.Rules[0].Filters
		newRouteFilters := rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Filters

		assert.Equal(t, len(originalFilters), len(newRouteFilters), "New route should have same number of filters as original")

		// Verify first filter (RequestHeaderModifier)
		assert.Equal(t, originalFilters[0].Type, newRouteFilters[0].Type)
		assert.Equal(t, originalFilters[0].RequestHeaderModifier.Add[0].Name, newRouteFilters[0].RequestHeaderModifier.Add[0].Name)
		assert.Equal(t, originalFilters[0].RequestHeaderModifier.Add[0].Value, newRouteFilters[0].RequestHeaderModifier.Add[0].Value)

		// Verify second filter (ResponseHeaderModifier)
		assert.Equal(t, originalFilters[1].Type, newRouteFilters[1].Type)
		assert.Equal(t, originalFilters[1].ResponseHeaderModifier.Set[0].Name, newRouteFilters[1].ResponseHeaderModifier.Set[0].Name)
		assert.Equal(t, originalFilters[1].ResponseHeaderModifier.Set[0].Value, newRouteFilters[1].ResponseHeaderModifier.Set[0].Value)
	})
	t.Run("SetGRPCHeaderRouteWithoutFilters", func(t *testing.T) {
		// Create a GRPCRoute mock without filters (using the original mock which has no filters)
		grpcRouteWithoutFilters := mocks.GRPCRouteObj
		grpcRouteWithoutFilters.Spec.Rules[0].Filters = nil // Explicitly set to nil

		// Update the plugin's GRPCRouteClient with the mock without filters
		rpcPluginImp.GRPCRouteClient = gwFake.NewSimpleClientset(&grpcRouteWithoutFilters).GatewayV1().GRPCRoutes(mocks.RolloutNamespace)

		headerName := "X-Test"
		headerValue := "test"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		// Verify that the new header route rule (index 1) has no filters, same as the original route rule (index 0)
		originalFilters := grpcRouteWithoutFilters.Spec.Rules[0].Filters
		newRouteFilters := rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Filters

		assert.Nil(t, originalFilters, "Original route should have no filters")
		assert.Equal(t, len(originalFilters), len(newRouteFilters), "New route should have same number of filters as original (none)")
		assert.Empty(t, newRouteFilters, "New route should have no filters when original has none")
	})
	t.Run("SetHTTPHeaderRouteWithFilters", func(t *testing.T) {
		// Create an HTTPRoute mock with filters
		httpRouteWithFilters := mocks.HTTPRouteObj
		httpRouteWithFilters.Spec.Rules[0].Filters = []gatewayv1.HTTPRouteFilter{
			{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{
							Name:  "X-Custom-Header",
							Value: "custom-value",
						},
					},
				},
			},
			{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{
							Name:  "X-Response-Header",
							Value: "response-value",
						},
					},
				},
			},
		}

		// Update the plugin's HTTPRouteClient with the new mock
		rpcPluginImp.HTTPRouteClient = gwFake.NewSimpleClientset(&httpRouteWithFilters).GatewayV1().HTTPRoutes(mocks.RolloutNamespace)

		headerName := "X-Test"
		headerValue := "test"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		// Verify that the new header route rule (index 1) has the same filters as the original route rule (index 0)
		originalFilters := httpRouteWithFilters.Spec.Rules[0].Filters
		newRouteFilters := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Filters

		assert.Equal(t, len(originalFilters), len(newRouteFilters), "New route should have same number of filters as original")

		// Verify first filter (RequestHeaderModifier)
		assert.Equal(t, originalFilters[0].Type, newRouteFilters[0].Type)
		assert.Equal(t, originalFilters[0].RequestHeaderModifier.Add[0].Name, newRouteFilters[0].RequestHeaderModifier.Add[0].Name)
		assert.Equal(t, originalFilters[0].RequestHeaderModifier.Add[0].Value, newRouteFilters[0].RequestHeaderModifier.Add[0].Value)

		// Verify second filter (ResponseHeaderModifier)
		assert.Equal(t, originalFilters[1].Type, newRouteFilters[1].Type)
		assert.Equal(t, originalFilters[1].ResponseHeaderModifier.Set[0].Name, newRouteFilters[1].ResponseHeaderModifier.Set[0].Name)
		assert.Equal(t, originalFilters[1].ResponseHeaderModifier.Set[0].Value, newRouteFilters[1].ResponseHeaderModifier.Set[0].Value)
	})
	t.Run("SetHTTPHeaderRouteWithoutFilters", func(t *testing.T) {
		// Create an HTTPRoute mock without filters (using the original mock which has no filters)
		httpRouteWithoutFilters := mocks.HTTPRouteObj
		httpRouteWithoutFilters.Spec.Rules[0].Filters = nil // Explicitly set to nil

		// Update the plugin's HTTPRouteClient with the mock without filters
		rpcPluginImp.HTTPRouteClient = gwFake.NewSimpleClientset(&httpRouteWithoutFilters).GatewayV1().HTTPRoutes(mocks.RolloutNamespace)

		headerName := "X-Test"
		headerValue := "test"
		headerMatch := v1alpha1.StringMatch{
			Prefix: headerValue,
		}
		headerRouting := v1alpha1.SetHeaderRoute{
			Name: mocks.ManagedRouteName,
			Match: []v1alpha1.HeaderRoutingMatch{
				{
					HeaderName:  headerName,
					HeaderValue: &headerMatch,
				},
			},
		}
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		// Verify that the new header route rule (index 1) has no filters, same as the original route rule (index 0)
		originalFilters := httpRouteWithoutFilters.Spec.Rules[0].Filters
		newRouteFilters := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Filters

		assert.Nil(t, originalFilters, "Original route should have no filters")
		assert.Equal(t, len(originalFilters), len(newRouteFilters), "New route should have same number of filters as original (none)")
		assert.Empty(t, newRouteFilters, "New route should have no filters when original has none")
	})
	t.Run("RemoveHTTPManagedRoutes", func(t *testing.T) {
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			HTTPRoute: mocks.HTTPRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.RemoveManagedRoutes(rollout)

		assert.Empty(t, err.Error())
		assert.Equal(t, 1, len(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules))
	})
	t.Run("RemoveGRPCManagedRoutes", func(t *testing.T) {
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
			Namespace: mocks.RolloutNamespace,
			GRPCRoute: mocks.GRPCRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.RemoveManagedRoutes(rollout)

		assert.Empty(t, err.Error())
		assert.Equal(t, 1, len(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules))
	})

	// Canceling should cause an exit
	cancel()
	<-closeCh
}

func TestHTTPRouteWithSelector(t *testing.T) {
	// Simple test to verify selector parsing works
	config := &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRouteSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":            "test-app",
				"canary-enabled": "true",
			},
		},
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	// Just test that config parsing works with selector
	assert.NotNil(t, rollout)
	assert.NotNil(t, rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName])

	// Parse back the config to verify selector is preserved
	var parsedConfig GatewayAPITrafficRouting
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &parsedConfig)
	assert.NoError(t, err)
	assert.NotNil(t, parsedConfig.HTTPRouteSelector)
	assert.Equal(t, "test-app", parsedConfig.HTTPRouteSelector.MatchLabels["app"])
	assert.Equal(t, "true", parsedConfig.HTTPRouteSelector.MatchLabels["canary-enabled"])
}

func TestCombinedSelectorAndExplicitRoute(t *testing.T) {
	// Test that both selector and explicit route can coexist
	config := &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: "explicit-route",
		HTTPRouteSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-app",
			},
		},
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	// Parse config to verify both are preserved
	parsedConfig, err := getGatewayAPITrafficRoutingConfig(rollout)
	assert.NoError(t, err)
	assert.NotNil(t, parsedConfig.HTTPRouteSelector)
	assert.Equal(t, "explicit-route", parsedConfig.HTTPRoute)

	// After insertGatewayAPIRouteLists, we should have the explicit route in the list
	assert.Len(t, parsedConfig.HTTPRoutes, 1)
	assert.Equal(t, "explicit-route", parsedConfig.HTTPRoutes[0].Name)
}

func TestNamespaceDefaulting(t *testing.T) {
	t.Run("DefaultsToRolloutNamespaceWhenNotSpecified", func(t *testing.T) {
		// Create a rollout with namespace "my-namespace" but config without namespace
		config := &GatewayAPITrafficRouting{
			// Namespace intentionally not set (empty string)
			HTTPRoute: mocks.HTTPRouteName,
		}
		rolloutNamespace := "my-namespace"
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config, rolloutNamespace)

		// Parse the config - this is where namespace defaulting should happen
		parsedConfig, err := getGatewayAPITrafficRoutingConfig(rollout)

		assert.NoError(t, err)
		// Before the fix, this would be empty string. After the fix, it should default to rollout's namespace.
		assert.Equal(t, rolloutNamespace, parsedConfig.Namespace, "Namespace should default to rollout's namespace when not specified")
	})

	t.Run("UsesExplicitNamespaceWhenSpecified", func(t *testing.T) {
		// Create a rollout with explicit namespace in config
		explicitNamespace := "explicit-namespace"
		config := &GatewayAPITrafficRouting{
			Namespace: explicitNamespace,
			HTTPRoute: mocks.HTTPRouteName,
		}
		rolloutNamespace := "rollout-namespace"
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config, rolloutNamespace)

		// Parse the config
		parsedConfig, err := getGatewayAPITrafficRoutingConfig(rollout)

		assert.NoError(t, err)
		// Should use the explicitly specified namespace, not the rollout's namespace
		assert.Equal(t, explicitNamespace, parsedConfig.Namespace, "Should use explicit namespace when specified")
	})

	t.Run("DefaultsToRolloutNamespaceWithEmptyString", func(t *testing.T) {
		// Explicitly set namespace to empty string
		config := &GatewayAPITrafficRouting{
			Namespace: "", // Explicitly empty
			GRPCRoute: mocks.GRPCRouteName,
		}
		rolloutNamespace := "another-namespace"
		rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config, rolloutNamespace)

		parsedConfig, err := getGatewayAPITrafficRoutingConfig(rollout)

		assert.NoError(t, err)
		assert.Equal(t, rolloutNamespace, parsedConfig.Namespace, "Empty namespace should default to rollout's namespace")
	})
}

func newRollout(stableSvc, canarySvc string, config *GatewayAPITrafficRouting, namespace ...string) *v1alpha1.Rollout {
	ns := mocks.RolloutNamespace
	if len(namespace) > 0 {
		ns = namespace[0]
	}
	encodedConfig, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	return &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout",
			Namespace: ns,
		},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: stableSvc,
					CanaryService: canarySvc,
					TrafficRouting: &v1alpha1.RolloutTrafficRouting{
						ManagedRoutes: []v1alpha1.MangedRoutes{
							{
								Name: mocks.ManagedRouteName,
							},
						},
						Plugins: map[string]json.RawMessage{
							PluginName: encodedConfig,
						},
					},
				},
			},
		},
	}
}

// Helper function to create HTTPRoute with various match criteria for testing
func createHTTPRouteWithMatches(name string, headers []gatewayv1.HTTPHeaderMatch, method *gatewayv1.HTTPMethod, queryParams []gatewayv1.HTTPQueryParamMatch, path *gatewayv1.HTTPPathMatch) *gatewayv1.HTTPRoute {
	port := gatewayv1.PortNumber(80)
	stableWeight := int32(100)
	canaryWeight := int32(0)

	match := gatewayv1.HTTPRouteMatch{}
	if len(headers) > 0 {
		match.Headers = headers
	}
	if method != nil {
		match.Method = method
	}
	if len(queryParams) > 0 {
		match.QueryParams = queryParams
	}
	if path != nil {
		match.Path = path
	}

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mocks.RolloutNamespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: mocks.StableServiceName,
									Port: &port,
								},
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: mocks.CanaryServiceName,
									Port: &port,
								},
								Weight: &canaryWeight,
							},
						},
					},
					Matches: []gatewayv1.HTTPRouteMatch{match},
				},
			},
		},
	}
}

// Helper function to create GRPCRoute with various match criteria for testing
func createGRPCRouteWithMatches(name string, headers []gatewayv1.GRPCHeaderMatch, method *gatewayv1.GRPCMethodMatch) *gatewayv1.GRPCRoute {
	port := gatewayv1.PortNumber(80)
	stableWeight := int32(100)
	canaryWeight := int32(0)

	match := gatewayv1.GRPCRouteMatch{}
	if len(headers) > 0 {
		match.Headers = headers
	}
	if method != nil {
		match.Method = method
	}

	return &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mocks.RolloutNamespace,
		},
		Spec: gatewayv1.GRPCRouteSpec{
			Rules: []gatewayv1.GRPCRouteRule{
				{
					BackendRefs: []gatewayv1.GRPCBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: mocks.StableServiceName,
									Port: &port,
								},
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: mocks.CanaryServiceName,
									Port: &port,
								},
								Weight: &canaryWeight,
							},
						},
					},
					Matches: []gatewayv1.GRPCRouteMatch{match},
				},
			},
		},
	}
}

// Tests to verify that SetHeaderRoute preserves and merges existing match criteria

// TestSetHTTPHeaderRouteWithExistingHeaders verifies that when adding canary header-based routing
// to an HTTPRoute that already has header match criteria (e.g., Host header), the plugin preserves
// the original headers and merges them with the new canary headers. This ensures that existing
// routing rules based on headers continue to work alongside the new canary routing.
func TestSetHTTPHeaderRouteWithExistingHeaders(t *testing.T) {
	// Create HTTPRoute with existing header match
	existingHeaderType := gatewayv1.HeaderMatchExact
	existingHeaders := []gatewayv1.HTTPHeaderMatch{
		{
			Type:  &existingHeaderType,
			Name:  "Host",
			Value: "example.com",
		},
	}
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, existingHeaders, nil, nil, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes BOTH original header AND canary header
	managedRouteMatches := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify headers are merged correctly
	if len(managedRouteMatches) > 0 && len(managedRouteMatches[0].Headers) > 0 {
		headers := managedRouteMatches[0].Headers
		assert.Equal(t, 2, len(headers), "Should have both original and canary headers")

		// Check if both headers are present
		hasOriginalHeader := false
		hasCanaryHeader := false
		for _, h := range headers {
			if string(h.Name) == "Host" && h.Value == "example.com" {
				hasOriginalHeader = true
			}
			if string(h.Name) == canaryHeaderName {
				hasCanaryHeader = true
			}
		}
		assert.True(t, hasOriginalHeader, "Original Host header should be preserved")
		assert.True(t, hasCanaryHeader, "Canary header should be added")
	} else {
		t.Fatal("Managed route should have at least one match with headers")
	}
}

// TestSetHTTPHeaderRouteWithExistingMethod verifies that when adding canary header-based routing
// to an HTTPRoute that already has HTTP method match criteria (e.g., POST), the plugin preserves
// the original method specification. This ensures method-based routing continues to function
// correctly when canary headers are added for progressive delivery.
func TestSetHTTPHeaderRouteWithExistingMethod(t *testing.T) {
	// Create HTTPRoute with existing method match
	postMethod := gatewayv1.HTTPMethodPost
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, &postMethod, nil, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes BOTH method AND canary header
	managedRouteMatches := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify method and headers are both preserved
	if len(managedRouteMatches) > 0 {
		match := managedRouteMatches[0]
		assert.NotNil(t, match.Method, "Method should be preserved")
		if match.Method != nil {
			assert.Equal(t, postMethod, *match.Method, "POST method should be preserved")
		}
		assert.NotEmpty(t, match.Headers, "Canary headers should be present")
	} else {
		t.Fatal("Managed route should have at least one match")
	}
}

// TestSetHTTPHeaderRouteWithExistingQueryParams verifies that when adding canary header-based
// routing to an HTTPRoute that already has query parameter match criteria (e.g., version=v2),
// the plugin preserves the original query parameter specifications. This ensures query
// parameter-based routing continues to work correctly alongside canary header routing.
func TestSetHTTPHeaderRouteWithExistingQueryParams(t *testing.T) {
	// Create HTTPRoute with existing query param matches
	queryParamType := gatewayv1.QueryParamMatchExact
	queryParams := []gatewayv1.HTTPQueryParamMatch{
		{
			Type:  &queryParamType,
			Name:  "version",
			Value: "v2",
		},
	}
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, queryParams, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes BOTH query params AND canary header
	managedRouteMatches := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify query params and headers are both preserved
	if len(managedRouteMatches) > 0 {
		match := managedRouteMatches[0]
		assert.NotEmpty(t, match.QueryParams, "Query params should be preserved")
		if len(match.QueryParams) > 0 {
			assert.Equal(t, "version", string(match.QueryParams[0].Name))
			assert.Equal(t, "v2", match.QueryParams[0].Value)
		}
		assert.NotEmpty(t, match.Headers, "Canary headers should be present")
	} else {
		t.Fatal("Managed route should have at least one match")
	}
}

// TestSetHTTPHeaderRouteWithMultipleExistingHeaders verifies that when adding canary header-based
// routing to an HTTPRoute that already has multiple header match criteria (e.g., Host and User-Agent),
// the plugin preserves all original headers and merges them with the new canary headers. This ensures
// complex header-based routing rules continue to work when implementing progressive delivery.
func TestSetHTTPHeaderRouteWithMultipleExistingHeaders(t *testing.T) {
	// Create HTTPRoute with multiple existing header matches
	exactType := gatewayv1.HeaderMatchExact
	regexType := gatewayv1.HeaderMatchRegularExpression
	existingHeaders := []gatewayv1.HTTPHeaderMatch{
		{
			Type:  &exactType,
			Name:  "Host",
			Value: "example.com",
		},
		{
			Type:  &regexType,
			Name:  "User-Agent",
			Value: ".*Mobile.*",
		},
	}
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, existingHeaders, nil, nil, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes ALL original headers plus canary header
	managedRouteMatches := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify all headers are merged correctly
	if len(managedRouteMatches) > 0 && len(managedRouteMatches[0].Headers) > 0 {
		headers := managedRouteMatches[0].Headers
		assert.Equal(t, 3, len(headers), "Should have 2 original headers plus 1 canary header")

		// Check for all three headers
		hasHostHeader := false
		hasUserAgentHeader := false
		hasCanaryHeader := false
		for _, h := range headers {
			if string(h.Name) == "Host" {
				hasHostHeader = true
			}
			if string(h.Name) == "User-Agent" {
				hasUserAgentHeader = true
			}
			if string(h.Name) == canaryHeaderName {
				hasCanaryHeader = true
			}
		}
		assert.True(t, hasHostHeader, "Original Host header should be preserved")
		assert.True(t, hasUserAgentHeader, "Original User-Agent header should be preserved")
		assert.True(t, hasCanaryHeader, "Canary header should be added")
	} else {
		t.Fatal("Managed route should have at least one match with headers")
	}
}

// TestSetHTTPHeaderRouteWithCombinedMatches verifies that when adding canary header-based routing
// to an HTTPRoute that has multiple types of match criteria (headers, HTTP method, query parameters,
// and path), the plugin preserves all original match criteria and merges headers appropriately. This
// ensures comprehensive routing rules with multiple conditions continue to work during canary deployments.
func TestSetHTTPHeaderRouteWithCombinedMatches(t *testing.T) {
	// Create HTTPRoute with headers, method, query params, and path
	exactType := gatewayv1.HeaderMatchExact
	existingHeaders := []gatewayv1.HTTPHeaderMatch{
		{
			Type:  &exactType,
			Name:  "Host",
			Value: "example.com",
		},
	}
	postMethod := gatewayv1.HTTPMethodPost
	queryParamType := gatewayv1.QueryParamMatchExact
	queryParams := []gatewayv1.HTTPQueryParamMatch{
		{
			Type:  &queryParamType,
			Name:  "version",
			Value: "v2",
		},
	}
	pathType := gatewayv1.PathMatchPathPrefix
	pathValue := "/api"
	path := &gatewayv1.HTTPPathMatch{
		Type:  &pathType,
		Value: &pathValue,
	}

	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, existingHeaders, &postMethod, queryParams, path)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes ALL original match criteria plus canary header
	managedRouteMatches := rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify all match criteria are preserved
	if len(managedRouteMatches) > 0 {
		match := managedRouteMatches[0]

		// Check headers (should have both original and canary)
		assert.NotEmpty(t, match.Headers, "Headers should be present")
		if len(match.Headers) > 0 {
			assert.Equal(t, 2, len(match.Headers), "Should have original Host header and canary header")
		}

		// Check method
		assert.NotNil(t, match.Method, "Method should be preserved")
		if match.Method != nil {
			assert.Equal(t, postMethod, *match.Method, "POST method should be preserved")
		}

		// Check query params
		assert.NotEmpty(t, match.QueryParams, "Query params should be preserved")
		if len(match.QueryParams) > 0 {
			assert.Equal(t, "version", string(match.QueryParams[0].Name))
			assert.Equal(t, "v2", match.QueryParams[0].Value)
		}

		// Check path
		assert.NotNil(t, match.Path, "Path should be preserved")
		if match.Path != nil {
			assert.Equal(t, "/api", *match.Path.Value)
		}
	} else {
		t.Fatal("Managed route should have at least one match")
	}
}

// TestSetGRPCHeaderRouteWithExistingHeaders verifies that when adding canary header-based routing
// to a GRPCRoute that already has header match criteria (e.g., Host header), the plugin preserves
// the original headers and merges them with the new canary headers. This ensures that existing
// gRPC routing rules based on headers continue to work alongside the new canary routing.
func TestSetGRPCHeaderRouteWithExistingHeaders(t *testing.T) {
	// Create GRPCRoute with existing header match
	exactType := gatewayv1.GRPCHeaderMatchExact
	existingHeaders := []gatewayv1.GRPCHeaderMatch{
		{
			Type:  &exactType,
			Name:  "Host",
			Value: "example.com",
		},
	}
	grpcRoute := createGRPCRouteWithMatches(mocks.GRPCRouteName, existingHeaders, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		GRPCRouteClient: gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		GRPCRoute: mocks.GRPCRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes BOTH original header AND canary header
	managedRouteMatches := rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify headers are merged correctly
	if len(managedRouteMatches) > 0 && len(managedRouteMatches[0].Headers) > 0 {
		headers := managedRouteMatches[0].Headers
		assert.Equal(t, 2, len(headers), "Should have both original and canary headers")

		// Check if both headers are present
		hasOriginalHeader := false
		hasCanaryHeader := false
		for _, h := range headers {
			if string(h.Name) == "Host" && h.Value == "example.com" {
				hasOriginalHeader = true
			}
			if string(h.Name) == canaryHeaderName {
				hasCanaryHeader = true
			}
		}
		assert.True(t, hasOriginalHeader, "Original Host header should be preserved")
		assert.True(t, hasCanaryHeader, "Canary header should be added")
	} else {
		t.Fatal("Managed route should have at least one match with headers")
	}
}

// TestSetGRPCHeaderRouteWithMultipleExistingHeaders verifies that when adding canary header-based
// routing to a GRPCRoute that already has multiple header match criteria (e.g., Host and User-Agent),
// the plugin preserves all original headers and merges them with the new canary headers. This ensures
// complex gRPC header-based routing rules continue to work when implementing progressive delivery.
func TestSetGRPCHeaderRouteWithMultipleExistingHeaders(t *testing.T) {
	// Create GRPCRoute with multiple existing header matches
	exactType := gatewayv1.GRPCHeaderMatchExact
	regexType := gatewayv1.GRPCHeaderMatchRegularExpression
	existingHeaders := []gatewayv1.GRPCHeaderMatch{
		{
			Type:  &exactType,
			Name:  "Host",
			Value: "example.com",
		},
		{
			Type:  &regexType,
			Name:  "User-Agent",
			Value: ".*gRPC.*",
		},
	}
	grpcRoute := createGRPCRouteWithMatches(mocks.GRPCRouteName, existingHeaders, nil)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		GRPCRouteClient: gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		GRPCRoute: mocks.GRPCRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes ALL original headers plus canary header
	managedRouteMatches := rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify all headers are merged correctly
	if len(managedRouteMatches) > 0 && len(managedRouteMatches[0].Headers) > 0 {
		headers := managedRouteMatches[0].Headers
		assert.Equal(t, 3, len(headers), "Should have 2 original headers plus 1 canary header")

		// Check for all three headers
		hasHostHeader := false
		hasUserAgentHeader := false
		hasCanaryHeader := false
		for _, h := range headers {
			if string(h.Name) == "Host" {
				hasHostHeader = true
			}
			if string(h.Name) == "User-Agent" {
				hasUserAgentHeader = true
			}
			if string(h.Name) == canaryHeaderName {
				hasCanaryHeader = true
			}
		}
		assert.True(t, hasHostHeader, "Original Host header should be preserved")
		assert.True(t, hasUserAgentHeader, "Original User-Agent header should be preserved")
		assert.True(t, hasCanaryHeader, "Canary header should be added")
	} else {
		t.Fatal("Managed route should have at least one match with headers")
	}
}

// TestSetGRPCHeaderRouteWithMethodAndHeaders verifies that when adding canary header-based routing
// to a GRPCRoute that already has both gRPC method match criteria (service/method) and header match
// criteria, the plugin preserves the method specification and merges the headers appropriately. This
// ensures method-specific gRPC routing with header conditions continues to work during canary deployments.
func TestSetGRPCHeaderRouteWithMethodAndHeaders(t *testing.T) {
	// Create GRPCRoute with method match and header matches
	exactType := gatewayv1.GRPCHeaderMatchExact
	existingHeaders := []gatewayv1.GRPCHeaderMatch{
		{
			Type:  &exactType,
			Name:  "Host",
			Value: "example.com",
		},
	}
	methodMatchType := gatewayv1.GRPCMethodMatchExact
	service := "my.service.v1.MyService"
	method := "GetUser"
	grpcMethod := &gatewayv1.GRPCMethodMatch{
		Type:    &methodMatchType,
		Service: &service,
		Method:  &method,
	}
	grpcRoute := createGRPCRouteWithMatches(mocks.GRPCRouteName, existingHeaders, grpcMethod)

	// Setup plugin
	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		GRPCRouteClient: gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TestClientset:   fake.NewSimpleClientset(&mocks.ConfigMapObj).CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	// Setup canary header
	canaryHeaderName := "X-Canary"
	canaryHeaderValue := "true"
	headerMatch := v1alpha1.StringMatch{
		Exact: canaryHeaderValue,
	}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  canaryHeaderName,
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		GRPCRoute: mocks.GRPCRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// Call SetHeaderRoute
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())

	// Verify that managed route (index 1) includes method AND all headers (original + canary)
	managedRouteMatches := rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches
	assert.NotEmpty(t, managedRouteMatches, "Managed route should have matches")

	// Verify method and headers are both preserved
	if len(managedRouteMatches) > 0 {
		match := managedRouteMatches[0]

		// Check method
		assert.NotNil(t, match.Method, "Method should be preserved")
		if match.Method != nil {
			assert.NotNil(t, match.Method.Service, "Service should be preserved")
			if match.Method.Service != nil {
				assert.Equal(t, service, *match.Method.Service)
			}
			assert.NotNil(t, match.Method.Method, "Method name should be preserved")
			if match.Method.Method != nil {
				assert.Equal(t, method, *match.Method.Method)
			}
		}

		// Check headers (should have both original and canary)
		assert.NotEmpty(t, match.Headers, "Headers should be present")
		if len(match.Headers) > 0 {
			assert.Equal(t, 2, len(match.Headers), "Should have original Host header and canary header")
			hasOriginalHeader := false
			hasCanaryHeader := false
			for _, h := range match.Headers {
				if string(h.Name) == "Host" {
					hasOriginalHeader = true
				}
				if string(h.Name) == canaryHeaderName {
					hasCanaryHeader = true
				}
			}
			assert.True(t, hasOriginalHeader, "Original Host header should be preserved")
			assert.True(t, hasCanaryHeader, "Canary header should be added")
		}
	} else {
		t.Fatal("Managed route should have at least one match")
	}
}

// TestSetHTTPHeaderRouteNoDuplicateOnRepeatedCall verifies that calling SetHeaderRoute multiple
// times for the same managed route name does not create duplicate rules in the HTTPRoute.
// This covers the bug reported in issue #151 where rapid consecutive deployments caused
// orphaned duplicate header-based routing rules.
func TestSetHTTPHeaderRouteNoDuplicateOnRepeatedCall(t *testing.T) {
	httpRoute := mocks.CreateHTTPRouteWithLabels(mocks.HTTPRouteName, nil)
	configMapClientset := fake.NewSimpleClientset(&mocks.ConfigMapObj)

	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   configMapClientset.CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	headerMatch := v1alpha1.StringMatch{Exact: "true"}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  "X-Canary",
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// First call — should add one managed rule (total: 2 rules)
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules), "first call should add exactly one managed rule")

	// Simulate the state the fake client has after the first update
	rpcPluginImp.HTTPRouteClient = gwFake.NewSimpleClientset(rpcPluginImp.UpdatedHTTPRouteMock).GatewayV1().HTTPRoutes(mocks.RolloutNamespace)

	// Second call with the same header route name — should update in place, not append
	err = rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules), "second call must not add a duplicate managed rule")
}

// TestSetGRPCHeaderRouteNoDuplicateOnRepeatedCall verifies that calling SetHeaderRoute multiple
// times for the same managed route name does not create duplicate rules in the GRPCRoute.
func TestSetGRPCHeaderRouteNoDuplicateOnRepeatedCall(t *testing.T) {
	grpcRoute := mocks.CreateGRPCRouteWithLabels(mocks.GRPCRouteName, nil)
	configMapClientset := fake.NewSimpleClientset(&mocks.ConfigMapObj)

	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		GRPCRouteClient: gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TestClientset:   configMapClientset.CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	headerMatch := v1alpha1.StringMatch{Exact: "true"}
	headerRouting := v1alpha1.SetHeaderRoute{
		Name: mocks.ManagedRouteName,
		Match: []v1alpha1.HeaderRoutingMatch{
			{
				HeaderName:  "X-Canary",
				HeaderValue: &headerMatch,
			},
		},
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		GRPCRoute: mocks.GRPCRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	// First call — should add one managed rule (total: 2 rules)
	err := rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules), "first call should add exactly one managed rule")

	// Simulate the state the fake client has after the first update
	rpcPluginImp.GRPCRouteClient = gwFake.NewSimpleClientset(rpcPluginImp.UpdatedGRPCRouteMock).GatewayV1().GRPCRoutes(mocks.RolloutNamespace)

	// Second call with the same header route name — should update in place, not append
	err = rpcPluginImp.SetHeaderRoute(rollout, &headerRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules), "second call must not add a duplicate managed rule")
}

// TestSetHTTPHeaderRouteTwoDistinctNamesAppendsBoth verifies that adding two managed header
// routes with different names results in both being appended (3 rules total: 1 base + 2 managed).
func TestSetHTTPHeaderRouteTwoDistinctNamesAppendsBoth(t *testing.T) {
	httpRoute := mocks.CreateHTTPRouteWithLabels(mocks.HTTPRouteName, nil)
	configMapClientset := fake.NewSimpleClientset(&mocks.ConfigMapObj)

	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		HTTPRouteClient: gwFake.NewSimpleClientset(httpRoute).GatewayV1().HTTPRoutes(mocks.RolloutNamespace),
		TestClientset:   configMapClientset.CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	firstMatch := v1alpha1.StringMatch{Exact: "true"}
	firstHeaderRouting := v1alpha1.SetHeaderRoute{
		Name: "header-route-one",
		Match: []v1alpha1.HeaderRoutingMatch{
			{HeaderName: "X-Canary", HeaderValue: &firstMatch},
		},
	}

	secondMatch := v1alpha1.StringMatch{Exact: "beta"}
	secondHeaderRouting := v1alpha1.SetHeaderRoute{
		Name: "header-route-two",
		Match: []v1alpha1.HeaderRoutingMatch{
			{HeaderName: "X-Version", HeaderValue: &secondMatch},
		},
	}

	// Add the first managed route
	err := rpcPluginImp.SetHeaderRoute(rollout, &firstHeaderRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules), "first call should add one managed rule")

	// Reflect the updated route in the fake client before the second call
	rpcPluginImp.HTTPRouteClient = gwFake.NewSimpleClientset(rpcPluginImp.UpdatedHTTPRouteMock).GatewayV1().HTTPRoutes(mocks.RolloutNamespace)

	// Add a second managed route with a different name — must be appended, not replace the first
	err = rpcPluginImp.SetHeaderRoute(rollout, &secondHeaderRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 3, len(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules), "second distinct header route must be appended")
}

// TestSetGRPCHeaderRouteTwoDistinctNamesAppendsBoth verifies that adding two managed header
// routes with different names results in both being appended (3 rules total: 1 base + 2 managed).
func TestSetGRPCHeaderRouteTwoDistinctNamesAppendsBoth(t *testing.T) {
	grpcRoute := mocks.CreateGRPCRouteWithLabels(mocks.GRPCRouteName, nil)
	configMapClientset := fake.NewSimpleClientset(&mocks.ConfigMapObj)

	rpcPluginImp := &RpcPlugin{
		LogCtx:          utils.SetupLog(),
		IsTest:          true,
		GRPCRouteClient: gwFake.NewSimpleClientset(grpcRoute).GatewayV1().GRPCRoutes(mocks.RolloutNamespace),
		TestClientset:   configMapClientset.CoreV1().ConfigMaps(mocks.RolloutNamespace),
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		GRPCRoute: mocks.GRPCRouteName,
		ConfigMap: mocks.ConfigMapName,
	})

	firstMatch := v1alpha1.StringMatch{Exact: "true"}
	firstHeaderRouting := v1alpha1.SetHeaderRoute{
		Name: "header-route-one",
		Match: []v1alpha1.HeaderRoutingMatch{
			{HeaderName: "X-Canary", HeaderValue: &firstMatch},
		},
	}

	secondMatch := v1alpha1.StringMatch{Exact: "beta"}
	secondHeaderRouting := v1alpha1.SetHeaderRoute{
		Name: "header-route-two",
		Match: []v1alpha1.HeaderRoutingMatch{
			{HeaderName: "X-Version", HeaderValue: &secondMatch},
		},
	}

	// Add the first managed route
	err := rpcPluginImp.SetHeaderRoute(rollout, &firstHeaderRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 2, len(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules), "first call should add one managed rule")

	// Reflect the updated route in the fake client before the second call
	rpcPluginImp.GRPCRouteClient = gwFake.NewSimpleClientset(rpcPluginImp.UpdatedGRPCRouteMock).GatewayV1().GRPCRoutes(mocks.RolloutNamespace)

	// Add a second managed route with a different name — must be appended, not replace the first
	err = rpcPluginImp.SetHeaderRoute(rollout, &secondHeaderRouting)
	assert.Empty(t, err.Error())
	assert.Equal(t, 3, len(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules), "second distinct header route must be appended")
}
