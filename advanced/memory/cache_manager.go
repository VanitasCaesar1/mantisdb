package memory

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// EvictionPolicy defines the interface for cache eviction strategies
type EvictionPolicy interface {
	Evict(cache *Cache, needed int64) []CacheEntry
	OnAccess(entry *CacheEntry)
	OnInsert(entry *CacheEntry)
	Name() string
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key         string
	Value       any
	Size        int64
	AccessCount int64
	LastAccess  time.Time
	CreatedAt   time.Time
	TTL         time.Duration
	Priority    int
	mutex       sync.RWMutex
}

// Cache represents the cache storage
type Cache struct {
	entries map[string]*CacheEntry
	mutex   sync.RWMutex
}

// MemoryLimits defines memory constraints
type MemoryLimits struct {
	MaxSize         int64         // Maximum cache size in bytes
	MaxEntries      int           // Maximum number of entries
	MemoryThreshold float64       // System memory threshold (0.0-1.0)
	CheckInterval   time.Duration // Memory check interval
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	TotalEntries   int64
	TotalSize      int64
	MaxSize        int64
	HitCount       int64
	MissCount      int64
	EvictionCount  int64
	HitRatio       float64
	MemoryUsage    int64
	SystemMemory   int64
	LastEviction   time.Time
	EvictionPolicy string
}

// CacheManager handles memory management and eviction policies
type CacheManager struct {
	cache    *Cache
	policies map[string]EvictionPolicy
	limits   *MemoryLimits
	stats    *CacheStats
	monitor  *MemoryMonitor
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// Atomic counters for thread-safe stats
	hitCount      int64
	missCount     int64
	evictionCount int64
}

// Config holds cache manager configuration
type Config struct {
	MaxSize         int64
	MaxEntries      int
	MemoryThreshold float64
	CheckInterval   time.Duration
	DefaultPolicy   string
	CleanupInterval time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxSize:         1024 * 1024 * 1024, // 1GB
		MaxEntries:      100000,
		MemoryThreshold: 0.8, // 80% of system memory
		CheckInterval:   time.Second * 30,
		DefaultPolicy:   "lru",
		CleanupInterval: time.Minute * 5,
	}
}

// NewCacheManager creates a new cache manager with the specified configuration
func NewCacheManager(config *Config) *CacheManager {
	if config == nil {
		config = DefaultConfig()
	}

	cm := &CacheManager{
		cache: &Cache{
			entries: make(map[string]*CacheEntry),
		},
		policies: make(map[string]EvictionPolicy),
		limits: &MemoryLimits{
			MaxSize:         config.MaxSize,
			MaxEntries:      config.MaxEntries,
			MemoryThreshold: config.MemoryThreshold,
			CheckInterval:   config.CheckInterval,
		},
		stats: &CacheStats{
			MaxSize:        config.MaxSize,
			EvictionPolicy: config.DefaultPolicy,
		},
		monitor: NewMemoryMonitor(config.CheckInterval),
		stopCh:  make(chan struct{}),
	}

	// Register built-in eviction policies
	cm.RegisterPolicy("lru", NewLRUPolicy())
	cm.RegisterPolicy("lfu", NewLFUPolicy())
	cm.RegisterPolicy("ttl", NewTTLPolicy())
	cm.RegisterPolicy("adaptive", NewAdaptivePolicy())

	// Start background processes
	cm.wg.Add(2)
	go cm.backgroundCleanup(config.CleanupInterval)
	go cm.memoryMonitor()

	return cm
}

// RegisterPolicy registers a custom eviction policy
func (cm *CacheManager) RegisterPolicy(name string, policy EvictionPolicy) {
	cm.policies[name] = policy
}

