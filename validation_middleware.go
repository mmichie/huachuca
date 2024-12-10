package main

import (
	"bytes"
	"io"
	"net/http"
)

type ValidationMiddleware struct {
	maxBytes int64
}

func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		maxBytes: MaxRequestBodyBytes,
	}
}

func (m *ValidationMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check body size for methods that typically have a body
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			// Limit the request body size
			r.Body = http.MaxBytesReader(w, r.Body, m.maxBytes)

			// Read and restore the body for later use
			var bodyBytes []byte
			var err error
			if bodyBytes, err = io.ReadAll(r.Body); err != nil {
				http.Error(w, ErrRequestBodyTooBig.Error(), http.StatusRequestEntityTooLarge)
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		next.ServeHTTP(w, r)
	})
}
