package plugin

import (
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayApiv1alpha2 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1alpha2"
	gatewayApiv1beta1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

type RpcPlugin struct {
	IsTest               bool
	LogCtx               *logrus.Entry
	Client               *gatewayApiClientset.Clientset
	UpdatedHTTPRouteMock *v1beta1.HTTPRoute
	UpdatedTCPRouteMock  *v1alpha2.TCPRoute
	HttpRouteClient      gatewayApiv1beta1.HTTPRouteInterface
	TcpRouteClient       gatewayApiv1alpha2.TCPRouteInterface
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute,omitempty"`
	// TCPRoute refers to the name of the TCPRoute used to route traffic to the
	// service
	TCPRoute string `json:"tcpRoute,omitempty"`
	// Namespace refers to the namespace of the specified resource
	Namespace string `json:"namespace"`
}
