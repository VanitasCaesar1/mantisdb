package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"mantisDB/api"
	"mantisDB/benchmark"
	"mantisDB/cache"
	"mantisDB/config"
	"mantisDB/health"
	"mantisDB/query"
	"mantisDB/shutdown"
	"mantisDB/storage"
	"mantisDB/store"
)

var (
	// Version is set during build time
	Version = "dev"
	// BuildTime is set during build time
	BuildTime = "unknown"
	// GitCommit is set during build time
	GitCommit = "unknown"
)

// VersionInfo contains version information
type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetVersionInfo returns version information
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// PrintVersion prints version information
func PrintVersion() {
	info := GetVersionInfo()
	fmt.Printf("MantisDB %s\n", info.Version)
	fmt.Printf("Build Time: %s\n", info.BuildTime)
	fmt.Printf("Git Commit: %s\n", info.GitCommit)
	fmt.Printf("Go Version: %s\n", info.GoVersion)
	fmt.Printf("Platform: %s\n", info.Platform)
}

// LegacyConfig holds legacy command line configuration
type LegacyConfig struct {
	DataDir         string
	Port            int
	AdminPort       int
	UseCGO          bool
	CacheSize       int64
	BufferSize      int64
	LogLevel        string
	EnableAPI       bool
	EnableCLI       bool
	EnableAdmin     bool
	RunBenchmark    bool
	BenchmarkOnly   bool
	BenchmarkStress string
	ShowVersion     bool
	ShowHelp        bool
}

// MantisDB represents the main database instance
type MantisDB struct {
	config          *config.Config
	legacyConfig    *LegacyConfig
	storageEngine   storage.StorageEngine
	cacheManager    *cache.CacheManager
	queryParser     *query.Parser
	queryOptimizer  *query.QueryOptimizer
	queryExecutor   *query.QueryExecutor
	store           *store.MantisStore
	apiServer       *api.Server
	adminServerProc *os.Process // Rust admin-server process
	healthChecker   *health.HealthChecker
	shutdownManager *shutdown.Manager
	startupManager  *shutdown.StartupManager
}