// Get retrieves a value from cache
func (cm *CacheManager) Get(ctx context.Context, key string) (any, bool) {
	cm.cache.mutex.RLock()
	entry, exists := cm.cache.entries[key]
	cm.cache.mutex.RUnlock()

	if !exists {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Check TTL expiration
	if cm.isExpired(entry) {
		cm.Delete(ctx, key)
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Update access statistics
	entry.mutex.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	entry.mutex.Unlock()

	// Notify eviction policy of access
	for _, policy := range cm.policies {
		policy.OnAccess(entry)
	}

	atomic.AddInt64(&cm.hitCount, 1)
	return entry.Value, true
}

// Put stores a value in cache
func (cm *CacheManager) Put(ctx context.Context, key string, value any, ttl time.Duration) error {
	size := cm.calculateSize(value)

	// Check if we need to evict before inserting
	if cm.needsEviction(size) {
		if err := cm.evict(size); err != nil {
			return fmt.Errorf("eviction failed: %w", err)
		}
	}

	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		Size:        size,
		AccessCount: 1,
		LastAccess:  time.Now(),
		CreatedAt:   time.Now(),
		TTL:         ttl,
		Priority:    1,
	}

	cm.cache.mutex.Lock()
	cm.cache.entries[key] = entry
	cm.cache.mutex.Unlock()

	// Notify eviction policies of insertion
	for _, policy := range cm.policies {
		policy.OnInsert(entry)
	}

	// Update stats
	atomic.AddInt64(&cm.stats.TotalEntries, 1)
	atomic.AddInt64(&cm.stats.TotalSize, size)

	return nil
}

// Delete removes a key from cache
func (cm *CacheManager) Delete(ctx context.Context, key string) bool {
	cm.cache.mutex.Lock()
	entry, exists := cm.cache.entries[key]
	if exists {
		delete(cm.cache.entries, key)
		atomic.AddInt64(&cm.stats.TotalEntries, -1)
		atomic.AddInt64(&cm.stats.TotalSize, -entry.Size)
	}
	cm.cache.mutex.Unlock()

	return exists
}

// Clear removes all entries from cache
func (cm *CacheManager) Clear(ctx context.Context) {
	cm.cache.mutex.Lock()
	cm.cache.entries = make(map[string]*CacheEntry)
	cm.cache.mutex.Unlock()

	atomic.StoreInt64(&cm.stats.TotalEntries, 0)
	atomic.StoreInt64(&cm.stats.TotalSize, 0)
}

// GetStats returns current cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	hits := atomic.LoadInt64(&cm.hitCount)
	misses := atomic.LoadInt64(&cm.missCount)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return CacheStats{
		TotalEntries:   atomic.LoadInt64(&cm.stats.TotalEntries),
		TotalSize:      atomic.LoadInt64(&cm.stats.TotalSize),
		MaxSize:        cm.limits.MaxSize,
		HitCount:       hits,
		MissCount:      misses,
		EvictionCount:  atomic.LoadInt64(&cm.evictionCount),
		HitRatio:       hitRatio,
		MemoryUsage:    int64(m.Alloc),
		SystemMemory:   int64(m.Sys),
		LastEviction:   cm.stats.LastEviction,
		EvictionPolicy: cm.stats.EvictionPolicy,
	}
}

// SetEvictionPolicy changes the active eviction policy
func (cm *CacheManager) SetEvictionPolicy(policyName string) error {
	if _, exists := cm.policies[policyName]; !exists {
		return fmt.Errorf("unknown eviction policy: %s", policyName)
	}
	cm.stats.EvictionPolicy = policyName
	return nil
}

// Close shuts down the cache manager
func (cm *CacheManager) Close() error {
	close(cm.stopCh)
	cm.wg.Wait()
	return nil
}

// Private methods

func (cm *CacheManager) isExpired(entry *CacheEntry) bool {
	if entry.TTL == 0 {
		return false
	}
	return time.Since(entry.CreatedAt) > entry.TTL
}

func (cm *CacheManager) calculateSize(value any) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		// Rough estimate for complex types
		return 1024
	}
}

func (cm *CacheManager) needsEviction(newSize int64) bool {
	currentSize := atomic.LoadInt64(&cm.stats.TotalSize)
	currentEntries := atomic.LoadInt64(&cm.stats.TotalEntries)

	// Check size limit
	if currentSize+newSize > cm.limits.MaxSize {
		return true
	}

	// Check entry count limit
	if int(currentEntries) >= cm.limits.MaxEntries {
		return true
	}

	// Check system memory threshold
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryUsage := float64(m.Alloc) / float64(m.Sys)
	return memoryUsage > cm.limits.MemoryThreshold
}

