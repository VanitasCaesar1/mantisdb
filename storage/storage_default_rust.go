//go:build rust
// +build rust

package storage

// NewDefaultStorageEngine creates the default storage engine using Rust
func NewDefaultStorageEngine(config StorageConfig) StorageEngine {
	return NewRustStorageEngine(config)
}