func main() {
	// Parse command line flags
	legacyConfig := parseFlags()

	// Handle version flag
	if legacyConfig.ShowVersion {
		PrintVersion()
		return
	}

	// Load configuration
	cfg := config.DefaultConfig()

	// Override with legacy command line flags
	if legacyConfig.Port != 0 {
		cfg.Server.Port = legacyConfig.Port
	}
	if legacyConfig.AdminPort != 0 {
		cfg.Server.AdminPort = legacyConfig.AdminPort
	}
	if legacyConfig.DataDir != "" {
		cfg.Database.DataDir = legacyConfig.DataDir
	}
	cfg.Database.UseCGO = legacyConfig.UseCGO

	// Load from environment variables
	if err := cfg.LoadFromEnv(); err != nil {
		log.Fatalf("Failed to load configuration from environment: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set GC percentage if configured
	if cfg.Memory.GCPercent > 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
		runtime.GC()
	}

	// Initialize MantisDB
	db, err := NewMantisDB(cfg, legacyConfig)
	if err != nil {
		log.Fatalf("Failed to initialize MantisDB: %v", err)
	}

	// Setup graceful shutdown
	db.shutdownManager.Listen()

	// Start the database
	ctx := context.Background()
	if err := db.startupManager.Start(ctx); err != nil {
		log.Fatalf("Failed to start MantisDB: %v", err)
	}

	// If benchmark-only mode, benchmarks will trigger shutdown when complete
	if legacyConfig.BenchmarkOnly {
		// Wait a bit for benchmarks to start
		time.Sleep(time.Second * 2)
		// Benchmarks will call shutdown when complete (see runBenchmarks)
	}

	// Wait for shutdown signal
	db.shutdownManager.Wait()

	fmt.Println("MantisDB shutdown complete")
}

// parseFlags parses command line flags and returns legacy configuration
func parseFlags() *LegacyConfig {
	legacyConfig := &LegacyConfig{}

	flag.StringVar(&legacyConfig.DataDir, "data-dir", "", "Data directory path")
	flag.IntVar(&legacyConfig.Port, "port", 0, "Server port")
	flag.IntVar(&legacyConfig.AdminPort, "admin-port", 0, "Admin dashboard port")
	flag.BoolVar(&legacyConfig.UseCGO, "use-cgo", false, "Use CGO storage engine")
	flag.Int64Var(&legacyConfig.CacheSize, "cache-size", 0, "Cache size in bytes")
	flag.Int64Var(&legacyConfig.BufferSize, "buffer-size", 0, "Buffer size in bytes")
	flag.StringVar(&legacyConfig.LogLevel, "log-level", "", "Log level (debug, info, warn, error)")
	flag.BoolVar(&legacyConfig.EnableAPI, "enable-api", true, "Enable HTTP API server")
	flag.BoolVar(&legacyConfig.EnableCLI, "enable-cli", true, "Enable CLI interface")
	flag.BoolVar(&legacyConfig.EnableAdmin, "enable-admin", true, "Enable admin dashboard")
	flag.BoolVar(&legacyConfig.RunBenchmark, "benchmark", false, "Run benchmarks after startup")
	flag.BoolVar(&legacyConfig.BenchmarkOnly, "benchmark-only", false, "Run benchmarks and exit")
	flag.StringVar(&legacyConfig.BenchmarkStress, "benchmark-stress", "", "Benchmark stress level (light, medium, heavy, extreme)")
	flag.BoolVar(&legacyConfig.ShowVersion, "version", false, "Show version information")
	flag.BoolVar(&legacyConfig.ShowHelp, "help", false, "Show help information")

	flag.Parse()

	// Handle help flag
	if legacyConfig.ShowHelp {
		ShowUsage()
		os.Exit(0)
	}

	return legacyConfig
}

// NewMantisDB creates a new MantisDB instance
func NewMantisDB(cfg *config.Config, legacyConfig *LegacyConfig) (*MantisDB, error) {
	db := &MantisDB{
		config:       cfg,
		legacyConfig: legacyConfig,
	}

	// Initialize shutdown manager
	db.shutdownManager = shutdown.NewManager(30 * time.Second)
	db.startupManager = shutdown.NewStartupManager(60 * time.Second)

	// Parse cache and buffer sizes
	cacheSize, err := config.ParseSize(cfg.Database.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("invalid cache size: %v", err)
	}

	bufferSize, err := config.ParseSize(cfg.Database.BufferSize)
	if err != nil {
		return nil, fmt.Errorf("invalid buffer size: %v", err)
	}

	// Initialize storage engine
	storageConfig := storage.StorageConfig{
		DataDir:    cfg.Database.DataDir,
		BufferSize: bufferSize,
		CacheSize:  cacheSize,
		UseCGO:     cfg.Database.UseCGO,
		SyncWrites: cfg.Database.SyncWrites,
	}

	// Use default storage engine (Rust if built with -tags rust, Pure Go otherwise)
	db.storageEngine = storage.NewDefaultStorageEngine(storageConfig)

	// Initialize cache manager
	cacheConfig := cache.CacheConfig{
		MaxSize:         cacheSize,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute * 5,
		EvictionPolicy:  cfg.Memory.EvictionPolicy,
	}
	db.cacheManager = cache.NewCacheManager(cacheConfig)

	// Initialize query components
	db.queryParser = query.NewParser()

	optimizerConfig := query.OptimizerConfig{
		EnableIndexHints:        true,
		EnableJoinReordering:    true,
		EnablePredicatePushdown: true,
		CostThreshold:           100.0,
	}
	db.queryOptimizer = query.NewQueryOptimizer(optimizerConfig)

	executorConfig := query.ExecutorConfig{
		EnableCaching:   true,
		CacheTimeout:    300,
		MaxConcurrency:  10,
		QueryTimeout:    int(cfg.Database.QueryTimeout.Seconds()),
		EnableProfiling: true,
	}
	db.queryExecutor = query.NewQueryExecutor(db.storageEngine, db.cacheManager, executorConfig)

	// Initialize unified store
	db.store = store.NewMantisStore(db.storageEngine, db.cacheManager)

	// Initialize API server
	db.apiServer = api.NewServer(db.store, cfg.Server.Port)

	// Initialize health checker
	db.healthChecker = health.NewHealthChecker(
		cfg.Health.CheckInterval,
		cfg.Health.Timeout,
		cfg.Health.Enabled,
	)

	// Register health checks
	db.registerHealthChecks()

	// Register startup functions
	db.registerStartupFunctions()

	// Register shutdown functions
	db.registerShutdownFunctions()

	return db, nil
}

// findAvailablePort tries to find an available port starting from the given port
func findAvailablePort(startPort int, maxAttempts int) (int, error) {
	return findAvailablePortWithIncrement(startPort, maxAttempts, 1)
}

// findAvailablePortWithIncrement tries to find an available port with custom increment
func findAvailablePortWithIncrement(startPort int, maxAttempts int, increment int) (int, error) {
	for i := 0; i < maxAttempts; i++ {
		port := startPort + (i * increment)
		addr := fmt.Sprintf(":%d", port)

		// Try to listen on the port
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			// Port is available, close the listener and return
			listener.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available port found after %d attempts starting from %d", maxAttempts, startPort)
}

// startAdminServer starts the Rust admin-server binary
// The Rust server provides both the admin API and serves the static UI
func (db *MantisDB) startAdminServer(ctx context.Context) error {
	// Check if admin-server binary exists
	adminServerPath := "bin/admin-server"
	if _, err := os.Stat(adminServerPath); os.IsNotExist(err) {
		log.Printf("⚠️  Rust admin-server not found at %s", adminServerPath)
		log.Printf("   Build it with: make build")
		return nil // Don't fail, just skip admin server
	}

	log.Printf("🚀 Starting Rust admin-server...")

	// Don't use CommandContext - we want the process to outlive the startup context
	// Use Command instead so it runs independently
	cmd := exec.Command(adminServerPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set process group ID so we can kill the entire process tree
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start admin server: %v", err)
	}

	// Store the process in the MantisDB struct so we can kill it on shutdown
	db.adminServerProc = cmd.Process

	// Monitor the process and properly reap it when it exits
	// This prevents zombie processes
	go func() {
		if err := cmd.Wait(); err != nil {
			// Only log if it's not a normal termination
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 0 && exitErr.ExitCode() != -1 {
					log.Printf("⚠️  Admin server exited with error: %v", err)
				}
			}
		}
		// Process has been reaped, clear the reference
		db.adminServerProc = nil
	}()

	log.Printf("✅ Rust admin-server started (PID: %d) on port 8081", cmd.Process.Pid)
	return nil
}

// ExecuteQuery executes a query string
func (db *MantisDB) ExecuteQuery(ctx context.Context, queryStr string) (*query.ExecutionResult, error) {
	// Parse the query
	parsedQuery, err := db.queryParser.Parse(queryStr)
	if err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	// Optimize the query
	optimizedQuery, err := db.queryOptimizer.Optimize(parsedQuery)
	if err != nil {
		return nil, fmt.Errorf("optimization error: %v", err)
	}

	// Execute the query
	execCtx := &query.ExecutionContext{
		Query:       optimizedQuery,
		Parameters:  make(map[string]interface{}),
		Timeout:     30,
		EnableCache: true,
		CacheKey:    fmt.Sprintf("query:%s", queryStr),
	}

	result, err := db.queryExecutor.Execute(ctx, execCtx)
	if err != nil {
		return nil, fmt.Errorf("execution error: %v", err)
	}

	return result, nil
}

// GetStats returns database statistics
func (db *MantisDB) GetStats() map[string]interface{} {
	ctx := context.Background()
	return db.store.GetStats(ctx)
}

// startCLI starts the command line interface
func (db *MantisDB) startCLI(ctx context.Context) {
	// CLI is ready, no need for verbose output
}

// healthCheck performs a health check on the database
func (db *MantisDB) healthCheck(ctx context.Context) error {
	// Check storage engine
	if err := db.storageEngine.HealthCheck(ctx); err != nil {
		return fmt.Errorf("storage engine unhealthy: %v", err)
	}

	// Test basic operations
	testKey := "health_check_test"
	testValue := "test_value"

	// Test put
	if err := db.storageEngine.Put(ctx, testKey, testValue); err != nil {
		return fmt.Errorf("health check put failed: %v", err)
	}

	// Test get
	value, err := db.storageEngine.Get(ctx, testKey)
	if err != nil {
		return fmt.Errorf("health check get failed: %v", err)
	}

	if value != testValue {
		return fmt.Errorf("health check value mismatch: expected %s, got %s", testValue, value)
	}

	// Test delete
	if err := db.storageEngine.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("health check delete failed: %v", err)
	}

	return nil
}

