package cache

import (
	"sync"
	"time"
)

// TTLManager handles time-to-live for cache entries
type TTLManager struct {
	timers   map[string]*time.Timer
	mutex    sync.RWMutex
	interval time.Duration
	callback func(string) // Callback when TTL expires
}

// NewTTLManager creates a new TTL manager
func NewTTLManager(cleanupInterval time.Duration) *TTLManager {
	return &TTLManager{
		timers:   make(map[string]*time.Timer),
		interval: cleanupInterval,
	}
}

// SetCallback sets the callback function for TTL expiration
func (tm *TTLManager) SetCallback(callback func(string)) {
	tm.callback = callback
}

// Schedule schedules a key for expiration after the given TTL
func (tm *TTLManager) Schedule(key string, ttl time.Duration) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Cancel existing timer if any
	if timer, exists := tm.timers[key]; exists {
		timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(ttl, func() {
		tm.expire(key)
	})

	tm.timers[key] = timer
}

// Cancel cancels the TTL for a key
func (tm *TTLManager) Cancel(key string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if timer, exists := tm.timers[key]; exists {
		timer.Stop()
		delete(tm.timers, key)
	}
}

// Reschedule updates the TTL for a key
func (tm *TTLManager) Reschedule(key string, newTTL time.Duration) {
	tm.Schedule(key, newTTL)
}

// GetRemainingTTL returns the remaining TTL for a key
func (tm *TTLManager) GetRemainingTTL(key string) time.Duration {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// This is a simplified implementation
	// In a real implementation, you'd track creation time and calculate remaining time
	if _, exists := tm.timers[key]; exists {
		return time.Hour // Placeholder
	}
	return 0
}

// GetActiveKeys returns all keys with active TTL timers
func (tm *TTLManager) GetActiveKeys() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	keys := make([]string, 0, len(tm.timers))
	for key := range tm.timers {
		keys = append(keys, key)
	}
	return keys
}

// GetStats returns TTL manager statistics
func (tm *TTLManager) GetStats() TTLStats {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	return TTLStats{
		ActiveTimers: len(tm.timers),
	}
}

// TTLStats holds TTL manager statistics
type TTLStats struct {
	ActiveTimers int
}

// Cleanup removes all expired timers (called internally)
func (tm *TTLManager) Cleanup() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// In a real implementation, you'd check for expired timers
	// This is a simplified version
	expiredKeys := make([]string, 0)

	for key, timer := range tm.timers {
		// Check if timer has expired (simplified check)
		select {
		case <-timer.C:
			expiredKeys = append(expiredKeys, key)
		default:
			// Timer still active
		}
	}

	// Remove expired timers
	for _, key := range expiredKeys {
		delete(tm.timers, key)
	}
}

// Private methods

func (tm *TTLManager) expire(key string) {
	tm.mutex.Lock()
	delete(tm.timers, key)
	tm.mutex.Unlock()

	// Call callback if set
	if tm.callback != nil {
		tm.callback(key)
	}
}

// TTLEntry represents a TTL entry with expiration time
type TTLEntry struct {
	Key       string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsExpired checks if the entry has expired
func (e *TTLEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// RemainingTTL returns the remaining time to live
func (e *TTLEntry) RemainingTTL() time.Duration {
	remaining := time.Until(e.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// TTLHeap implements a min-heap for efficient TTL management
type TTLHeap []*TTLEntry

func (h TTLHeap) Len() int           { return len(h) }
func (h TTLHeap) Less(i, j int) bool { return h[i].ExpiresAt.Before(h[j].ExpiresAt) }
func (h TTLHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TTLHeap) Push(x interface{}) {
	*h = append(*h, x.(*TTLEntry))
}

func (h *TTLHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Peek returns the entry with the earliest expiration time
func (h TTLHeap) Peek() *TTLEntry {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}
