package plugin

import (
	"sync"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	TLSRouteClient       gatewayApiClientv1alpha2.TLSRouteInterface
	TestClientset        v1.ConfigMapInterface
	GatewayAPIClientset  *gatewayAPIClientset.Clientset
	Clientset            *kubernetes.Clientset
	UpdatedHTTPRouteMock *gatewayv1.HTTPRoute
	UpdatedTCPRouteMock  *v1alpha2.TCPRoute
	UpdatedGRPCRouteMock *gatewayv1.GRPCRoute
	UpdatedTLSRouteMock  *v1alpha2.TLSRoute
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
	// TLSRoute refers to the name of the TLSRoute used to route traffic to the
	// service
	TLSRoute string `json:"tlsRoute,omitempty"`
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
	// TLSRoutes refer to names of TLSRoute resources used to route traffic to the
	// service
	TLSRoutes []TLSRoute `json:"tlsRoutes,omitempty"`
	// HTTPRouteSelector refers to label selector for auto-discovery of HTTPRoutes
	HTTPRouteSelector *metav1.LabelSelector `json:"httpRouteSelector,omitempty"`
	// GRPCRouteSelector refers to label selector for auto-discovery of GRPCRoutes
	GRPCRouteSelector *metav1.LabelSelector `json:"grpcRouteSelector,omitempty"`
	// TCPRouteSelector refers to label selector for auto-discovery of TCPRoutes
	TCPRouteSelector *metav1.LabelSelector `json:"tcpRouteSelector,omitempty"`
	// TLSRouteSelector refers to label selector for auto-discovery of TLSRoutes
	TLSRouteSelector *metav1.LabelSelector `json:"tlsRouteSelector,omitempty"`
	// DisableInProgressLabel disables the automatic label that marks routes as managed during canary steps
	DisableInProgressLabel bool `json:"disableInProgressLabel,omitempty"`
	// InProgressLabelKey overrides the label key used while a canary is running
	InProgressLabelKey string `json:"inProgressLabelKey,omitempty"`
	// InProgressLabelValue overrides the label value used while a canary is running
	InProgressLabelValue string `json:"inProgressLabelValue,omitempty"`
	// SkipManagedRoutesOnSetWeight controls whether setWeight should skip managed routes (header routes).
	// When true, setWeight will not modify the weight of routes created by setHeaderRoute,
	// ensuring those routes always send 100% traffic to canary.
	// Default is false to maintain backward compatibility.
	SkipManagedRoutesOnSetWeight bool `json:"skipManagedRoutesOnSetWeight,omitempty"`
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

type TLSRoute struct {
	// Name refers to the TLSRoute name
	Name string `json:"name" validate:"required"`
	// UseHeaderRoutes indicates header routes will be added to this route or not
	// during setHeaderRoute step
	UseHeaderRoutes bool `json:"useHeaderRoutes"`
}

type ManagedRouteMap map[string]map[string]int

type HTTPRouteRule gatewayv1.HTTPRouteRule

type GRPCRouteRule gatewayv1.GRPCRouteRule

type TCPRouteRule v1alpha2.TCPRouteRule

type TLSRouteRule v1alpha2.TLSRouteRule

type HTTPRouteRuleList []gatewayv1.HTTPRouteRule

type GRPCRouteRuleList []gatewayv1.GRPCRouteRule

type TCPRouteRuleList []v1alpha2.TCPRouteRule

type TLSRouteRuleList []v1alpha2.TLSRouteRule

type HTTPBackendRef gatewayv1.HTTPBackendRef

type GRPCBackendRef gatewayv1.GRPCBackendRef

type TCPBackendRef gatewayv1.BackendRef

type TLSBackendRef gatewayv1.BackendRef

type GatewayAPIRoute interface {
	HTTPRoute | GRPCRoute | TCPRoute | TLSRoute
	GetName() string
}

type GatewayAPIRouteRule[T1 GatewayAPIBackendRef] interface {
	*HTTPRouteRule | *GRPCRouteRule | *TCPRouteRule | *TLSRouteRule
	Iterator() (GatewayAPIRouteRuleIterator[T1], bool)
}

type GatewayAPIRouteRuleList[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] interface {
	HTTPRouteRuleList | GRPCRouteRuleList | TCPRouteRuleList | TLSRouteRuleList
	Iterator() (GatewayAPIRouteRuleListIterator[T1, T2], bool)
	Error() error
}

type GatewayAPIBackendRef interface {
	*HTTPBackendRef | *GRPCBackendRef | *TCPBackendRef | *TLSBackendRef
	GetName() string
}

type GatewayAPIRouteRuleListIterator[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] func() (T2, bool)

type GatewayAPIRouteRuleIterator[T1 GatewayAPIBackendRef] func() (T1, bool)
