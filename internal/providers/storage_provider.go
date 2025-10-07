// Package providers contains service providers for dependency injection
package providers

import (
	"mantisDB/internal/container"
	"mantisDB/internal/storage"
	"mantisDB/pkg/config"
	pkgStorage "mantisDB/pkg/storage"
)

// StorageProvider provides storage-related services
type StorageProvider struct{}

// Register registers storage services
func (p *StorageProvider) Register(c *container.Container) error {
	// Register storage engine factory
	c.RegisterFactory("storage.engine", func() interface{} {
		// Get configuration
		configService, err := c.Get("config.manager")
		if err != nil {
			// Fallback to default configuration
			return storage.NewMemoryEngine()
		}

		configManager := configService.(config.ConfigManager)

		// Get storage configuration
		useCGO, _ := configManager.GetBool("database.use_cgo")
		dataDir, _ := configManager.GetString("database.data_dir")
		cacheSize, _ := configManager.GetInt("database.cache_size")
		bufferSize, _ := configManager.GetInt("database.buffer_size")
		syncWrites, _ := configManager.GetBool("database.sync_writes")

		storageConfig := pkgStorage.Config{
			DataDir:    dataDir,
			CacheSize:  int64(cacheSize),
			BufferSize: int64(bufferSize),
			UseCGO:     useCGO,
			SyncWrites: syncWrites,
		}

		if useCGO {
			return NewCGOStorageEngine(storageConfig)
		}
		return NewPureGoStorageEngine(storageConfig)
	})

	// Register memory engine for testing
	c.RegisterFactory("storage.memory", func() interface{} {
		return storage.NewMemoryEngine()
	})

	return nil
}

// Boot initializes storage services
func (p *StorageProvider) Boot(c *container.Container) error {
	// Initialize storage engine
	engine, err := c.Get("storage.engine")
	if err != nil {
		return err
	}

	storageEngine := engine.(pkgStorage.Engine)

	// Get data directory from config
	configService, err := c.Get("config.manager")
	if err == nil {
		configManager := configService.(config.ConfigManager)
		if dataDir, err := configManager.GetString("database.data_dir"); err == nil {
			storageEngine.Open(nil, dataDir)
		}
	}

	return nil
}

// NewCGOStorageEngine creates a CGO storage engine (placeholder)
func NewCGOStorageEngine(config pkgStorage.Config) pkgStorage.Engine {
	// This would create the actual CGO storage engine
	// For now, return memory engine as placeholder
	return storage.NewMemoryEngine()
}

// NewPureGoStorageEngine creates a pure Go storage engine (placeholder)
func NewPureGoStorageEngine(config pkgStorage.Config) pkgStorage.Engine {
	// This would create the actual pure Go storage engine
	// For now, return memory engine as placeholder
	return storage.NewMemoryEngine()
}
