package plugin

import (
	"context"
	"testing"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pkg/mocks"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwFake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
)

func TestSetMirrorRouteFilterMode(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	percentage := int32(50)
	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name:       "mirror-test",
		Percentage: &percentage,
		Match: []v1alpha1.RouteMatch{
			{
				Method: &v1alpha1.StringMatch{Exact: "GET"},
			},
		},
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	})

	err := rpcPluginImp.setHTTPMirrorRouteAsFilter(rollout, &mirrorRoute, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	})
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	rule := updatedHTTP.Spec.Rules[0]
	hasMirrorFilter := false
	for _, f := range rule.Filters {
		if f.Type == gatewayv1.HTTPRouteFilterRequestMirror {
			hasMirrorFilter = true
			assert.Equal(t, gatewayv1.ObjectName(mocks.CanaryServiceName), f.RequestMirror.BackendRef.Name)
			assert.Equal(t, &percentage, f.RequestMirror.Percent)
		}
	}
	assert.True(t, hasMirrorFilter, "Weighted rule should have a RequestMirror filter")
}

func TestSetMirrorRouteFilterModeRemovesOnEmptyMatch(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	percentage := int32(100)
	addRoute := v1alpha1.SetMirrorRoute{
		Name:       "mirror-test",
		Percentage: &percentage,
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "POST"}},
		},
	}
	err := rpcPluginImp.setHTTPMirrorRouteAsFilter(rollout, &addRoute, config)
	assert.Empty(t, err.Error())

	removeRoute := v1alpha1.SetMirrorRoute{
		Name:  "mirror-test",
		Match: nil,
	}
	err = rpcPluginImp.setHTTPMirrorRouteAsFilter(rollout, &removeRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	for _, f := range updatedHTTP.Spec.Rules[0].Filters {
		assert.NotEqual(t, gatewayv1.HTTPRouteFilterRequestMirror, f.Type, "Mirror filter should be removed")
	}
}

func TestSetMirrorRouteRuleMode(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace:  mocks.RolloutNamespace,
		HTTPRoute:  mocks.HTTPRouteName,
		MirrorMode: "rule",
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-rule",
		Match: []v1alpha1.RouteMatch{
			{
				Headers: map[string]v1alpha1.StringMatch{
					"X-Test": {Exact: "true"},
				},
			},
		},
	}

	err := rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &mirrorRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Len(t, updatedHTTP.Spec.Rules, 2, "Should have original rule + managed mirror rule")

	mirrorRuleFound := false
	for _, rule := range updatedHTTP.Spec.Rules {
		if rule.Name != nil && string(*rule.Name) == "mirror-rule" {
			mirrorRuleFound = true
			hasMirrorFilter := false
			for _, f := range rule.Filters {
				if f.Type == gatewayv1.HTTPRouteFilterRequestMirror {
					hasMirrorFilter = true
					assert.Equal(t, gatewayv1.ObjectName(mocks.CanaryServiceName), f.RequestMirror.BackendRef.Name)
				}
			}
			assert.True(t, hasMirrorFilter, "Mirror rule should have RequestMirror filter")
			assert.Equal(t, gatewayv1.ObjectName(mocks.StableServiceName), rule.BackendRefs[0].Name, "Mirror rule should route to stable")
			assert.NotEmpty(t, rule.Matches, "Mirror rule should have match criteria")
		}
	}
	assert.True(t, mirrorRuleFound, "Should find the managed mirror rule")
}

func TestSetMirrorRouteRuleModeRemovesOnEmptyMatch(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace:  mocks.RolloutNamespace,
		HTTPRoute:  mocks.HTTPRouteName,
		MirrorMode: "rule",
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	addRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-rule",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "GET"}},
		},
	}
	err := rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &addRoute, config)
	assert.Empty(t, err.Error())

	removeRoute := v1alpha1.SetMirrorRoute{
		Name:  "mirror-rule",
		Match: nil,
	}
	err = rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &removeRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Len(t, updatedHTTP.Spec.Rules, 1, "Mirror rule should be removed, only original rule remains")
}

