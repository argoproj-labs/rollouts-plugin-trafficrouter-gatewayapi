package plugin

import (
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayAPIClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayApiClientv1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1"
	gatewayApiClientv1alpha2 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1alpha2"
)

type CommandLineOpts struct {
	KubeClientQPS   float32
	KubeClientBurst int
}

type RpcPlugin struct {
	CommandLineOpts      CommandLineOpts
	HTTPRouteClient      gatewayApiClientv1.HTTPRouteInterface
	TCPRouteClient       gatewayApiClientv1alpha2.TCPRouteInterface
	GRPCRouteClient      gatewayApiClientv1.GRPCRouteInterface
	TestClientset        v1.ConfigMapInterface
	GatewayAPIClientset  *gatewayAPIClientset.Clientset
	Clientset            *kubernetes.Clientset
	UpdatedHTTPRouteMock *gatewayv1.HTTPRoute
	UpdatedTCPRouteMock  *v1alpha2.TCPRoute
	UpdatedGRPCRouteMock *gatewayv1.GRPCRoute
	LogCtx               *logrus.Entry
	IsTest               bool
}

type GatewayAPITrafficRouting struct {
	// HTTPRoute refers to the name of the HTTPRoute used to route traffic to the
	// service
	HTTPRoute string `json:"httpRoute,omitempty"`
	// GRPCRoute refers to the name of the GRPCRoute used to route traffic to the
	// service
	GRPCRoute string `json:"grpcRoute,omitempty"`
	// TCPRoute refers to the name of the TCPRoute used to route traffic to the
	// service
	TCPRoute string `json:"tcpRoute,omitempty"`
	// Namespace refers to the namespace of the specified resource
	Namespace string `json:"namespace,omitempty"`
	// ConfigMap refers to the config map where plugin stores data about managed routes
	ConfigMap string `json:"configMap,omitempty"`
	// HTTPRoutes refer to names of HTTPRoute resources used to route traffic to the
	// service
	HTTPRoutes []HTTPRoute `json:"httpRoutes,omitempty"`
	// TCPRoutes refer to names of TCPRoute resources used to route traffic to the
	// service
	TCPRoutes []TCPRoute `json:"tcpRoutes,omitempty"`
	// GRPCRoutes refer to names of GRPCRoute resources used to route traffic to the
	// service
	GRPCRoutes []GRPCRoute `json:"grpcRoutes,omitempty"`
	// ConfigMapRWMutex refers to the RWMutex that we use to enter to the critical section
	// critical section is config map
	ConfigMapRWMutex sync.RWMutex
}

type HTTPRoute struct {
	// Name refers to the HTTPRoute name
	Name string `json:"name" validate:"required"`
	// UseHeaderRoutes defines header routes will be added to this route or not
	// during setHeaderRoute step
	UseHeaderRoutes bool `json:"useHeaderRoutes,omitempty"`
}

type TCPRoute struct {
	// Name refers to the TCPRoute name
	Name string `json:"name" validate:"required"`
	// UseHeaderRoutes indicates header routes will be added to this route or not
	// during setHeaderRoute step
	UseHeaderRoutes bool `json:"useHeaderRoutes"`
}

type GRPCRoute struct {
	// Name refers to the GRPCRoute name
	Name string `json:"name" validate:"required"`
	// UseHeaderRoutes indicates header routes will be added to this route or not
	// during setHeaderRoute step
	UseHeaderRoutes bool `json:"useHeaderRoutes"`
}

type ManagedRouteMap map[string]map[string]int

type HTTPRouteRule gatewayv1.HTTPRouteRule

type GRPCRouteRule gatewayv1.GRPCRouteRule

type TCPRouteRule v1alpha2.TCPRouteRule

type HTTPRouteRuleList []gatewayv1.HTTPRouteRule

type GRPCRouteRuleList []gatewayv1.GRPCRouteRule

type TCPRouteRuleList []v1alpha2.TCPRouteRule

type HTTPBackendRef gatewayv1.HTTPBackendRef

type GRPCBackendRef gatewayv1.GRPCBackendRef

type TCPBackendRef gatewayv1.BackendRef

type GatewayAPIRoute interface {
	HTTPRoute | GRPCRoute | TCPRoute
	GetName() string
}

type GatewayAPIRouteRule[T1 GatewayAPIBackendRef] interface {
	*HTTPRouteRule | *GRPCRouteRule | *TCPRouteRule
	Iterator() (GatewayAPIRouteRuleIterator[T1], bool)
}

type GatewayAPIRouteRuleList[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] interface {
	HTTPRouteRuleList | GRPCRouteRuleList | TCPRouteRuleList
	Iterator() (GatewayAPIRouteRuleListIterator[T1, T2], bool)
	Error() error
}

type GatewayAPIBackendRef interface {
	*HTTPBackendRef | *GRPCBackendRef | *TCPBackendRef
	GetName() string
}

type GatewayAPIRouteRuleListIterator[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] func() (T2, bool)

type GatewayAPIRouteRuleIterator[T1 GatewayAPIBackendRef] func() (T1, bool)

type IndexedBackendRefs[T GatewayAPIBackendRef] struct {
	RuleIndex int
	Refs      []T
}
