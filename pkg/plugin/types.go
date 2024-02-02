package plugin

import (
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayAPIClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayApiv1alpha2 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1alpha2"
	gatewayApiv1beta1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

type RpcPlugin struct {
	IsTest               bool
	LogCtx               *logrus.Entry
	GatewayAPIClientset  *gatewayAPIClientset.Clientset
	Clientset            *kubernetes.Clientset
	TestClientset        v1.ConfigMapInterface
	UpdatedHTTPRouteMock *v1beta1.HTTPRoute
	UpdatedTCPRouteMock  *v1alpha2.TCPRoute
	HTTPRouteClient      gatewayApiv1beta1.HTTPRouteInterface
	TCPRouteClient       gatewayApiv1alpha2.TCPRouteInterface
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
	// ConfigMap name refers to the config map where plugin stores data about managed routes
	ConfigMap string `json:"configMap,omitempty"`
}

type HTTPHeaderRoute struct {
	mutex           sync.Mutex
	managedRouteMap map[string]int
}

type HTTPRouteRule v1beta1.HTTPRouteRule

type TCPRouteRule v1alpha2.TCPRouteRule

type HTTPRouteRuleList []v1beta1.HTTPRouteRule

type TCPRouteRuleList []v1alpha2.TCPRouteRule

type HTTPBackendRef v1beta1.HTTPBackendRef

type TCPBackendRef v1beta1.BackendRef

type GatewayAPIRouteRule[T1 GatewayAPIBackendRef] interface {
	*HTTPRouteRule | *TCPRouteRule
	Iterator() (GatewayAPIRouteRuleIterator[T1], bool)
}

type GatewayAPIRouteRuleList[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] interface {
	HTTPRouteRuleList | TCPRouteRuleList
	Iterator() (GatewayAPIRouteRuleListIterator[T1, T2], bool)
	Error() error
}

type GatewayAPIBackendRef interface {
	*HTTPBackendRef | *TCPBackendRef
	GetName() string
}

type GatewayAPIRouteRuleListIterator[T1 GatewayAPIBackendRef, T2 GatewayAPIRouteRule[T1]] func() (T2, bool)

type GatewayAPIRouteRuleIterator[T1 GatewayAPIBackendRef] func() (T1, bool)