func (cm *CacheManager) evict(neededSize int64) error {
	policy, exists := cm.policies[cm.stats.EvictionPolicy]
	if !exists {
		policy = cm.policies["lru"] // Fallback to LRU
	}

	entries := policy.Evict(cm.cache, neededSize)

	cm.cache.mutex.Lock()
	var freedSize int64
	for i := range entries {
		entry := &entries[i]
		if _, exists := cm.cache.entries[entry.Key]; exists {
			delete(cm.cache.entries, entry.Key)
			freedSize += entry.Size
			atomic.AddInt64(&cm.evictionCount, 1)
		}
	}
	cm.cache.mutex.Unlock()

	atomic.AddInt64(&cm.stats.TotalEntries, -int64(len(entries)))
	atomic.AddInt64(&cm.stats.TotalSize, -freedSize)
	cm.stats.LastEviction = time.Now()

	if freedSize < neededSize {
		return fmt.Errorf("insufficient space freed: needed %d, freed %d", neededSize, freedSize)
	}

	return nil
}

func (cm *CacheManager) backgroundCleanup(interval time.Duration) {
	defer cm.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.cleanupExpired()
		case <-cm.stopCh:
			return
		}
	}
}

func (cm *CacheManager) cleanupExpired() {
	cm.cache.mutex.RLock()
	var expiredKeys []string
	for key, entry := range cm.cache.entries {
		if cm.isExpired(entry) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	cm.cache.mutex.RUnlock()

	if len(expiredKeys) > 0 {
		cm.cache.mutex.Lock()
		var freedSize int64
		for _, key := range expiredKeys {
			if entry, exists := cm.cache.entries[key]; exists {
				delete(cm.cache.entries, key)
				freedSize += entry.Size
			}
		}
		cm.cache.mutex.Unlock()

		atomic.AddInt64(&cm.stats.TotalEntries, -int64(len(expiredKeys)))
		atomic.AddInt64(&cm.stats.TotalSize, -freedSize)
	}
}

func (cm *CacheManager) memoryMonitor() {
	defer cm.wg.Done()
	ticker := time.NewTicker(cm.limits.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkMemoryPressure()
		case <-cm.stopCh:
			return
		}
	}
}

func (cm *CacheManager) checkMemoryPressure() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsage := float64(m.Alloc) / float64(m.Sys)
	if memoryUsage > cm.limits.MemoryThreshold {
		// Force eviction of 10% of cache
		targetSize := atomic.LoadInt64(&cm.stats.TotalSize) / 10
		cm.evict(targetSize)
	}
}

// LRU Policy Implementation
type LRUPolicy struct{}

func NewLRUPolicy() *LRUPolicy {
	return &LRUPolicy{}
}

func (p *LRUPolicy) Name() string {
	return "lru"
}

func (p *LRUPolicy) OnAccess(entry *CacheEntry) {
	// LRU updates are handled in the Get method
}

func (p *LRUPolicy) OnInsert(entry *CacheEntry) {
	// No special handling needed for LRU on insert
}

func (p *LRUPolicy) Evict(cache *Cache, needed int64) []CacheEntry {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	var entries []*CacheEntry
	for _, entry := range cache.entries {
		entries = append(entries, entry)
	}

	// Sort by last access time (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastAccess.Before(entries[j].LastAccess)
	})

	var result []CacheEntry
	var freedSize int64
	for _, entry := range entries {
		if freedSize >= needed {
			break
		}
		// Create a copy without the mutex
		result = append(result, CacheEntry{
			Key:         entry.Key,
			Value:       entry.Value,
			Size:        entry.Size,
			AccessCount: entry.AccessCount,
			LastAccess:  entry.LastAccess,
			CreatedAt:   entry.CreatedAt,
			TTL:         entry.TTL,
			Priority:    entry.Priority,
		})
		freedSize += entry.Size
	}

	return result
}

// LFU Policy Implementation
type LFUPolicy struct{}

func NewLFUPolicy() *LFUPolicy {
	return &LFUPolicy{}
}

func (p *LFUPolicy) Name() string {
	return "lfu"
}

func (p *LFUPolicy) OnAccess(entry *CacheEntry) {
	// Access count is updated in Get method
}

func (p *LFUPolicy) OnInsert(entry *CacheEntry) {
	// No special handling needed for LFU on insert
}

