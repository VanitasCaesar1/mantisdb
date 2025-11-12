//go:build !rust
// +build !rust

package storage

// NewDefaultStorageEngine creates the default storage engine (pure Go fallback)
func NewDefaultStorageEngine(config StorageConfig) StorageEngine {
	return NewPureGoStorageEngine(config)
}