// getStorageEngineType returns a string describing the storage engine type
func (db *MantisDB) getStorageEngineType() string {
	if db.config.Database.UseCGO {
		return "CGO (C-based)"
	}
	return "Pure Go"
}

// runBenchmarks runs the benchmark suite
func (db *MantisDB) runBenchmarks(ctx context.Context) {
	fmt.Println("Waiting for system to initialize before running benchmarks...")
	time.Sleep(time.Second * 3)

	// Determine stress level based on configuration
	stressLevel := "medium"
	if db.legacyConfig.BenchmarkStress != "" {
		stressLevel = db.legacyConfig.BenchmarkStress
	} else if db.legacyConfig.BenchmarkOnly {
		stressLevel = "heavy" // More intensive for benchmark-only mode
	}

	config := benchmark.GetStressTestConfig(stressLevel)
	benchmarkSuite := benchmark.NewProductionBenchmarkSuite(db.store, config)

	// Run production benchmarks with scoring
	score, err := benchmarkSuite.RunProductionBenchmarks(ctx)
	if err != nil {
		log.Printf("Production benchmark failed: %v", err)

		// Fallback to legacy benchmarks
		fmt.Println("Falling back to legacy benchmarks...")
		results, legacyErr := benchmarkSuite.RunAllBenchmarks(ctx)
		if legacyErr != nil {
			log.Printf("Legacy benchmark also failed: %v", legacyErr)
			return
		}

		benchmarkSuite.PrintResults(results)
		benchmarkSuite.SaveResults(results, "benchmark_results.json")
		return
	}

	// Print comprehensive results
	fmt.Printf("\n=== MANTISDB PRODUCTION BENCHMARK RESULTS ===\n")
	fmt.Printf("Overall Score: %.2f/100 (%s)\n", score.OverallScore, score.Grade)
	fmt.Printf("Test Environment: %s stress level\n", score.TestEnvironment.StressLevel)
	fmt.Printf("Total Operations: %d\n", score.TestEnvironment.TotalOperations)
	fmt.Printf("Data Processed: %.2f MB\n", score.TestEnvironment.DataProcessedMB)
	fmt.Printf("System: %s on %s (%d CPUs, %d MB RAM)\n",
		score.SystemInfo.OS, score.SystemInfo.Architecture,
		score.SystemInfo.CPUs, score.SystemInfo.Memory)

	fmt.Printf("\nCategory Scores:\n")
	for category, categoryScore := range score.CategoryScores {
		fmt.Printf("  %s: %.2f/100\n", category, categoryScore)
	}

	if len(score.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range score.Recommendations {
			fmt.Printf("  • %s\n", rec)
		}
	}

	// Save detailed results
	benchmarkSuite.SaveBenchmarkScore(score, "production_benchmark_results.json")

	if db.legacyConfig.BenchmarkOnly {
		fmt.Println("\nProduction benchmarks complete. Exiting...")
		// Signal shutdown
		go func() {
			time.Sleep(time.Second)
			if p, err := os.FindProcess(os.Getpid()); err == nil {
				p.Signal(syscall.SIGTERM)
			}
		}()
	}
}

