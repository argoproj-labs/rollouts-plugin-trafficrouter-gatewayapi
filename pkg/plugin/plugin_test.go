package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pkg/mocks"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	logCtx := log.WithFields(log.Fields{"plugin": "trafficrouter"})

	utils.SetLogLevel("debug")
	log.SetFormatter(utils.CreateFormatter("text"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rpcPluginImp := &RpcPlugin{
		LogCtx:          logCtx,
		IsTest:          true,
		HttpRouteClient: gwFake.NewSimpleClientset(&(mocks.HttpRouteObj)).GatewayV1beta1().HTTPRoutes("default"),
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
	t.Run("SetWeight", func(t *testing.T) {
		var desiredWeight int32 = 30
		err := pluginInstance.SetWeight(newRollout(mocks.StableServiceName, mocks.CanaryServiceName, mocks.HttpRouteName), desiredWeight, []v1alpha1.WeightDestination{})

		assert.Empty(t, err.Error())
		assert.Equal(t, 100-desiredWeight, *(rpcPluginImp.UpdatedMockHttpRoute.Spec.Rules[0].BackendRefs[0].BackendRef.Weight))
		assert.Equal(t, desiredWeight, *(rpcPluginImp.UpdatedMockHttpRoute.Spec.Rules[0].BackendRefs[1].BackendRef.Weight))
	})

	// Canceling should cause an exit
	cancel()
	<-closeCh
}

func newRollout(stableSvc, canarySvc, httpRouteName string) *v1alpha1.Rollout {
	gatewayAPIConfig := GatewayAPITrafficRouting{
		HTTPRoute: httpRouteName,
		Namespace: "default",
	}
	encodedGatewayAPIConfig, err := json.Marshal(gatewayAPIConfig)
	if err != nil {
		log.Fatal(err)
	}
	return &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout",
			Namespace: "default",
		},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: stableSvc,
					CanaryService: canarySvc,
					TrafficRouting: &v1alpha1.RolloutTrafficRouting{
						Plugins: map[string]json.RawMessage{
							"argoproj-labs/gatewayAPI": encodedGatewayAPIConfig,
						},
					},
				},
			},
		},
	}
}
