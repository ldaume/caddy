// The package internal provides a simple middleware that (a) prevents access
// to internal locations and (b) allows to return files from internal location
// by setting a special header, e.g. in a proxy response.
package internal

import (
	"net/http"

	"github.com/mholt/caddy/middleware"
)

// Internal middleware protects internal locations from external requests -
// but allows access from the inside by using a special HTTP header.
type Internal struct {
	Next  middleware.Handler
	Paths []string
}

const redirectHeader string = "X-Accel-Redirect"

func isInternalRedirect(w http.ResponseWriter) bool {
	return w.Header().Get(redirectHeader) != ""
}

// ServeHTTP implements the middlware.Handler interface.
func (i Internal) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	// Internal location requested? -> Not found.
	for _, prefix := range i.Paths {
		if middleware.Path(r.URL.Path).Matches(prefix) {
			return http.StatusNotFound, nil
		}
	}

	// Use internal response writer to ignore responses that will be
	// redirected to internal locations
	iw := internalResponseWriter{ResponseWriter: w}
	status, err := i.Next.ServeHTTP(iw, r)

	if isInternalRedirect(iw) && status < 400 {
		// Redirect - adapt request URL path and send it again
		// "down the chain"
		r.URL.Path = iw.Header().Get(redirectHeader)
		iw.ClearHeader()

		status, err = i.Next.ServeHTTP(iw, r)

		if isInternalRedirect(iw) {
			// multiple redirects not supported
			iw.ClearHeader()
			return http.StatusInternalServerError, nil
		}
	}

	return status, err
}

// internalResponseWriter wraps the underlying http.ResponseWriter and ignores
// calls to Write and WriteHeader if the response should be redirected to an
// internal location.
type internalResponseWriter struct {
	http.ResponseWriter
}

// ClearHeader removes all header fields that are already set.
func (w internalResponseWriter) ClearHeader() {
	for k := range w.Header() {
		w.Header().Del(k)
	}
}

// WriteHeader ignores the call if the response should be redirected to an
// internal location.
func (w internalResponseWriter) WriteHeader(code int) {
	if !isInternalRedirect(w) && code < 400 {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write ignores the call if the response should be redirected to an internal
// location.
func (w internalResponseWriter) Write(b []byte) (int, error) {
	if isInternalRedirect(w) {
		return 0, nil
	} else {
		return w.ResponseWriter.Write(b)
	}
}
