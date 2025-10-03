package cache

import (
	"context"
	"sync"
	"time"
)

// CacheManager handles intelligent caching with dependency tracking
type CacheManager struct {
	entries    map[string]*CacheEntry
	mutex      sync.RWMutex
	depGraph   *DependencyGraph
	ttlManager *TTLManager
	config     CacheConfig
}

// CacheEntry represents a cached item
type CacheEntry struct {
	Key          string
	Value        interface{}
	Size         int64
	AccessCount  int64
	LastAccess   time.Time
	CreatedAt    time.Time
	TTL          time.Duration
	Dependencies []string
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	MaxSize         int64
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	EvictionPolicy  string // "lru", "lfu", "ttl"
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config CacheConfig) *CacheManager {
	cm := &CacheManager{
		entries:    make(map[string]*CacheEntry),
		depGraph:   NewDependencyGraph(),
		ttlManager: NewTTLManager(config.CleanupInterval),
		config:     config,
	}

	// Start background cleanup
	go cm.backgroundCleanup()

	return cm
}

// Get retrieves a value from cache
func (cm *CacheManager) Get(ctx context.Context, key string) (interface{}, bool) {
	cm.mutex.RLock()
	entry, exists := cm.entries[key]
	cm.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if expired
	if cm.isExpired(entry) {
		cm.Delete(ctx, key)
		return nil, false
	}

	// Update access statistics
	cm.mutex.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	cm.mutex.Unlock()

	return entry.Value, true
}

// Put stores a value in cache
func (cm *CacheManager) Put(ctx context.Context, key string, value interface{}, ttl time.Duration, dependencies []string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Calculate size (simplified)
	size := cm.calculateSize(value)

	// Check if we need to evict
	if cm.needsEviction(size) {
		cm.evict(size)
	}

	entry := &CacheEntry{
		Key:          key,
		Value:        value,
		Size:         size,
		AccessCount:  1,
		LastAccess:   time.Now(),
		CreatedAt:    time.Now(),
		TTL:          ttl,
		Dependencies: dependencies,
	}

	cm.entries[key] = entry

	// Update dependency graph
	for _, dep := range dependencies {
		cm.depGraph.AddDependency(key, dep)
	}

	// Schedule TTL cleanup
	if ttl > 0 {
		cm.ttlManager.Schedule(key, ttl)
	}

	return nil
}

// Delete removes a key from cache
func (cm *CacheManager) Delete(ctx context.Context, key string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.entries[key]; exists {
		delete(cm.entries, key)
		cm.depGraph.RemoveNode(key)
		cm.ttlManager.Cancel(key)

		// Invalidate dependents
		dependents := cm.depGraph.GetDependents(key)
		for _, dependent := range dependents {
			cm.invalidateKey(dependent)
		}
	}
}

// InvalidateDependencies invalidates all entries that depend on the given key
func (cm *CacheManager) InvalidateDependencies(ctx context.Context, key string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	dependents := cm.depGraph.GetDependents(key)
	for _, dependent := range dependents {
		cm.invalidateKey(dependent)
	}
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var totalSize int64
	var totalEntries int

	for _, entry := range cm.entries {
		totalSize += entry.Size
		totalEntries++
	}

	return CacheStats{
		TotalEntries: totalEntries,
		TotalSize:    totalSize,
		MaxSize:      cm.config.MaxSize,
		HitRate:      cm.calculateHitRate(),
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	TotalEntries int
	TotalSize    int64
	MaxSize      int64
	HitRate      float64
}

// Private methods

func (cm *CacheManager) isExpired(entry *CacheEntry) bool {
	if entry.TTL == 0 {
		return false
	}
	return time.Since(entry.CreatedAt) > entry.TTL
}

func (cm *CacheManager) calculateSize(value interface{}) int64 {
	// Simplified size calculation
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	default:
		return 1024 // Default size
	}
}

func (cm *CacheManager) needsEviction(newSize int64) bool {
	currentSize := cm.getCurrentSize()
	return currentSize+newSize > cm.config.MaxSize
}

func (cm *CacheManager) getCurrentSize() int64 {
	var total int64
	for _, entry := range cm.entries {
		total += entry.Size
	}
	return total
}

func (cm *CacheManager) evict(neededSize int64) {
	switch cm.config.EvictionPolicy {
	case "lru":
		cm.evictLRU(neededSize)
	case "lfu":
		cm.evictLFU(neededSize)
	default:
		cm.evictLRU(neededSize)
	}
}

func (cm *CacheManager) evictLRU(neededSize int64) {
	// Find least recently used entries
	var candidates []*CacheEntry
	for _, entry := range cm.entries {
		candidates = append(candidates, entry)
	}

	// Sort by last access time
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].LastAccess.After(candidates[j].LastAccess) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Evict until we have enough space
	var freedSize int64
	for _, entry := range candidates {
		if freedSize >= neededSize {
			break
		}
		cm.invalidateKey(entry.Key)
		freedSize += entry.Size
	}
}

func (cm *CacheManager) evictLFU(neededSize int64) {
	// Find least frequently used entries
	var candidates []*CacheEntry
	for _, entry := range cm.entries {
		candidates = append(candidates, entry)
	}

	// Sort by access count
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].AccessCount > candidates[j].AccessCount {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Evict until we have enough space
	var freedSize int64
	for _, entry := range candidates {
		if freedSize >= neededSize {
			break
		}
		cm.invalidateKey(entry.Key)
		freedSize += entry.Size
	}
}

func (cm *CacheManager) invalidateKey(key string) {
	if _, exists := cm.entries[key]; exists {
		delete(cm.entries, key)
		cm.depGraph.RemoveNode(key)
		cm.ttlManager.Cancel(key)

		// Recursively invalidate dependents
		dependents := cm.depGraph.GetDependents(key)
		for _, dependent := range dependents {
			cm.invalidateKey(dependent)
		}
	}
}

func (cm *CacheManager) calculateHitRate() float64 {
	// Simplified hit rate calculation
	return 0.85 // Placeholder
}

func (cm *CacheManager) backgroundCleanup() {
	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		cm.cleanupExpired()
	}
}

func (cm *CacheManager) cleanupExpired() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	var expiredKeys []string
	for key, entry := range cm.entries {
		if cm.isExpired(entry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		cm.invalidateKey(key)
	}
}
