package e2e

import (
	"time"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	FIRST_HTTP_ROUTE_PATH                       = "./testdata/first-httproute.yml"
	FIRST_GRPC_ROUTE_PATH                       = "./testdata/first-grpcroute.yml"
	FIRST_TCP_ROUTE_PATH                        = "./testdata/first-tcproute.yml"
	SINGLE_HTTP_ROUTE_ROLLOUT_PATH              = "./testdata/single-httproute-rollout.yml"
	SINGLE_GRPC_ROUTE_ROLLOUT_PATH              = "./testdata/single-grpcroute-rollout.yml"
	SINGLE_HEADER_BASED_HTTP_ROUTE_ROLLOUT_PATH = "./testdata/single-header-based-httproute-rollout.yml"
	SINGLE_HEADER_BASED_GRPC_ROUTE_ROLLOUT_PATH = "./testdata/single-header-based-grpcroute-rollout.yml"
	SINGLE_TCP_ROUTE_ROLLOUT_PATH               = "./testdata/single-tcproute-rollout.yml"

	ROLLOUT_TEMPLATE_CONTAINERS_FIELD      = "spec.template.spec.containers"
	ROLLOUT_TEMPLATE_FIRST_CONTAINER_FIELD = "spec.template.spec.containers.0"
	NEW_IMAGE_FIELD_VALUE                  = "argoproj/rollouts-demo:green"

	ROLLOUT_ROUTE_RULE_INDEX        = 0
	FIRST_HEADER_BASED_RULES_LENGTH = 2
	HEADER_BASED_RULE_INDEX         = 1
	LAST_HEADER_BASED_RULES_LENGTH  = 1

	HEADER_BASED_MATCH_INDEX  = 0
	HEADER_BASED_HEADER_INDEX = 0

	CANARY_BACKEND_REF_INDEX       = 1
	HEADER_BASED_BACKEND_REF_INDEX = 0

	FIRST_CANARY_ROUTE_WEIGHT = 0
	LAST_CANARY_ROUTE_WEIGHT  = 30
	DEFAULT_ROUTE_WEIGHT      = 1 // HTTPRoute rules that are managed by the rollout should never update their weight to the setWeight value. It should stay as the default 1

	RESOURCES_MAP_KEY contextKey = "resourcesMap"

	HTTP_ROUTE_KEY = "httpRoute"
	GRPC_ROUTE_KEY = "grpcRoute"
	TCP_ROUTE_KEY  = "tcpRoute"
	ROLLOUT_KEY    = "rollout"
)

const (
	SHORT_PERIOD  = time.Second
	MEDIUM_PERIOD = 30 * time.Second
	LONG_PERIOD   = 60 * time.Second
)

var (
	FIRST_HEADER_BASED_HTTP_ROUTE_VALUE gatewayv1.HTTPHeaderMatch
	headerBasedHTTPRouteValueType       = gatewayv1.HeaderMatchExact
	LAST_HEADER_BASED_HTTP_ROUTE_VALUE  = gatewayv1.HTTPHeaderMatch{
		Name:  "X-Test",
		Type:  &headerBasedHTTPRouteValueType,
		Value: "test",
	}

	FIRST_HEADER_BASED_GRPC_ROUTE_VALUE gatewayv1.GRPCHeaderMatch
	headerBasedGRPCRouteValueType       = gatewayv1.HeaderMatchExact
	LAST_HEADER_BASED_GRPC_ROUTE_VALUE  = gatewayv1.GRPCHeaderMatch{
		Name:  "X-Test",
		Type:  &headerBasedGRPCRouteValueType,
		Value: "test",
	}
)
