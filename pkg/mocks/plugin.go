package mocks

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	HTTPRoute         = "HTTPRoute"
	TCPRoute          = "TCPRoute"
	StableServiceName = "argo-rollouts-stable-service"
	CanaryServiceName = "argo-rollouts-canary-service"
	HTTPRouteName     = "argo-rollouts-http-route"
	GRPCRouteName     = "argo-rollouts-grpc-route"
	TCPRouteName      = "argo-rollouts-tcp-route"
	RolloutNamespace  = "default"
	ConfigMapName     = "test-config"
	ManagedRouteName  = "test-header-route"
)

var (
	port                     = gatewayv1.PortNumber(80)
	weight             int32 = 0
	httpPathMatchType        = gatewayv1.PathMatchPathPrefix
	httpPathMatchValue       = "/"
	httpPathMatch            = gatewayv1.HTTPPathMatch{
		Type:  &httpPathMatchType,
		Value: &httpPathMatchValue,
	}
)

var HTTPRouteObj = gatewayv1.HTTPRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name:      HTTPRouteName,
		Namespace: RolloutNamespace,
	},
	Spec: gatewayv1.HTTPRouteSpec{
		Rules: []gatewayv1.HTTPRouteRule{
			{
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: StableServiceName,
								Port: &port,
							},
							Weight: &weight,
						},
					},
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: CanaryServiceName,
								Port: &port,
							},
							Weight: &weight,
						},
					},
				},
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &httpPathMatch,
					},
				},
			},
		},
	},
}

var GRPCRouteObj = gatewayv1.GRPCRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name:      GRPCRouteName,
		Namespace: RolloutNamespace,
	},
	Spec: gatewayv1.GRPCRouteSpec{
		Rules: []gatewayv1.GRPCRouteRule{
			{
				BackendRefs: []gatewayv1.GRPCBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: StableServiceName,
								Port: &port,
							},
							Weight: &weight,
						},
					},
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: CanaryServiceName,
								Port: &port,
							},
							Weight: &weight,
						},
					},
				},
			},
		},
	},
}

var TCPPRouteObj = v1alpha2.TCPRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name:      TCPRouteName,
		Namespace: RolloutNamespace,
	},
	Spec: v1alpha2.TCPRouteSpec{
		Rules: []v1alpha2.TCPRouteRule{
			{
				BackendRefs: []v1alpha2.BackendRef{
					{
						BackendObjectReference: v1alpha2.BackendObjectReference{
							Name: StableServiceName,
							Port: &port,
						},
						Weight: &weight,
					},
					{
						BackendObjectReference: v1alpha2.BackendObjectReference{
							Name: CanaryServiceName,
							Port: &port,
						},
						Weight: &weight,
					},
				},
			},
		},
	},
}

var ConfigMapObj = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      ConfigMapName,
		Namespace: RolloutNamespace,
	},
}
