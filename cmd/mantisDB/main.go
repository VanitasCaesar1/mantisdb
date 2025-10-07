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
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	adminapi "mantisDB/admin/api"
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
	adminServer     *http.Server
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

	// If benchmark-only mode, wait for benchmarks to complete and exit
	if legacyConfig.BenchmarkOnly {
		// Wait a bit for benchmarks to start
		time.Sleep(time.Second * 2)
		// In benchmark-only mode, we'll exit after benchmarks complete
		go func() {
			time.Sleep(time.Second * 10) // Give benchmarks time to run
			db.shutdownManager.Shutdown()
		}()
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

	if cfg.Database.UseCGO {
		db.storageEngine = storage.NewCGOStorageEngine(storageConfig)
	} else {
		db.storageEngine = storage.NewPureGoStorageEngine(storageConfig)
	}

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

// startAdminServer starts the admin dashboard server with embedded assets
func (db *MantisDB) startAdminServer(ctx context.Context) error {
	// Import admin API package
	adminAPI := db.createAdminAPI()

	// Create HTTP mux
	mux := http.NewServeMux()

	// Add authentication middleware
	authMiddleware := db.createAuthMiddleware()

	// Mount admin API with authentication
	mux.Handle("/api/", authMiddleware(adminAPI))

	// Try to use embedded assets first
	if adminapi.AssetsAvailable() {
		log.Printf("Using embedded admin dashboard assets")
		// Serve embedded static files
		fileServer := http.FileServer(adminapi.GetAssetsFS())
		mux.Handle("/", fileServer)
	} else {
		// Fallback: try filesystem
		assetsDir := "admin/api/assets/dist"
		if _, err := os.Stat(filepath.Join(assetsDir, "index.html")); os.IsNotExist(err) {
			// Try alternative path
			assetsDir = "admin/assets/dist"
			if _, err := os.Stat(filepath.Join(assetsDir, "index.html")); os.IsNotExist(err) {
				log.Printf("Warning: admin assets not found, admin UI will not be available")
				// Serve a simple message
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprintf(w, `<html><body><h1>MantisDB Admin</h1><p>Admin UI assets not found. Build with: make build<br>API is available at <a href="/api/">/api/</a></p></body></html>`)
				})
			} else {
				log.Printf("Using filesystem admin dashboard assets from: %s", assetsDir)
				mux.Handle("/", http.FileServer(http.Dir(assetsDir)))
			}
		} else {
			log.Printf("Using filesystem admin dashboard assets from: %s", assetsDir)
			mux.Handle("/", http.FileServer(http.Dir(assetsDir)))
		}
	}

	// Find an available port starting from the configured port
	// Use increment of 2 to avoid collision with API server (which uses +1, +2, +3...)
	adminPort, err := findAvailablePortWithIncrement(db.config.Server.AdminPort, 10, 2)
	if err != nil {
		return fmt.Errorf("failed to find available admin port: %v", err)
	}
	
	// Update config with actual port
	if adminPort != db.config.Server.AdminPort {
		log.Printf("Admin port %d in use, using port %d instead", db.config.Server.AdminPort, adminPort)
		db.config.Server.AdminPort = adminPort
	}

	// Create server with timeouts and security headers
	db.adminServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", adminPort),
		Handler:      db.addSecurityHeaders(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	if err := db.adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("admin server failed: %v", err)
	}

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

// createAdminAPI creates and configures the admin API handler
func (db *MantisDB) createAdminAPI() http.Handler {
	return adminapi.NewAdminAPI(db.store)
}

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

	// 3. Start API server (skip in benchmark-only mode)
	if db.legacyConfig.EnableAPI && !db.legacyConfig.BenchmarkOnly {
		db.startupManager.RegisterStartupFunc("api", 3, func(ctx context.Context) error {
			go func() {
				if err := db.apiServer.Start(ctx); err != nil {
					log.Printf("API server error: %v", err)
				}
			}()
			return nil
		})
	}

	// 4. Start admin dashboard (skip in benchmark-only mode)
	if db.legacyConfig.EnableAdmin && !db.legacyConfig.BenchmarkOnly {
		db.startupManager.RegisterStartupFunc("admin", 4, func(ctx context.Context) error {
			go func() {
				if err := db.startAdminServer(ctx); err != nil {
					log.Printf("Admin server error: %v", err)
				}
			}()
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
				fmt.Printf("  Admin: http://localhost:%d\n", db.config.Server.AdminPort)
			}
			if db.legacyConfig.EnableAPI {
				fmt.Printf("  API:   http://localhost:%d/api/v1/\n", db.apiServer.GetPort())
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

	// 2. Stop admin server
	db.shutdownManager.RegisterShutdownFunc("admin", 2, func(ctx context.Context) error {
		if db.adminServer != nil {
			return db.adminServer.Shutdown(ctx)
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