func (p *LFUPolicy) Evict(cache *Cache, needed int64) []CacheEntry {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	var entries []*CacheEntry
	for _, entry := range cache.entries {
		entries = append(entries, entry)
	}

	// Sort by access count (least frequent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AccessCount < entries[j].AccessCount
	})

	var result []CacheEntry
	var freedSize int64
	for _, entry := range entries {
		if freedSize >= needed {
			break
		}
		// Create a copy without the mutex
		result = append(result, CacheEntry{
			Key:         entry.Key,
			Value:       entry.Value,
			Size:        entry.Size,
			AccessCount: entry.AccessCount,
			LastAccess:  entry.LastAccess,
			CreatedAt:   entry.CreatedAt,
			TTL:         entry.TTL,
			Priority:    entry.Priority,
		})
		freedSize += entry.Size
	}

	return result
}

// TTL Policy Implementation
type TTLPolicy struct{}

func NewTTLPolicy() *TTLPolicy {
	return &TTLPolicy{}
}

func (p *TTLPolicy) Name() string {
	return "ttl"
}

func (p *TTLPolicy) OnAccess(entry *CacheEntry) {
	// No special handling for TTL on access
}

func (p *TTLPolicy) OnInsert(entry *CacheEntry) {
	// No special handling for TTL on insert
}

func (p *TTLPolicy) Evict(cache *Cache, needed int64) []CacheEntry {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	var entries []*CacheEntry

	for _, entry := range cache.entries {
		entries = append(entries, entry)
	}

	// Sort by expiration time (soonest to expire first)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TTL == 0 && entries[j].TTL == 0 {
			return entries[i].CreatedAt.Before(entries[j].CreatedAt)
		}
		if entries[i].TTL == 0 {
			return false
		}
		if entries[j].TTL == 0 {
			return true
		}

		expiryI := entries[i].CreatedAt.Add(entries[i].TTL)
		expiryJ := entries[j].CreatedAt.Add(entries[j].TTL)
		return expiryI.Before(expiryJ)
	})

	var result []CacheEntry
	var freedSize int64
	for _, entry := range entries {
		if freedSize >= needed {
			break
		}
		// Create a copy without the mutex
		result = append(result, CacheEntry{
			Key:         entry.Key,
			Value:       entry.Value,
			Size:        entry.Size,
			AccessCount: entry.AccessCount,
			LastAccess:  entry.LastAccess,
			CreatedAt:   entry.CreatedAt,
			TTL:         entry.TTL,
			Priority:    entry.Priority,
		})
		freedSize += entry.Size
	}

	return result
}

// Adaptive Policy Implementation (combines LRU and LFU)
type AdaptivePolicy struct {
	lru *LRUPolicy
	lfu *LFUPolicy
}

func NewAdaptivePolicy() *AdaptivePolicy {
	return &AdaptivePolicy{
		lru: NewLRUPolicy(),
		lfu: NewLFUPolicy(),
	}
}

func (p *AdaptivePolicy) Name() string {
	return "adaptive"
}

func (p *AdaptivePolicy) OnAccess(entry *CacheEntry) {
	p.lru.OnAccess(entry)
	p.lfu.OnAccess(entry)
}

func (p *AdaptivePolicy) OnInsert(entry *CacheEntry) {
	p.lru.OnInsert(entry)
	p.lfu.OnInsert(entry)
}

func (p *AdaptivePolicy) Evict(cache *Cache, needed int64) []CacheEntry {
	// Use LRU for 70% of evictions, LFU for 30%
	lruNeeded := (needed * 7) / 10
	lfuNeeded := needed - lruNeeded

	lruEntries := p.lru.Evict(cache, lruNeeded)
	lfuEntries := p.lfu.Evict(cache, lfuNeeded)

	// Combine and deduplicate
	entryMap := make(map[string]*CacheEntry)
	for i := range lruEntries {
		entry := &lruEntries[i]
		entryMap[entry.Key] = entry
	}
	for i := range lfuEntries {
		entry := &lfuEntries[i]
		entryMap[entry.Key] = entry
	}

	var result []CacheEntry
	for _, entry := range entryMap {
		// Create a copy without the mutex
		result = append(result, CacheEntry{
			Key:         entry.Key,
			Value:       entry.Value,
			Size:        entry.Size,
			AccessCount: entry.AccessCount,
			LastAccess:  entry.LastAccess,
			CreatedAt:   entry.CreatedAt,
			TTL:         entry.TTL,
			Priority:    entry.Priority,
		})
	}

	return result
}
