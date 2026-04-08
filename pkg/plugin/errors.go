package plugin

const (
	GatewayAPIUpdateError                    = "error updating Gateway API %q: %s"
	GatewayAPIManifestError                  = "No routes configured. At least one of 'httpRoutes', 'grpcRoutes', 'tcpRoutes', 'tlsRoutes', 'httpRoute', 'grpcRoute', 'tcpRoute' or 'tlsRoute' must be set"
	InvalidHeaderMatchTypeError              = "invalid header match type"
	BackendRefWasNotFoundInHTTPRouteError    = "backendRef was not found in httpRoute"
	BackendRefWasNotFoundInGRPCRouteError    = "backendRef was not found in grpcRoute"
	BackendRefWasNotFoundInTCPRouteError     = "backendRef was not found in tcpRoute"
	BackendRefWasNotFoundInTLSRouteError     = "backendRef was not found in tlsRoute"
	BackendRefListWasNotFoundInTCPRouteError = "backendRef list was not found in tcpRoute"
	BackendRefListWasNotFoundInTLSRouteError = "backendRef list was not found in tlsRoute"
)
