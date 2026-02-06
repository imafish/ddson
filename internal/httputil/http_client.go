package httputil

import "net/http"

// HTTPDoer is the interface for executing HTTP requests.
// *http.Client satisfies this interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
