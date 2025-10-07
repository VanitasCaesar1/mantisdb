// Package providers contains service providers for dependency injection
package providers

import (
	"context"
	"time"

	"mantisDB/internal/container"
	"mantisDB/pkg/config"
)

// ConfigProvider provides configuration-related services
type ConfigProvider struct{}

// Register registers configuration services
func (p *ConfigProvider) Register(c *container.Container) error {
	// Register config manager
	c.RegisterSingleton("config.manager", func() interface{} {
		return NewConfigManager()
	})

	// Register config validator
	c.RegisterSingleton("config.validator", func() interface{} {
		return NewConfigValidator()
	})

	return nil
}

// Boot initializes configuration services
func (p *ConfigProvider) Boot(c *container.Container) error {
	// Load default configuration
	configManager, err := c.Get("config.manager")
	if err != nil {
		return err
	}

	manager := configManager.(config.ConfigManager)

	// Load configuration from default sources
	return manager.Load(context.Background(), "")
}

// NewConfigManager creates a new configuration manager (placeholder)
func NewConfigManager() config.ConfigManager {
	return &MemoryConfigManager{
		config:    make(map[string]interface{}),
		callbacks: make(map[string][]config.ConfigCallback),
	}
}

// NewConfigValidator creates a new configuration validator (placeholder)
func NewConfigValidator() config.Validator {
	return &DefaultConfigValidator{}
}

// MemoryConfigManager is a placeholder configuration manager
type MemoryConfigManager struct {
	config    map[string]interface{}
	callbacks map[string][]config.ConfigCallback
}

func (m *MemoryConfigManager) Load(ctx context.Context, source string) error {
	// Load default configuration
	m.config = map[string]interface{}{
		"server.port":           8080,
		"server.admin_port":     8081,
		"server.host":           "0.0.0.0",
		"database.data_dir":     "/var/lib/mantisdb",
		"database.cache_size":   512 * 1024 * 1024, // 512MB
		"database.buffer_size":  128 * 1024 * 1024, // 128MB
		"database.use_cgo":      false,
		"database.sync_writes":  true,
		"cache.enabled":         true,
		"cache.max_size":        256 * 1024 * 1024, // 256MB
		"cache.default_ttl":     time.Hour,
		"cache.eviction_policy": "lru",
		"logging.level":         "info",
		"logging.format":        "json",
		"logging.output":        "stdout",
	}
	return nil
}

func (m *MemoryConfigManager) Reload(ctx context.Context) error {
	return m.Load(ctx, "")
}

func (m *MemoryConfigManager) Get(key string) (interface{}, error) {
	if value, exists := m.config[key]; exists {
		return value, nil
	}
	return nil, config.ErrKeyNotFound
}

func (m *MemoryConfigManager) GetString(key string) (string, error) {
	value, err := m.Get(key)
	if err != nil {
		return "", err
	}

	if str, ok := value.(string); ok {
		return str, nil
	}

	return "", config.ErrInvalidType
}

func (m *MemoryConfigManager) GetInt(key string) (int, error) {
	value, err := m.Get(key)
	if err != nil {
		return 0, err
	}

	if i, ok := value.(int); ok {
		return i, nil
	}

	return 0, config.ErrInvalidType
}

func (m *MemoryConfigManager) GetBool(key string) (bool, error) {
	value, err := m.Get(key)
	if err != nil {
		return false, err
	}

	if b, ok := value.(bool); ok {
		return b, nil
	}

	return false, config.ErrInvalidType
}

func (m *MemoryConfigManager) GetDuration(key string) (time.Duration, error) {
	value, err := m.Get(key)
	if err != nil {
		return 0, err
	}

	if d, ok := value.(time.Duration); ok {
		return d, nil
	}

	return 0, config.ErrInvalidType
}

func (m *MemoryConfigManager) Set(key string, value interface{}) error {
	oldValue := m.config[key]
	m.config[key] = value

	// Notify callbacks
	if callbacks, exists := m.callbacks[key]; exists {
		for _, callback := range callbacks {
			callback(key, oldValue, value)
		}
	}

	return nil
}

func (m *MemoryConfigManager) Save(ctx context.Context) error {
	// Memory config manager doesn't persist
	return nil
}

func (m *MemoryConfigManager) Watch(key string, callback config.ConfigCallback) error {
	if m.callbacks[key] == nil {
		m.callbacks[key] = make([]config.ConfigCallback, 0)
	}
	m.callbacks[key] = append(m.callbacks[key], callback)
	return nil
}

func (m *MemoryConfigManager) Unwatch(key string) error {
	delete(m.callbacks, key)
	return nil
}

// DefaultConfigValidator is a placeholder configuration validator
type DefaultConfigValidator struct{}

func (v *DefaultConfigValidator) Validate(configMap map[string]interface{}) error {
	// Basic validation
	if port, exists := configMap["server.port"]; exists {
		if p, ok := port.(int); ok {
			if p < 1 || p > 65535 {
				return config.ErrInvalidValue
			}
		}
	}

	return nil
}

func (v *DefaultConfigValidator) ValidateField(key string, value interface{}) error {
	switch key {
	case "server.port", "server.admin_port":
		if p, ok := value.(int); ok {
			if p < 1 || p > 65535 {
				return config.ErrInvalidValue
			}
		}
	case "database.cache_size", "database.buffer_size":
		if size, ok := value.(int); ok {
			if size < 0 {
				return config.ErrInvalidValue
			}
		}
	}

	return nil
}
