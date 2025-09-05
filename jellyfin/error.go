package jellyfin

import (
	"net/http"
)

// HTTPError represents a structured HTTP error response.
type HTTPError struct {
	Status  int                 `json:"status"`
	Type    string              `json:"type,omitempty"`
	Title   string              `json:"title,omitempty"`
	Errors  map[string][]string `json:"errors,omitempty"`
	TraceID string              `json:"traceId,omitempty"`
}

// statusTypeMap maps HTTP status codes to RFC 9110 types.
var statusTypeMap = map[int]string{
	400: "https://tools.ietf.org/html/rfc9110#section-15.5.1",  // Bad Request
	401: "https://tools.ietf.org/html/rfc9110#section-15.5.2",  // Unauthorized
	403: "https://tools.ietf.org/html/rfc9110#section-15.5.3",  // Forbidden
	404: "https://tools.ietf.org/html/rfc9110#section-15.5.5",  // Not Found
	405: "https://tools.ietf.org/html/rfc9110#section-15.5.6",  // Method Not Allowed
	406: "https://tools.ietf.org/html/rfc9110#section-15.5.7",  // Not Acceptable
	408: "https://tools.ietf.org/html/rfc9110#section-15.5.9",  // Request Timeout
	409: "https://tools.ietf.org/html/rfc9110#section-15.5.10", // Conflict
	410: "https://tools.ietf.org/html/rfc9110#section-15.5.11", // Gone
	411: "https://tools.ietf.org/html/rfc9110#section-15.5.12", // Length Required
	412: "https://tools.ietf.org/html/rfc9110#section-15.5.13", // Precondition Failed
	413: "https://tools.ietf.org/html/rfc9110#section-15.5.14", // Content Too Large
	414: "https://tools.ietf.org/html/rfc9110#section-15.5.15", // URI Too Long
	415: "https://tools.ietf.org/html/rfc9110#section-15.5.16", // Unsupported Media Type
	416: "https://tools.ietf.org/html/rfc9110#section-15.5.17", // Range Not Satisfiable
	417: "https://tools.ietf.org/html/rfc9110#section-15.5.18", // Expectation Failed
	500: "https://tools.ietf.org/html/rfc9110#section-15.6.1",  // Internal Server Error
	501: "https://tools.ietf.org/html/rfc9110#section-15.6.2",  // Not Implemented
	502: "https://tools.ietf.org/html/rfc9110#section-15.6.3",  // Bad Gateway
	503: "https://tools.ietf.org/html/rfc9110#section-15.6.4",  // Service Unavailable
	504: "https://tools.ietf.org/html/rfc9110#section-15.6.5",  // Gateway Timeout
}

// apierror writes a structured error response.
func apierror(w http.ResponseWriter, msg string, status int) {
	response := HTTPError{
		Status: status,
		Title:  msg,
	}
	if typeUrl, ok := statusTypeMap[status]; ok {
		response.Type = typeUrl
	}
	w.WriteHeader(status)
	serveJSON(response, w)
}
