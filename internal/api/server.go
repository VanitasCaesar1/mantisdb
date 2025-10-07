// Package api provides internal API server implementations
package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mantisDB/pkg/api"
	"mantisDB/pkg/monitoring"
)

// HTTPServer implements the api.Server interface
type HTTPServer struct {
	server  *http.Server
	mux     *http.ServeMux
	logger  monitoring.Logger
	mu      sync.RWMutex
	running bool
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(logger monitoring.Logger) api.Server {
	mux := http.NewServeMux()
	return &HTTPServer{
		mux:    mux,
		logger: logger,
	}
}

// Start starts the HTTP server
func (s *HTTPServer) Start(ctx context.Context, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.running = true
	s.logger.Info("Starting HTTP server", monitoring.Field{Key: "addr", Value: addr})

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", monitoring.Field{Key: "error", Value: err})
		}
	}()

	return nil
}

// Stop stops the HTTP server
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping HTTP server")

	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	s.running = false
	return nil
}

// RegisterHandler registers a handler for the given pattern
func (s *HTTPServer) RegisterHandler(pattern string, handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mux.Handle(pattern, handler)
	s.logger.Info("Registered handler", monitoring.Field{Key: "pattern", Value: pattern})
}

// Health returns the health status of the server
func (s *HTTPServer) Health() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return fmt.Errorf("server is not running")
	}

	return nil
}
