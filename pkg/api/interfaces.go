// Package api provides public interfaces for MantisDB API components
package api

import (
	"context"
	"net/http"
)

// Server defines the interface for API servers
type Server interface {
	Start(ctx context.Context, addr string) error
	Stop(ctx context.Context) error
	RegisterHandler(pattern string, handler http.Handler)
	Health() error
}

// Handler defines the interface for API request handlers
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Middleware defines the interface for HTTP middleware
type Middleware interface {
	Wrap(next http.Handler) http.Handler
}
