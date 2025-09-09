package e2e

import (
	"time"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// HTTP Route test paths
	HTTP_ROUTE_BASIC_PATH          = "./testdata/httproute-basic.yml"
	HTTP_ROUTE_HEADER_PATH         = "./testdata/httproute-header.yml"
	HTTP_ROUTE_BASIC_ROLLOUT_PATH  = "./testdata/single-httproute-rollout.yml"
	HTTP_ROUTE_HEADER_ROLLOUT_PATH = "./testdata/single-header-based-httproute-rollout.yml"

	// GRPC Route test paths
	GRPC_ROUTE_BASIC_PATH          = "./testdata/grpcroute-basic.yml"
	GRPC_ROUTE_HEADER_PATH         = "./testdata/grpcroute-header.yml"
	GRPC_ROUTE_BASIC_ROLLOUT_PATH  = "./testdata/single-grpcroute-rollout.yml"
	GRPC_ROUTE_HEADER_ROLLOUT_PATH = "./testdata/single-header-based-grpcroute-rollout.yml"

	// TCP Route test paths
	TCP_ROUTE_BASIC_PATH         = "./testdata/tcproute-basic.yml"
	TCP_ROUTE_BASIC_ROLLOUT_PATH = "./testdata/single-tcproute-rollout.yml"

	// TLS Route test paths
	TLS_ROUTE_BASIC_PATH         = "./testdata/tlsroute-basic.yml"
	TLS_ROUTE_BASIC_ROLLOUT_PATH = "./testdata/single-tlsroute-rollout.yml"

	// HTTP Route filter test paths
	HTTP_ROUTE_FILTERS_PATH         = "./testdata/httproute-filters.yml"
	HTTP_ROUTE_FILTERS_ROLLOUT_PATH = "./testdata/single-httproute-filters-rollout.yml"

	// GRPC Route filter test paths
	GRPC_ROUTE_FILTERS_PATH         = "./testdata/grpcroute-filters.yml"
	GRPC_ROUTE_FILTERS_ROLLOUT_PATH = "./testdata/single-grpcroute-filters-rollout.yml"

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

	RESOURCES_MAP_KEY contextKey = "resourcesMap"

	HTTP_ROUTE_KEY = "httpRoute"
	GRPC_ROUTE_KEY = "grpcRoute"
	TCP_ROUTE_KEY  = "tcpRoute"
	TLS_ROUTE_KEY  = "tlsRoute"
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