func TestSetMirrorRouteRuleModeReplacesExistingRule(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace:  mocks.RolloutNamespace,
		HTTPRoute:  mocks.HTTPRouteName,
		MirrorMode: "rule",
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	firstRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-rule",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "GET"}},
		},
	}
	err := rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &firstRoute, config)
	assert.Empty(t, err.Error())

	secondRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-rule",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "POST"}},
		},
	}
	err = rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &secondRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Len(t, updatedHTTP.Spec.Rules, 2, "Should not duplicate — old rule replaced with new one")
}

func TestSetMirrorRouteDispatchesToFilterByDefault(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-test",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "GET"}},
		},
	}

	err := rpcPluginImp.setHTTPMirrorRoute(rollout, &mirrorRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Len(t, updatedHTTP.Spec.Rules, 1, "Filter mode should not add new rules")
	hasMirrorFilter := false
	for _, f := range updatedHTTP.Spec.Rules[0].Filters {
		if f.Type == gatewayv1.HTTPRouteFilterRequestMirror {
			hasMirrorFilter = true
		}
	}
	assert.True(t, hasMirrorFilter, "Default mode should add mirror filter to existing rule")
}

func TestSetMirrorRouteDispatchesToRuleMode(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace:  mocks.RolloutNamespace,
		HTTPRoute:  mocks.HTTPRouteName,
		MirrorMode: "rule",
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-test",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "GET"}},
		},
	}

	err := rpcPluginImp.setHTTPMirrorRoute(rollout, &mirrorRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Len(t, updatedHTTP.Spec.Rules, 2, "Rule mode should add a new managed rule")
}

func TestBuildMirrorFilterWithPercentage(t *testing.T) {
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	})

	port := gatewayv1.PortNumber(80)
	stableWeight := int32(100)
	canaryWeight := int32(0)
	baseRule := gatewayv1.HTTPRouteRule{
		BackendRefs: []gatewayv1.HTTPBackendRef{
			{BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{Name: mocks.StableServiceName, Port: &port},
				Weight:                 &stableWeight,
			}},
			{BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{Name: mocks.CanaryServiceName, Port: &port},
				Weight:                 &canaryWeight,
			}},
		},
	}
	typedRule := HTTPRouteRule(baseRule)
	rules := []*HTTPRouteRule{&typedRule}

	percentage := int32(42)
	filter := buildMirrorFilter(rollout, rules, &percentage)

	assert.Equal(t, gatewayv1.HTTPRouteFilterRequestMirror, filter.Type)
	assert.Equal(t, gatewayv1.ObjectName(mocks.CanaryServiceName), filter.RequestMirror.BackendRef.Name)
	assert.Equal(t, &port, filter.RequestMirror.BackendRef.Port)
	assert.Equal(t, &percentage, filter.RequestMirror.Percent)
}

func TestBuildMirrorFilterWithoutPercentage(t *testing.T) {
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	})

	port := gatewayv1.PortNumber(80)
	stableWeight := int32(100)
	canaryWeight := int32(0)
	baseRule := gatewayv1.HTTPRouteRule{
		BackendRefs: []gatewayv1.HTTPBackendRef{
			{BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{Name: mocks.StableServiceName, Port: &port},
				Weight:                 &stableWeight,
			}},
			{BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{Name: mocks.CanaryServiceName, Port: &port},
				Weight:                 &canaryWeight,
			}},
		},
	}
	typedRule := HTTPRouteRule(baseRule)
	rules := []*HTTPRouteRule{&typedRule}

	filter := buildMirrorFilter(rollout, rules, nil)

	assert.Equal(t, gatewayv1.HTTPRouteFilterRequestMirror, filter.Type)
	assert.Nil(t, filter.RequestMirror.Percent, "Percent should be nil when no percentage specified")
}

func TestStripMirrorFilters(t *testing.T) {
	baseRule := gatewayv1.HTTPRouteRule{
		Filters: []gatewayv1.HTTPRouteFilter{
			{Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier},
			{Type: gatewayv1.HTTPRouteFilterRequestMirror},
			{Type: gatewayv1.HTTPRouteFilterURLRewrite},
			{Type: gatewayv1.HTTPRouteFilterRequestMirror},
		},
	}
	rule := HTTPRouteRule(baseRule)

	stripMirrorFilters(&rule)

	assert.Len(t, rule.Filters, 2, "Should remove all mirror filters")
	for _, f := range rule.Filters {
		assert.NotEqual(t, gatewayv1.HTTPRouteFilterRequestMirror, f.Type)
	}
}

