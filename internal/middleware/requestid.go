package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID injects a unique request ID into the request header if not present.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(RequestIDHeader) == "" {
			id := uuid.New().String()
			r.Header.Set(RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

// GetRequestID extracts the request ID from the request.
func GetRequestID(r *http.Request) string {
	return r.Header.Get(RequestIDHeader)
}
