/*
 * Storage Default Rust
 *
 * Part of MantisDB - High-performance multi-model database.
 * See CONTRIBUTING.md for code standards and comment guidelines.
 */
//go:build rust
// +build rust

package storage

// NewDefaultStorageEngine creates the default storage engine using Rust
func NewDefaultStorageEngine(config StorageConfig) StorageEngine {
	return NewRustStorageEngine(config)
}
