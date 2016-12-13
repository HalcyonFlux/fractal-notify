package notify

// The map of notification codes should be detailed enough to satisfy the use
// cases of your programm, but small enough to keep log-analysis meaningful.
// You can use your own list via notifier.SetCodes()
var standardCodes = map[int][2]string{
	0:   [2]string{"MSG", "GeneralMessage"},      // [Restricted] Notifications that are not errors will be logged as 0: GeneralMessage
	1:   [2]string{"ERR", "GeneralError"},        // [Restricted] Nonspecific error. Also used to log other error types implementing the errors.error interface
	2:   [2]string{"ERR", "ConfigurationError"},  // inapropriate configuration value (e.g. error parsing flags)
	3:   [2]string{"ERR", "FailedAction"},        // failed attempt to do something, e.g open or write to a file
	4:   [2]string{"ERR", "UserError"},           // e.g.
	10:  [2]string{"ERR", "CatastrophicFailure"}, // an error that will (should) cause a panic, e.g. cannot start the server
	100: [2]string{"MSG", "HTTP-StatusContinue"},
	101: [2]string{"MSG", "HTTP-StatusSwitchingProtocols"},
	102: [2]string{"MSG", "HTTP-StatusProcessing"},
	200: [2]string{"MSG", "HTTP-StatusOK"},
	201: [2]string{"MSG", "HTTP-StatusCreated"},
	202: [2]string{"MSG", "HTTP-StatusAccepted"},
	203: [2]string{"MSG", "HTTP-StatusNonAuthoritativeInfo"},
	204: [2]string{"MSG", "HTTP-StatusNoContent"},
	205: [2]string{"MSG", "HTTP-StatusResetContent"},
	206: [2]string{"MSG", "HTTP-StatusPartialContent"},
	207: [2]string{"MSG", "HTTP-StatusMultiStatus"},
	208: [2]string{"MSG", "HTTP-StatusAlreadyReported"},
	226: [2]string{"MSG", "HTTP-StatusIMUsed"},
	300: [2]string{"MSG", "HTTP-StatusMultipleChoices"},
	301: [2]string{"MSG", "HTTP-StatusMovedPermanently"},
	302: [2]string{"MSG", "HTTP-StatusFound"},
	303: [2]string{"MSG", "HTTP-StatusSeeOther"},
	304: [2]string{"MSG", "HTTP-StatusNotModified"},
	305: [2]string{"MSG", "HTTP-StatusUseProxy"},
	307: [2]string{"MSG", "HTTP-StatusTemporaryRedirect"},
	308: [2]string{"MSG", "HTTP-StatusPermanentRedirect"},
	400: [2]string{"ERR", "HTTP-StatusBadRequest"},
	401: [2]string{"ERR", "HTTP-StatusUnauthorized"},
	402: [2]string{"ERR", "HTTP-StatusPaymentRequired"},
	403: [2]string{"ERR", "HTTP-StatusForbidden"},
	404: [2]string{"ERR", "HTTP-StatusNotFound"},
	405: [2]string{"ERR", "HTTP-StatusMethodNotAllowed"},
	406: [2]string{"ERR", "HTTP-StatusNotAcceptable"},
	407: [2]string{"ERR", "HTTP-StatusProxyAuthRequired"},
	408: [2]string{"ERR", "HTTP-StatusRequestTimeout"},
	409: [2]string{"ERR", "HTTP-StatusConflict"},
	410: [2]string{"ERR", "HTTP-StatusGone"},
	411: [2]string{"ERR", "HTTP-StatusLengthRequired"},
	412: [2]string{"ERR", "HTTP-StatusPreconditionFailed"},
	413: [2]string{"ERR", "HTTP-StatusRequestEntityTooLarge"},
	414: [2]string{"ERR", "HTTP-StatusRequestURITooLong"},
	415: [2]string{"ERR", "HTTP-StatusUnsupportedMediaType"},
	416: [2]string{"ERR", "HTTP-StatusRequestedRangeNotSatisfiable"},
	417: [2]string{"ERR", "HTTP-StatusExpectationFailed"},
	418: [2]string{"ERR", "HTTP-StatusTeapot"},
	422: [2]string{"ERR", "HTTP-StatusUnprocessableEntity"},
	423: [2]string{"ERR", "HTTP-StatusLocked"},
	424: [2]string{"ERR", "HTTP-StatusFailedDependency"},
	426: [2]string{"ERR", "HTTP-StatusUpgradeRequired"},
	428: [2]string{"ERR", "HTTP-StatusPreconditionRequired"},
	429: [2]string{"ERR", "HTTP-StatusTooManyRequests"},
	431: [2]string{"ERR", "HTTP-StatusRequestHeaderFieldsTooLarge"},
	451: [2]string{"ERR", "HTTP-StatusUnavailableForLegalReasons"},
	500: [2]string{"ERR", "HTTP-StatusInternalServerError"},
	501: [2]string{"ERR", "HTTP-StatusNotImplemented"},
	502: [2]string{"ERR", "HTTP-StatusBadGateway"},
	503: [2]string{"ERR", "HTTP-StatusServiceUnavailable"},
	504: [2]string{"ERR", "HTTP-StatusGatewayTimeout"},
	505: [2]string{"ERR", "HTTP-StatusHTTPVersionNotSupported"},
	506: [2]string{"ERR", "HTTP-StatusVariantAlsoNegotiates"},
	507: [2]string{"ERR", "HTTP-StatusInsufficientStorage"},
	508: [2]string{"ERR", "HTTP-StatusLoopDetected"},
	510: [2]string{"ERR", "HTTP-StatusNotExtended"},
	511: [2]string{"ERR", "HTTP-StatusNetworkAuthenticationRequired"},
	999: [2]string{"ERR", "ShouldNeverHappen"}, // [Restricted]. Should be used to track "should-never-happen" cases
}