// Note: Admin API is now handled by the Rust admin-server binary
// located at rust-core/src/bin/admin-server.rs

// createAuthMiddleware creates authentication middleware for admin dashboard
func (db *MantisDB) createAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For development, allow all requests
			// In production, implement proper authentication
			adminToken := db.config.Security.AdminToken
			if adminToken == "" {
				// Development mode - no authentication required
				next.ServeHTTP(w, r)
				return
			}

			// Check for authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract token from "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(adminToken)) != 1 {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// addSecurityHeaders adds security headers to all responses
func (db *MantisDB) addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// CORS headers for API endpoints
		if strings.HasPrefix(r.URL.Path, "/api/") && db.config.Security.EnableCORS {
			origins := strings.Join(db.config.Security.CORSOrigins, ", ")
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// registerHealthChecks registers health checks
func (db *MantisDB) registerHealthChecks() {
	// Database health check
	dbCheck := health.NewDatabaseCheck("database", db.storageEngine)
	db.healthChecker.RegisterCheck(dbCheck)

	// Memory health check (80% threshold)
	memCheck := health.NewMemoryCheck("memory", 80.0)
	db.healthChecker.RegisterCheck(memCheck)

	// Disk health check (90% threshold)
	diskCheck := health.NewDiskCheck("disk", db.config.Database.DataDir, 90.0)
	db.healthChecker.RegisterCheck(diskCheck)
}

// registerStartupFunctions registers startup functions in priority order
func (db *MantisDB) registerStartupFunctions() {
	// 1. Initialize storage engine (highest priority)
	db.startupManager.RegisterStartupFunc("storage", 1, func(ctx context.Context) error {
		// Only show minimal startup info
		fmt.Printf("Starting MantisDB %s...\n", Version)

		// Create data directory if it doesn't exist
		if err := os.MkdirAll(db.config.Database.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %v", err)
		}

		// Initialize storage engine
		if err := db.storageEngine.Init(db.config.Database.DataDir); err != nil {
			return fmt.Errorf("failed to initialize storage engine: %v", err)
		}

		return nil
	})

	// 2. Start health checker
	db.startupManager.RegisterStartupFunc("health", 2, func(ctx context.Context) error {
		db.healthChecker.Start(ctx)
		return nil
	})

	// 3. API server disabled - Rust admin-server provides all API endpoints
	// The Rust admin-server on port 8081 handles both UI and API
	_ = db.apiServer // Keep reference to avoid unused variable warning

	// 4. Start admin dashboard (skip in benchmark-only mode)
	// Note: Starts the Rust admin-server binary which provides full admin API
	if db.legacyConfig.EnableAdmin && !db.legacyConfig.BenchmarkOnly {
		db.startupManager.RegisterStartupFunc("admin", 4, func(ctx context.Context) error {
			// Start admin server directly (not in goroutine) to catch errors
			if err := db.startAdminServer(ctx); err != nil {
				log.Printf("⚠️  Admin server startup error: %v", err)
				// Don't fail the whole startup, just log the error
			}
			return nil
		})
	}

	// 5. Perform health check
	db.startupManager.RegisterStartupFunc("health-check", 5, func(ctx context.Context) error {
		if err := db.healthCheck(ctx); err != nil {
			return fmt.Errorf("health check failed: %v", err)
		}
		return nil
	})

	// 6. Start CLI and create demo data
	if db.legacyConfig.EnableCLI {
		db.startupManager.RegisterStartupFunc("cli", 6, func(ctx context.Context) error {
			go db.startCLI(ctx)
			return nil
		})
	}

	// 7. Run benchmarks
	if db.legacyConfig.RunBenchmark || db.legacyConfig.BenchmarkOnly {
		db.startupManager.RegisterStartupFunc("benchmarks", 7, func(ctx context.Context) error {
			go db.runBenchmarks(ctx)
			return nil
		})
	}

	// 8. Final startup message (with delay to ensure ports are selected)
	db.startupManager.RegisterStartupFunc("startup-complete", 8, func(ctx context.Context) error {
		// Wait a moment for servers to start and select ports
		if !db.legacyConfig.BenchmarkOnly {
			time.Sleep(500 * time.Millisecond)
		}

		if db.legacyConfig.BenchmarkOnly {
			fmt.Printf("✓ MantisDB initialized for benchmarking\n")
		} else {
			fmt.Printf("✓ MantisDB started successfully\n")
			if db.legacyConfig.EnableAdmin {
				fmt.Printf("  Admin UI:  http://localhost:%d\n", db.config.Server.AdminPort)
				fmt.Printf("  Admin API: http://localhost:%d/api/\n", db.config.Server.AdminPort)
			}
		}
		return nil
	})
}

// registerShutdownFunctions registers shutdown functions in priority order
func (db *MantisDB) registerShutdownFunctions() {
	// 1. Stop health checker (highest priority)
	db.shutdownManager.RegisterShutdownFunc("health", 1, func(ctx context.Context) error {
		db.healthChecker.Stop()
		return nil
	})

	// 2. Stop admin server (Rust process)
	db.shutdownManager.RegisterShutdownFunc("admin", 2, func(ctx context.Context) error {
		if db.adminServerProc != nil {
			log.Printf("Stopping Rust admin-server (PID: %d)...", db.adminServerProc.Pid)
			// Kill the entire process group to ensure clean shutdown
			pgid, err := syscall.Getpgid(db.adminServerProc.Pid)
			if err == nil {
				// Send SIGTERM to the process group first for graceful shutdown
				syscall.Kill(-pgid, syscall.SIGTERM)
				// Wait a bit for graceful shutdown
				time.Sleep(500 * time.Millisecond)
				// If still running, force kill
				if db.adminServerProc != nil {
					syscall.Kill(-pgid, syscall.SIGKILL)
				}
			} else {
				// Fallback to killing just the process
				if err := db.adminServerProc.Kill(); err != nil {
					return fmt.Errorf("failed to kill admin server: %v", err)
				}
			}
		}
		return nil
	})

	// 3. Stop API server
	db.shutdownManager.RegisterShutdownFunc("api", 3, func(ctx context.Context) error {
		if db.apiServer != nil {
			return db.apiServer.Stop(ctx)
		}
		return nil
	})

	// 4. Close storage engine (lowest priority)
	db.shutdownManager.RegisterShutdownFunc("storage", 4, func(ctx context.Context) error {
		if db.storageEngine != nil {
			return db.storageEngine.Close()
		}
		return nil
	})
}

// ShowUsage displays usage information
func ShowUsage() {
	fmt.Println("MantisDB - A hybrid database system")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  mantisDB [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  mantisDB --data-dir=/var/lib/mantisdb --port=8080")
	fmt.Println("  mantisDB --use-cgo --cache-size=268435456")
	fmt.Println("  mantisDB --benchmark-only  # Run benchmarks and exit")
	fmt.Println("  mantisDB --benchmark       # Run benchmarks after startup")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  MANTIS_ADMIN_TOKEN - Token for admin dashboard authentication")
	fmt.Println("  MANTIS_LOG_LEVEL   - Log level (debug, info, warn, error)")
	fmt.Println("  MANTIS_DATA_DIR    - Data directory path")
	fmt.Println()
}
