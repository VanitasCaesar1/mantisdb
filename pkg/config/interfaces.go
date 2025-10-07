// Package config provides public interfaces for MantisDB configuration components
package config

import (
	"context"
	"fmt"
	"time"
)

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	// Configuration loading
	Load(ctx context.Context, source string) error
	Reload(ctx context.Context) error

	// Configuration access
	Get(key string) (interface{}, error)
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
	GetDuration(key string) (time.Duration, error)

	// Configuration updates
	Set(key string, value interface{}) error
	Save(ctx context.Context) error

	// Configuration watching
	Watch(key string, callback ConfigCallback) error
	Unwatch(key string) error
}

// ConfigCallback defines a callback function for configuration changes
type ConfigCallback func(key string, oldValue, newValue interface{})

// Validator defines the interface for configuration validation
type Validator interface {
	Validate(config map[string]interface{}) error
	ValidateField(key string, value interface{}) error
}

// Source defines the interface for configuration sources
type Source interface {
	Load(ctx context.Context) (map[string]interface{}, error)
	Save(ctx context.Context, config map[string]interface{}) error
	Watch(ctx context.Context, callback func(map[string]interface{})) error
}

// Config represents the main configuration structure
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// Cache configuration
	Cache CacheConfig `yaml:"cache" json:"cache"`

	// Monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
}

// StorageConfig contains storage-related configuration
type StorageConfig struct {
	Engine      string `yaml:"engine" json:"engine"`
	Path        string `yaml:"path" json:"path"`
	MaxSize     int64  `yaml:"max_size" json:"max_size"`
	SyncWrites  bool   `yaml:"sync_writes" json:"sync_writes"`
	Compression bool   `yaml:"compression" json:"compression"`
}

// CacheConfig contains cache-related configuration
type CacheConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	MaxSize        int64         `yaml:"max_size" json:"max_size"`
	DefaultTTL     time.Duration `yaml:"default_ttl" json:"default_ttl"`
	EvictionPolicy string        `yaml:"eviction_policy" json:"eviction_policy"`
}

// MonitoringConfig contains monitoring-related configuration
type MonitoringConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	MetricsPath    string `yaml:"metrics_path" json:"metrics_path"`
	HealthPath     string `yaml:"health_path" json:"health_path"`
	PrometheusAddr string `yaml:"prometheus_addr" json:"prometheus_addr"`
}

// LoggingConfig contains logging-related configuration
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	Output     string `yaml:"output" json:"output"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
}

// Common configuration errors
var (
	ErrKeyNotFound  = fmt.Errorf("configuration key not found")
	ErrInvalidType  = fmt.Errorf("invalid configuration type")
	ErrInvalidValue = fmt.Errorf("invalid configuration value")
)
