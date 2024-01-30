package plugin

const (
	GatewayAPIUpdateError                     = "Error updating Gateway API %q: %s"
	GatewayAPIManifestError                   = "httpRoute and tcpRoute fields are empty. tcpRoute or httpRoute should be set"
	HTTPRouteFieldIsEmptyError                = "httpRoute field is empty. It has to be set to remove managed routes"
	InvalidHeaderMatchTypeError               = "Invalid header match type"
	BackendRefWasNotFoundInHTTPRouteError     = "backendRef was not found in httpRoute"
	BackendRefListWasNotFoundInHTTPRouteError = "backendRef list was not found in httpRoute"
	BackendRefWasNotFoundInTCPRouteError      = "backendRef was not found in tcpRoute"
	BackendRefListWasNotFoundInTCPRouteError  = "backendRef list was not found in tcpRoute"
)
