package plugin

import (
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayApiv1alpha2 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1alpha2"
	gatewayApiv1beta1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

const (
	HTTPRouteKind = "HTTPRoute"
	TCPRouteKind  = "TCPRoute"
)

type RpcPlugin struct {
	IsTest               bool
	LogCtx               *logrus.Entry
	Client               *gatewayApiClientset.Clientset
	UpdatedHTTPRouteMock *v1beta1.HTTPRoute
	UpdatedTCPRouteMock  *v1alpha2.TCPRoute
	HTTPRouteClient      gatewayApiv1beta1.HTTPRouteInterface
	TCPRouteClient       gatewayApiv1alpha2.TCPRouteInterface
}

type GatewayAPIRouteReference struct {
	// Kind is kind of the route resource. For example "HTTPRoute" or "TCPRoute".
	Kind string
	// Name is the name of the route resource.
	Name string
}

type GatewayAPITrafficRouting struct {
	// Routes refers to HTTPRoute and TCPRoute resources used to route traffic
	// to the service
	Routes []GatewayAPIRouteReference `json:"routes,omitempty"`
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute,omitempty"`
	// TCPRoute refers to the name of the TCPRoute used to route traffic to the
	// service
	TCPRoute string `json:"tcpRoute,omitempty"`
	// Namespace refers to the namespace of the specified resource
	Namespace string `json:"namespace"`
}

type HTTPBackendRef v1beta1.HTTPBackendRef

type TCPBackendRef v1beta1.BackendRef

type HTTPRouteRuleList []v1beta1.HTTPRouteRule

type TCPRouteRuleList []v1alpha2.TCPRouteRule

type HTTPBackendRefList []v1beta1.HTTPBackendRef

type TCPBackendRefList []v1beta1.BackendRef

type GatewayAPIBackendRef interface {
	*HTTPBackendRef | *TCPBackendRef
	GetName() string
}

type GatewayAPIBackendRefList interface {
	HTTPBackendRefList | TCPBackendRefList
}

type GatewayAPIRouteRuleCollection[T GatewayAPIBackendRefList] interface {
	Iterator() (GatewayAPIRouteRuleIterator[T], bool)
	Error() error
}

type GatewayAPIBackendRefCollection[T GatewayAPIBackendRef] interface {
	Iterator() (GatewayAPIBackendRefIterator[T], bool)
	Error() error
}

type GatewayAPIRouteRuleIterator[T GatewayAPIBackendRefList] func() (T, bool)

type GatewayAPIBackendRefIterator[T GatewayAPIBackendRef] func() (T, bool)
