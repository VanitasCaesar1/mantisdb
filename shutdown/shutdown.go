package shutdown

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Manager manages graceful shutdown of the application
type Manager struct {
	shutdownFuncs []ShutdownFunc
	timeout       time.Duration
	signals       []os.Signal
	mutex         sync.Mutex
	shutdownCh    chan struct{}
	once          sync.Once
}

// ShutdownFunc represents a function to be called during shutdown
type ShutdownFunc struct {
	Name     string
	Priority int // Lower numbers have higher priority
	Func     func(ctx context.Context) error
}

// NewManager creates a new shutdown manager
func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		shutdownFuncs: make([]ShutdownFunc, 0),
		timeout:       timeout,
		signals:       []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		shutdownCh:    make(chan struct{}),
	}
}

// RegisterShutdownFunc registers a function to be called during shutdown
func (m *Manager) RegisterShutdownFunc(name string, priority int, fn func(ctx context.Context) error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	shutdownFunc := ShutdownFunc{
		Name:     name,
		Priority: priority,
		Func:     fn,
	}

	// Insert in priority order (lower numbers first)
	inserted := false
	for i, existing := range m.shutdownFuncs {
		if priority < existing.Priority {
			// Insert at position i
			m.shutdownFuncs = append(m.shutdownFuncs[:i], append([]ShutdownFunc{shutdownFunc}, m.shutdownFuncs[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		m.shutdownFuncs = append(m.shutdownFuncs, shutdownFunc)
	}
}

// SetSignals sets the signals to listen for
func (m *Manager) SetSignals(signals ...os.Signal) {
	m.signals = signals
}

// Listen starts listening for shutdown signals
func (m *Manager) Listen() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, m.signals...)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v", sig)
		m.Shutdown()
	}()
}

// Shutdown initiates graceful shutdown
func (m *Manager) Shutdown() {
	m.once.Do(func() {
		close(m.shutdownCh)
		m.executeShutdown()
	})
}

// Wait waits for shutdown to complete
func (m *Manager) Wait() {
	<-m.shutdownCh
}

// executeShutdown executes all registered shutdown functions
func (m *Manager) executeShutdown() {
	log.Println("Starting graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.mutex.Lock()
	funcs := make([]ShutdownFunc, len(m.shutdownFuncs))
	copy(funcs, m.shutdownFuncs)
	m.mutex.Unlock()

	var wg sync.WaitGroup
	errorCh := make(chan error, len(funcs))

	for _, shutdownFunc := range funcs {
		wg.Add(1)
		go func(sf ShutdownFunc) {
			defer wg.Done()

			log.Printf("Shutting down: %s", sf.Name)
			start := time.Now()

			if err := sf.Func(ctx); err != nil {
				log.Printf("Error shutting down %s: %v", sf.Name, err)
				errorCh <- fmt.Errorf("shutdown %s failed: %w", sf.Name, err)
			} else {
				log.Printf("Successfully shut down %s (took %v)", sf.Name, time.Since(start))
			}
		}(shutdownFunc)
	}

	// Wait for all shutdown functions to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All shutdown functions completed successfully")
	case <-ctx.Done():
		log.Println("Shutdown timeout reached, forcing exit")
	}

	// Collect any errors
	close(errorCh)
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		log.Printf("Shutdown completed with %d errors:", len(errors))
		for _, err := range errors {
			log.Printf("  - %v", err)
		}
	} else {
		log.Println("Graceful shutdown completed successfully")
	}
}

// StartupManager manages application startup
type StartupManager struct {
	startupFuncs []StartupFunc
	timeout      time.Duration
	mutex        sync.Mutex
}

// StartupFunc represents a function to be called during startup
type StartupFunc struct {
	Name     string
	Priority int // Lower numbers have higher priority
	Func     func(ctx context.Context) error
}

// NewStartupManager creates a new startup manager
func NewStartupManager(timeout time.Duration) *StartupManager {
	return &StartupManager{
		startupFuncs: make([]StartupFunc, 0),
		timeout:      timeout,
	}
}

// RegisterStartupFunc registers a function to be called during startup
func (m *StartupManager) RegisterStartupFunc(name string, priority int, fn func(ctx context.Context) error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	startupFunc := StartupFunc{
		Name:     name,
		Priority: priority,
		Func:     fn,
	}

	// Insert in priority order (lower numbers first)
	inserted := false
	for i, existing := range m.startupFuncs {
		if priority < existing.Priority {
			// Insert at position i
			m.startupFuncs = append(m.startupFuncs[:i], append([]StartupFunc{startupFunc}, m.startupFuncs[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		m.startupFuncs = append(m.startupFuncs, startupFunc)
	}
}

// Start executes all registered startup functions
func (m *StartupManager) Start(ctx context.Context) error {
	log.Println("Starting application...")

	startupCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	m.mutex.Lock()
	funcs := make([]StartupFunc, len(m.startupFuncs))
	copy(funcs, m.startupFuncs)
	m.mutex.Unlock()

	for _, startupFunc := range funcs {
		log.Printf("Starting: %s", startupFunc.Name)
		start := time.Now()

		if err := startupFunc.Func(startupCtx); err != nil {
			log.Printf("Failed to start %s: %v", startupFunc.Name, err)
			return fmt.Errorf("startup %s failed: %w", startupFunc.Name, err)
		}

		log.Printf("Successfully started %s (took %v)", startupFunc.Name, time.Since(start))
	}

	log.Println("Application startup completed successfully")
	return nil
}

// ReadinessProbe represents a readiness probe
type ReadinessProbe struct {
	name string
	fn   func(ctx context.Context) error
}

// NewReadinessProbe creates a new readiness probe
func NewReadinessProbe(name string, fn func(ctx context.Context) error) *ReadinessProbe {
	return &ReadinessProbe{
		name: name,
		fn:   fn,
	}
}

// Check executes the readiness probe
func (p *ReadinessProbe) Check(ctx context.Context) error {
	return p.fn(ctx)
}

// Name returns the probe name
func (p *ReadinessProbe) Name() string {
	return p.name
}

// LivenessProbe represents a liveness probe
type LivenessProbe struct {
	name string
	fn   func(ctx context.Context) error
}

// NewLivenessProbe creates a new liveness probe
func NewLivenessProbe(name string, fn func(ctx context.Context) error) *LivenessProbe {
	return &LivenessProbe{
		name: name,
		fn:   fn,
	}
}

// Check executes the liveness probe
func (p *LivenessProbe) Check(ctx context.Context) error {
	return p.fn(ctx)
}

// Name returns the probe name
func (p *LivenessProbe) Name() string {
	return p.name
}
