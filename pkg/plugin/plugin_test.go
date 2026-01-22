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
