package mocks

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	HTTPRoute         = "HTTPRoute"
	TCPRoute          = "TCPRoute"
	TLSRoute          = "TLSRoute"
	StableServiceName = "argo-rollouts-stable-service"
	CanaryServiceName = "argo-rollouts-canary-service"
	PingServiceName   = "argo-rollouts-ping-service"
	PongServiceName   = "argo-rollouts-pong-service"
	HTTPRouteName     = "argo-rollouts-http-route"
	GRPCRouteName     = "argo-rollouts-grpc-route"
	TCPRouteName      = "argo-rollouts-tcp-route"
	TLSRouteName      = "argo-rollouts-tls-route"
	RolloutNamespace  = "default"
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

func CreateHTTPRouteWithLabels(name string, labels map[string]string) *gatewayv1.HTTPRoute {
	stableWeight := int32(100)
	canaryWeight := int32(0)
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
			Labels:    labels,
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
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: CanaryServiceName,
									Port: &port,
								},
								Weight: &canaryWeight,
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
}

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

func CreateGRPCRouteWithLabels(name string, labels map[string]string) *gatewayv1.GRPCRoute {
	stableWeight := int32(100)
	canaryWeight := int32(0)
	return &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
			Labels:    labels,
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
								Weight: &stableWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: CanaryServiceName,
									Port: &port,
								},
								Weight: &canaryWeight,
							},
						},
					},
				},
			},
		},
	}
}

func CreateTCPRouteWithLabels(name string, labels map[string]string) *gatewayv1.TCPRoute {
	stableWeight := int32(100)
	canaryWeight := int32(0)
	return &gatewayv1.TCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
			Labels:    labels,
		},
		Spec: gatewayv1.TCPRouteSpec{
			Rules: []gatewayv1.TCPRouteRule{
				{
					BackendRefs: []gatewayv1.BackendRef{
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: StableServiceName,
								Port: &port,
							},
							Weight: &stableWeight,
						},
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: CanaryServiceName,
								Port: &port,
							},
							Weight: &canaryWeight,
						},
					},
				},
			},
		},
	}
}

func CreateTLSRouteWithLabels(name string, labels map[string]string) *gatewayv1.TLSRoute {
	stableWeight := int32(100)
	canaryWeight := int32(0)
	return &gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
			Labels:    labels,
		},
		Spec: gatewayv1.TLSRouteSpec{
			Rules: []gatewayv1.TLSRouteRule{
				{
					BackendRefs: []gatewayv1.BackendRef{
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: StableServiceName,
								Port: &port,
							},
							Weight: &stableWeight,
						},
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: CanaryServiceName,
								Port: &port,
							},
							Weight: &canaryWeight,
						},
					},
				},
			},
		},
	}
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

var TCPPRouteObj = gatewayv1.TCPRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name:      TCPRouteName,
		Namespace: RolloutNamespace,
	},
	Spec: gatewayv1.TCPRouteSpec{
		Rules: []gatewayv1.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: StableServiceName,
							Port: &port,
						},
						Weight: &weight,
					},
					{
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
}

var TLSRouteObj = gatewayv1.TLSRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name:      TLSRouteName,
		Namespace: RolloutNamespace,
	},
	Spec: gatewayv1.TLSRouteSpec{
		Rules: []gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: StableServiceName,
							Port: &port,
						},
						Weight: &weight,
					},
					{
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
}

func CreateHTTPRouteWithPingPong(name string) *gatewayv1.HTTPRoute {
	pingWeight := int32(100)
	pongWeight := int32(0)
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: PingServiceName,
									Port: &port,
								},
								Weight: &pingWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: PongServiceName,
									Port: &port,
								},
								Weight: &pongWeight,
							},
						},
					},
				},
			},
		},
	}
}

func CreateGRPCRouteWithPingPong(name string) *gatewayv1.GRPCRoute {
	pingWeight := int32(100)
	pongWeight := int32(0)
	return &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
		},
		Spec: gatewayv1.GRPCRouteSpec{
			Rules: []gatewayv1.GRPCRouteRule{
				{
					BackendRefs: []gatewayv1.GRPCBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: PingServiceName,
									Port: &port,
								},
								Weight: &pingWeight,
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: PongServiceName,
									Port: &port,
								},
								Weight: &pongWeight,
							},
						},
					},
				},
			},
		},
	}
}

func CreateTCPRouteWithPingPong(name string) *gatewayv1.TCPRoute {
	pingWeight := int32(100)
	pongWeight := int32(0)
	return &gatewayv1.TCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
		},
		Spec: gatewayv1.TCPRouteSpec{
			Rules: []gatewayv1.TCPRouteRule{
				{
					BackendRefs: []gatewayv1.BackendRef{
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: PingServiceName,
								Port: &port,
							},
							Weight: &pingWeight,
						},
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: PongServiceName,
								Port: &port,
							},
							Weight: &pongWeight,
						},
					},
				},
			},
		},
	}
}

func CreateTLSRouteWithPingPong(name string) *gatewayv1.TLSRoute {
	pingWeight := int32(100)
	pongWeight := int32(0)
	return &gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: RolloutNamespace,
		},
		Spec: gatewayv1.TLSRouteSpec{
			Rules: []gatewayv1.TLSRouteRule{
				{
					BackendRefs: []gatewayv1.BackendRef{
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: PingServiceName,
								Port: &port,
							},
							Weight: &pingWeight,
						},
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: PongServiceName,
								Port: &port,
							},
							Weight: &pongWeight,
						},
					},
				},
			},
		},
	}
}
