package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Backup   BackupConfig   `yaml:"backup"`
	Logging  LoggingConfig  `yaml:"logging"`
	Memory   MemoryConfig   `yaml:"memory"`
	Security SecurityConfig `yaml:"security"`
	Health   HealthConfig   `yaml:"health"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port      int    `yaml:"port" env:"MANTIS_PORT"`
	AdminPort int    `yaml:"admin_port" env:"MANTIS_ADMIN_PORT"`
	Host      string `yaml:"host" env:"MANTIS_HOST"`
	TLSCert   string `yaml:"tls_cert" env:"MANTIS_TLS_CERT"`
	TLSKey    string `yaml:"tls_key" env:"MANTIS_TLS_KEY"`
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	DataDir      string        `yaml:"data_dir" env:"MANTIS_DATA_DIR"`
	WALDir       string        `yaml:"wal_dir" env:"MANTIS_WAL_DIR"`
	CacheSize    string        `yaml:"cache_size" env:"MANTIS_CACHE_SIZE"`
	BufferSize   string        `yaml:"buffer_size" env:"MANTIS_BUFFER_SIZE"`
	UseCGO       bool          `yaml:"use_cgo" env:"MANTIS_USE_CGO"`
	SyncWrites   bool          `yaml:"sync_writes" env:"MANTIS_SYNC_WRITES"`
	MaxConns     int           `yaml:"max_connections" env:"MANTIS_MAX_CONNECTIONS"`
	QueryTimeout time.Duration `yaml:"query_timeout" env:"MANTIS_QUERY_TIMEOUT"`
}

// BackupConfig holds backup-related configuration
type BackupConfig struct {
	Enabled       bool   `yaml:"enabled" env:"MANTIS_BACKUP_ENABLED"`
	Schedule      string `yaml:"schedule" env:"MANTIS_BACKUP_SCHEDULE"`
	RetentionDays int    `yaml:"retention_days" env:"MANTIS_BACKUP_RETENTION_DAYS"`
	Destination   string `yaml:"destination" env:"MANTIS_BACKUP_DESTINATION"`
	Compression   bool   `yaml:"compression" env:"MANTIS_BACKUP_COMPRESSION"`
	Encryption    bool   `yaml:"encryption" env:"MANTIS_BACKUP_ENCRYPTION"`
}

// LoggingConfig holds logging-related configuration
type LoggingConfig struct {
	Level      string `yaml:"level" env:"MANTIS_LOG_LEVEL"`
	Format     string `yaml:"format" env:"MANTIS_LOG_FORMAT"`
	Output     string `yaml:"output" env:"MANTIS_LOG_OUTPUT"`
	MaxSize    int    `yaml:"max_size" env:"MANTIS_LOG_MAX_SIZE"`
	MaxBackups int    `yaml:"max_backups" env:"MANTIS_LOG_MAX_BACKUPS"`
	MaxAge     int    `yaml:"max_age" env:"MANTIS_LOG_MAX_AGE"`
}

// MemoryConfig holds memory management configuration
type MemoryConfig struct {
	CacheLimit     string `yaml:"cache_limit" env:"MANTIS_CACHE_LIMIT"`
	EvictionPolicy string `yaml:"eviction_policy" env:"MANTIS_EVICTION_POLICY"`
	GCPercent      int    `yaml:"gc_percent" env:"MANTIS_GC_PERCENT"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	AdminToken     string        `yaml:"admin_token" env:"MANTIS_ADMIN_TOKEN"`
	APIKeys        []string      `yaml:"api_keys" env:"MANTIS_API_KEYS"`
	RateLimit      int           `yaml:"rate_limit" env:"MANTIS_RATE_LIMIT"`
	SessionTimeout time.Duration `yaml:"session_timeout" env:"MANTIS_SESSION_TIMEOUT"`
	EnableCORS     bool          `yaml:"enable_cors" env:"MANTIS_ENABLE_CORS"`
	CORSOrigins    []string      `yaml:"cors_origins" env:"MANTIS_CORS_ORIGINS"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	CheckInterval time.Duration `yaml:"check_interval" env:"MANTIS_HEALTH_CHECK_INTERVAL"`
	Timeout       time.Duration `yaml:"timeout" env:"MANTIS_HEALTH_TIMEOUT"`
	Enabled       bool          `yaml:"enabled" env:"MANTIS_HEALTH_ENABLED"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:      8080,
			AdminPort: 8081,
			Host:      "localhost",
		},
		Database: DatabaseConfig{
			DataDir:      "./data",
			WALDir:       "./wal",
			CacheSize:    "100MB",
			BufferSize:   "64MB",
			UseCGO:       false,
			SyncWrites:   true,
			MaxConns:     100,
			QueryTimeout: 30 * time.Second,
		},
		Backup: BackupConfig{
			Enabled:       true,
			Schedule:      "0 2 * * *", // Daily at 2 AM
			RetentionDays: 30,
			Destination:   "./backups",
			Compression:   true,
			Encryption:    false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100, // MB
			MaxBackups: 3,
			MaxAge:     28, // days
		},
		Memory: MemoryConfig{
			CacheLimit:     "1GB",
			EvictionPolicy: "lru",
			GCPercent:      100,
		},
		Security: SecurityConfig{
			RateLimit:      1000,
			SessionTimeout: 24 * time.Hour,
			EnableCORS:     true,
			CORSOrigins:    []string{"*"},
		},
		Health: HealthConfig{
			CheckInterval: 30 * time.Second,
			Timeout:       5 * time.Second,
			Enabled:       true,
		},
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() error {
	// Server config
	if port := os.Getenv("MANTIS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}
	if adminPort := os.Getenv("MANTIS_ADMIN_PORT"); adminPort != "" {
		if p, err := strconv.Atoi(adminPort); err == nil {
			c.Server.AdminPort = p
		}
	}
	if host := os.Getenv("MANTIS_HOST"); host != "" {
		c.Server.Host = host
	}
	if cert := os.Getenv("MANTIS_TLS_CERT"); cert != "" {
		c.Server.TLSCert = cert
	}
	if key := os.Getenv("MANTIS_TLS_KEY"); key != "" {
		c.Server.TLSKey = key
	}

	// Database config
	if dataDir := os.Getenv("MANTIS_DATA_DIR"); dataDir != "" {
		c.Database.DataDir = dataDir
	}
	if walDir := os.Getenv("MANTIS_WAL_DIR"); walDir != "" {
		c.Database.WALDir = walDir
	}
	if cacheSize := os.Getenv("MANTIS_CACHE_SIZE"); cacheSize != "" {
		c.Database.CacheSize = cacheSize
	}
	if bufferSize := os.Getenv("MANTIS_BUFFER_SIZE"); bufferSize != "" {
		c.Database.BufferSize = bufferSize
	}
	if useCGO := os.Getenv("MANTIS_USE_CGO"); useCGO != "" {
		c.Database.UseCGO = strings.ToLower(useCGO) == "true"
	}
	if syncWrites := os.Getenv("MANTIS_SYNC_WRITES"); syncWrites != "" {
		c.Database.SyncWrites = strings.ToLower(syncWrites) == "true"
	}
	if maxConns := os.Getenv("MANTIS_MAX_CONNECTIONS"); maxConns != "" {
		if mc, err := strconv.Atoi(maxConns); err == nil {
			c.Database.MaxConns = mc
		}
	}
	if queryTimeout := os.Getenv("MANTIS_QUERY_TIMEOUT"); queryTimeout != "" {
		if qt, err := time.ParseDuration(queryTimeout); err == nil {
			c.Database.QueryTimeout = qt
		}
	}

	// Backup config
	if enabled := os.Getenv("MANTIS_BACKUP_ENABLED"); enabled != "" {
		c.Backup.Enabled = strings.ToLower(enabled) == "true"
	}
	if schedule := os.Getenv("MANTIS_BACKUP_SCHEDULE"); schedule != "" {
		c.Backup.Schedule = schedule
	}
	if retention := os.Getenv("MANTIS_BACKUP_RETENTION_DAYS"); retention != "" {
		if r, err := strconv.Atoi(retention); err == nil {
			c.Backup.RetentionDays = r
		}
	}
	if destination := os.Getenv("MANTIS_BACKUP_DESTINATION"); destination != "" {
		c.Backup.Destination = destination
	}
	if compression := os.Getenv("MANTIS_BACKUP_COMPRESSION"); compression != "" {
		c.Backup.Compression = strings.ToLower(compression) == "true"
	}
	if encryption := os.Getenv("MANTIS_BACKUP_ENCRYPTION"); encryption != "" {
		c.Backup.Encryption = strings.ToLower(encryption) == "true"
	}

	// Logging config
	if level := os.Getenv("MANTIS_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv("MANTIS_LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}
	if output := os.Getenv("MANTIS_LOG_OUTPUT"); output != "" {
		c.Logging.Output = output
	}

	// Memory config
	if cacheLimit := os.Getenv("MANTIS_CACHE_LIMIT"); cacheLimit != "" {
		c.Memory.CacheLimit = cacheLimit
	}
	if evictionPolicy := os.Getenv("MANTIS_EVICTION_POLICY"); evictionPolicy != "" {
		c.Memory.EvictionPolicy = evictionPolicy
	}
	if gcPercent := os.Getenv("MANTIS_GC_PERCENT"); gcPercent != "" {
		if gcp, err := strconv.Atoi(gcPercent); err == nil {
			c.Memory.GCPercent = gcp
		}
	}

	// Security config
	if adminToken := os.Getenv("MANTIS_ADMIN_TOKEN"); adminToken != "" {
		c.Security.AdminToken = adminToken
	}
	if apiKeys := os.Getenv("MANTIS_API_KEYS"); apiKeys != "" {
		c.Security.APIKeys = strings.Split(apiKeys, ",")
	}
	if rateLimit := os.Getenv("MANTIS_RATE_LIMIT"); rateLimit != "" {
		if rl, err := strconv.Atoi(rateLimit); err == nil {
			c.Security.RateLimit = rl
		}
	}
	if sessionTimeout := os.Getenv("MANTIS_SESSION_TIMEOUT"); sessionTimeout != "" {
		if st, err := time.ParseDuration(sessionTimeout); err == nil {
			c.Security.SessionTimeout = st
		}
	}
	if enableCORS := os.Getenv("MANTIS_ENABLE_CORS"); enableCORS != "" {
		c.Security.EnableCORS = strings.ToLower(enableCORS) == "true"
	}
	if corsOrigins := os.Getenv("MANTIS_CORS_ORIGINS"); corsOrigins != "" {
		c.Security.CORSOrigins = strings.Split(corsOrigins, ",")
	}

	// Health config
	if checkInterval := os.Getenv("MANTIS_HEALTH_CHECK_INTERVAL"); checkInterval != "" {
		if ci, err := time.ParseDuration(checkInterval); err == nil {
			c.Health.CheckInterval = ci
		}
	}
	if timeout := os.Getenv("MANTIS_HEALTH_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			c.Health.Timeout = t
		}
	}
	if enabled := os.Getenv("MANTIS_HEALTH_ENABLED"); enabled != "" {
		c.Health.Enabled = strings.ToLower(enabled) == "true"
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Server.AdminPort <= 0 || c.Server.AdminPort > 65535 {
		return fmt.Errorf("invalid admin port: %d", c.Server.AdminPort)
	}
	if c.Server.Port == c.Server.AdminPort {
		return fmt.Errorf("server port and admin port cannot be the same")
	}
	if c.Database.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}
	if c.Database.WALDir == "" {
		return fmt.Errorf("WAL directory cannot be empty")
	}
	if c.Database.MaxConns <= 0 {
		return fmt.Errorf("max connections must be positive")
	}
	if c.Database.QueryTimeout <= 0 {
		return fmt.Errorf("query timeout must be positive")
	}
	if c.Backup.RetentionDays <= 0 {
		return fmt.Errorf("backup retention days must be positive")
	}
	if c.Security.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}
	if c.Security.SessionTimeout <= 0 {
		return fmt.Errorf("session timeout must be positive")
	}

	return nil
}

// IsTLSEnabled returns true if TLS is configured
func (c *Config) IsTLSEnabled() bool {
	return c.Server.TLSCert != "" && c.Server.TLSKey != ""
}

// GetServerAddr returns the server address
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetAdminAddr returns the admin server address
func (c *Config) GetAdminAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.AdminPort)
}

// ParseSize parses a size string like "100MB" into bytes
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "B") {
		multiplier = 1
		numStr = strings.TrimSuffix(sizeStr, "B")
	} else {
		numStr = sizeStr
	}

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return num * multiplier, nil
}
