package plugin

import (
	"encoding/json"
	"testing"

	"github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/pkg/plugin/mocks"
	"github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/utils"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/tj/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logCtx = log.WithFields(log.Fields{"plugin": "trafficrouter"})

const (
	stableServiceName = "argo-rollouts-stable-service"
	canaryServiceName = "argo-rollouts-canary-service"
	httpRouteName     = "argo-rollouts-http-route"
)

func init() {
	utils.SetLogLevel("debug")
	log.SetFormatter(utils.CreateFormatter("text"))
}

func TestInitPlugin(t *testing.T) {
	t.Run("InitPlugin", func(t *testing.T) {
		t.Parallel()

		// Given
		rpcPluginImp := &RpcPlugin{
			LogCtx: logCtx,
		}
		rollout := newRollout(stableServiceName, canaryServiceName, httpRouteName)

		// When
		rpcErr := rpcPluginImp.InitPlugin(rollout)

		// Given
		assert.Equal(t, "", rpcErr.Error())
	})
}

func TestSetWeight(t *testing.T) {
	t.Run("SetWeight", func(t *testing.T) {
		t.Parallel()

		// Given
		rpcPluginImp := &RpcPlugin{
			LogCtx: logCtx,
			Client: &mocks.FakeClient{},
		}
		rollout := newRollout(stableServiceName, canaryServiceName, httpRouteName)

		// When
		rpcErr := rpcPluginImp.SetWeight(rollout, 30, []v1alpha1.WeightDestination{})

		// Given
		assert.Equal(t, "", rpcErr.Error())
	})
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
						Plugin: map[string]json.RawMessage{
							"argoproj-labs/gatewayAPI": encodedGatewayAPIConfig,
						},
					},
				},
			},
		},
	}
}
