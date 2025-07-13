package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
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
				ConfigMap: mocks.ConfigMapName,
			})
		err := pluginInstance.SetWeight(rollout, desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedHTTPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[0].Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedTCPRouteMock.Spec.Rules[0].BackendRefs[1].Weight))
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
			GRPCRoute: mocks.GRPCRouteName,
			ConfigMap: mocks.ConfigMapName,
		})
		err := pluginInstance.SetHeaderRoute(rollout, &headerRouting)

		assert.Empty(t, err.Error())
		assert.Equal(t, headerName, string(rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Name))
		assert.Equal(t, prefixedHeaderValue, rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Value)
		assert.Equal(t, headerValueType, *rpcPluginImp.UpdatedGRPCRouteMock.Spec.Rules[1].Matches[0].Headers[0].Type)
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

func newRollout(stableSvc, canarySvc string, config *GatewayAPITrafficRouting) *v1alpha1.Rollout {
	encodedConfig, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	return &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout",
			Namespace: mocks.RolloutNamespace,
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