func TestConvertRouteMatchesToHTTPRouteMatches(t *testing.T) {
	matches := []v1alpha1.RouteMatch{
		{
			Method: &v1alpha1.StringMatch{Exact: "POST"},
			Path:   &v1alpha1.StringMatch{Prefix: "/api"},
			Headers: map[string]v1alpha1.StringMatch{
				"X-Test": {Exact: "true"},
			},
		},
	}

	httpMatches := convertRouteMatchesToHTTPRouteMatches(matches)

	require.Len(t, httpMatches, 1)
	assert.Equal(t, gatewayv1.HTTPMethod("POST"), *httpMatches[0].Method)
	assert.Equal(t, gatewayv1.PathMatchPathPrefix, *httpMatches[0].Path.Type)
	assert.Equal(t, "/api", *httpMatches[0].Path.Value)
	require.Len(t, httpMatches[0].Headers, 1)
	assert.Equal(t, gatewayv1.HTTPHeaderName("X-Test"), httpMatches[0].Headers[0].Name)
}

func TestConvertRouteMatchesEmptyFields(t *testing.T) {
	matches := []v1alpha1.RouteMatch{
		{},
	}

	httpMatches := convertRouteMatchesToHTTPRouteMatches(matches)

	require.Len(t, httpMatches, 1)
	assert.Nil(t, httpMatches[0].Method)
	assert.Nil(t, httpMatches[0].Path)
	assert.Nil(t, httpMatches[0].Headers)
}

func TestSetMirrorRouteFilterModeNoPercentage(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-test",
		Match: []v1alpha1.RouteMatch{
			{Method: &v1alpha1.StringMatch{Exact: "GET"}},
		},
	}

	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	})
	config := &GatewayAPITrafficRouting{
		Namespace: mocks.RolloutNamespace,
		HTTPRoute: mocks.HTTPRouteName,
	}

	err := rpcPluginImp.setHTTPMirrorRouteAsFilter(rollout, &mirrorRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	for _, f := range updatedHTTP.Spec.Rules[0].Filters {
		if f.Type == gatewayv1.HTTPRouteFilterRequestMirror {
			assert.Nil(t, f.RequestMirror.Percent, "No percentage should mean nil Percent on filter")
		}
	}
}

func TestSetMirrorRouteRuleModeWithPathMatch(t *testing.T) {
	httpRoute := createHTTPRouteWithMatches(mocks.HTTPRouteName, nil, nil, nil, nil)

	rpcPluginImp := &RpcPlugin{
		LogCtx:              utils.SetupLog("text"),
		GatewayAPIClientset: gwFake.NewSimpleClientset(httpRoute),
	}

	config := &GatewayAPITrafficRouting{
		Namespace:  mocks.RolloutNamespace,
		HTTPRoute:  mocks.HTTPRouteName,
		MirrorMode: "rule",
	}
	rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, config)

	mirrorRoute := v1alpha1.SetMirrorRoute{
		Name: "mirror-path",
		Match: []v1alpha1.RouteMatch{
			{
				Path: &v1alpha1.StringMatch{Prefix: "/api/v2"},
			},
		},
	}

	err := rpcPluginImp.setHTTPMirrorRouteAsRule(rollout, &mirrorRoute, config)
	assert.Empty(t, err.Error())

	updatedHTTP, getErr := rpcPluginImp.GatewayAPIClientset.GatewayV1().HTTPRoutes(mocks.RolloutNamespace).Get(context.Background(), mocks.HTTPRouteName, metav1.GetOptions{})
	require.NoError(t, getErr)

	require.Len(t, updatedHTTP.Spec.Rules, 2)
	managedRule := updatedHTTP.Spec.Rules[1]
	require.NotEmpty(t, managedRule.Matches)
	assert.Equal(t, "/api/v2", *managedRule.Matches[0].Path.Value)
	assert.Equal(t, gatewayv1.PathMatchPathPrefix, *managedRule.Matches[0].Path.Type)
}
