// Package concurrency provides enhanced concurrency control mechanisms
package concurrency

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// LockHierarchy enforces ordered lock acquisition to prevent deadlocks
type LockHierarchy struct {
	resourceOrder map[string]int // resource -> order number
	mutex         sync.RWMutex
	nextOrder     int32
}

// NewLockHierarchy creates a new lock hierarchy system
func NewLockHierarchy() *LockHierarchy {
	return &LockHierarchy{
		resourceOrder: make(map[string]int),
		nextOrder:     1,
	}
}

// GetResourceOrder returns the order number for a resource, creating one if needed
func (lh *LockHierarchy) GetResourceOrder(resource string) int {
	lh.mutex.RLock()
	if order, exists := lh.resourceOrder[resource]; exists {
		lh.mutex.RUnlock()
		return order
	}
	lh.mutex.RUnlock()

	// Need to create order - acquire write lock
	lh.mutex.Lock()
	defer lh.mutex.Unlock()

	// Double-check after acquiring write lock
	if order, exists := lh.resourceOrder[resource]; exists {
		return order
	}

	order := int(atomic.AddInt32(&lh.nextOrder, 1))
	lh.resourceOrder[resource] = order
	return order
}

// ValidateOrder checks if acquiring locks in the given order would violate hierarchy
func (lh *LockHierarchy) ValidateOrder(currentLocks []string, newResource string) error {
	if len(currentLocks) == 0 {
		return nil
	}

	newOrder := lh.GetResourceOrder(newResource)

	for _, resource := range currentLocks {
		existingOrder := lh.GetResourceOrder(resource)
		if newOrder <= existingOrder {
			return fmt.Errorf("lock hierarchy violation: attempting to acquire %s (order %d) while holding %s (order %d)",
				newResource, newOrder, resource, existingOrder)
		}
	}

	return nil
}

// FastPathLocks provides optimized locking for uncontended scenarios
type FastPathLocks struct {
	locks sync.Map // resource -> *FastLock
}

// FastLock represents a fast-path lock with minimal overhead
type FastLock struct {
	owner     uint64 // transaction ID
	lockType  LockType
	acquired  int64 // atomic timestamp
	contested int32 // atomic flag for contention
}

// NewFastPathLocks creates a new fast-path lock system
func NewFastPathLocks() *FastPathLocks {
	return &FastPathLocks{}
}

// TryFastLock attempts to acquire a lock using the fast path
func (fpl *FastPathLocks) TryFastLock(resource string, txnID uint64, lockType LockType) bool {
	now := time.Now().UnixNano()

	// Try to load existing lock
	if value, exists := fpl.locks.Load(resource); exists {
		fastLock := value.(*FastLock)

		// Mark as contested
		atomic.StoreInt32(&fastLock.contested, 1)
		return false
	}

	// Try to create new lock
	newLock := &FastLock{
		owner:    txnID,
		lockType: lockType,
		acquired: now,
	}

	// Use LoadOrStore for atomic creation
	if actual, loaded := fpl.locks.LoadOrStore(resource, newLock); loaded {
		// Someone else created it first
		actualLock := actual.(*FastLock)
		atomic.StoreInt32(&actualLock.contested, 1)
		return false
	}

	return true
}

// ReleaseFastLock releases a fast-path lock
func (fpl *FastPathLocks) ReleaseFastLock(resource string, txnID uint64) bool {
	value, exists := fpl.locks.Load(resource)
	if !exists {
		return false
	}

	fastLock := value.(*FastLock)
	if fastLock.owner != txnID {
		return false
	}

	fpl.locks.Delete(resource)
	return true
}

// IsContested checks if a lock is contested
func (fpl *FastPathLocks) IsContested(resource string) bool {
	value, exists := fpl.locks.Load(resource)
	if !exists {
		return false
	}

	fastLock := value.(*FastLock)
	return atomic.LoadInt32(&fastLock.contested) == 1
}

// LockPool manages reusable lock objects to reduce allocation overhead
type LockPool struct {
	requestPool sync.Pool
	lockPool    sync.Pool
}

// NewLockPool creates a new lock pool
func NewLockPool() *LockPool {
	return &LockPool{
		requestPool: sync.Pool{
			New: func() interface{} {
				return &LockRequest{
					Done: make(chan error, 1),
				}
			},
		},
		lockPool: sync.Pool{
			New: func() interface{} {
				return &ResourceLock{
					Holders:   make(map[uint64]LockType),
					WaitQueue: make([]*LockRequest, 0, 4),
				}
			},
		},
	}
}

// GetLockRequest gets a lock request from the pool
func (lp *LockPool) GetLockRequest() *LockRequest {
	req := lp.requestPool.Get().(*LockRequest)
	// Reset the request
	req.TxnID = 0
	req.Resource = ""
	req.LockType = 0
	req.RequestAt = time.Time{}
	// Clear the channel
	select {
	case <-req.Done:
	default:
	}
	return req
}

// PutLockRequest returns a lock request to the pool
func (lp *LockPool) PutLockRequest(req *LockRequest) {
	lp.requestPool.Put(req)
}

// GetResourceLock gets a resource lock from the pool
func (lp *LockPool) GetResourceLock() *ResourceLock {
	lock := lp.lockPool.Get().(*ResourceLock)
	// Reset the lock
	lock.Resource = ""
	for k := range lock.Holders {
		delete(lock.Holders, k)
	}
	lock.WaitQueue = lock.WaitQueue[:0]
	return lock
}

// PutResourceLock returns a resource lock to the pool
func (lp *LockPool) PutResourceLock(lock *ResourceLock) {
	lp.lockPool.Put(lock)
}

// EnhancedLockManager provides optimized lock management with deadlock prevention
type EnhancedLockManager struct {
	// Core components
	locks         sync.Map // resource -> *ResourceLock
	txnLocks      sync.Map // txnID -> []string (resources)
	lockHierarchy *LockHierarchy
	fastPath      *FastPathLocks
	lockPool      *LockPool

	// Configuration
	lockTimeout     time.Duration
	maxWaitQueue    int
	enableFastPath  bool
	enableHierarchy bool

	// Metrics and monitoring
	metrics *LockMetrics

	// Lifecycle
	closed int32 // atomic
}

// LockMetrics tracks lock performance metrics
type LockMetrics struct {
	// Counters
	locksAcquired     int64 // atomic
	locksReleased     int64 // atomic
	lockTimeouts      int64 // atomic
	deadlocksDetected int64 // atomic
	fastPathHits      int64 // atomic
	fastPathMisses    int64 // atomic

	// Timing
	totalWaitTime int64 // atomic, nanoseconds
	maxWaitTime   int64 // atomic, nanoseconds
	avgWaitTime   int64 // atomic, nanoseconds

	// Contention
	contentionEvents int64 // atomic
	queueDepthSum    int64 // atomic
	maxQueueDepth    int32 // atomic
}

// NewEnhancedLockManager creates a new enhanced lock manager
func NewEnhancedLockManager(config *LockManagerConfig) *EnhancedLockManager {
	if config == nil {
		config = DefaultLockManagerConfig()
	}

	return &EnhancedLockManager{
		lockHierarchy:   NewLockHierarchy(),
		fastPath:        NewFastPathLocks(),
		lockPool:        NewLockPool(),
		lockTimeout:     config.LockTimeout,
		maxWaitQueue:    config.MaxWaitQueue,
		enableFastPath:  config.EnableFastPath,
		enableHierarchy: config.EnableHierarchy,
		metrics:         &LockMetrics{},
	}
}

// LockManagerConfig holds configuration for the enhanced lock manager
type LockManagerConfig struct {
	LockTimeout     time.Duration
	MaxWaitQueue    int
	EnableFastPath  bool
	EnableHierarchy bool
}

// DefaultLockManagerConfig returns default configuration
func DefaultLockManagerConfig() *LockManagerConfig {
	return &LockManagerConfig{
		LockTimeout:     30 * time.Second,
		MaxWaitQueue:    100,
		EnableFastPath:  true,
		EnableHierarchy: true,
	}
}

// AcquireLock attempts to acquire a lock with enhanced optimizations
func (elm *EnhancedLockManager) AcquireLock(txnID uint64, resource string, lockType LockType) error {
	if atomic.LoadInt32(&elm.closed) == 1 {
		return fmt.Errorf("lock manager is closed")
	}

	startTime := time.Now()

	// Validate lock hierarchy if enabled
	if elm.enableHierarchy {
		currentLocks := elm.getCurrentLocks(txnID)
		if err := elm.lockHierarchy.ValidateOrder(currentLocks, resource); err != nil {
			return fmt.Errorf("hierarchy validation failed: %w", err)
		}
	}

	// Try fast path first if enabled
	if elm.enableFastPath {
		if elm.fastPath.TryFastLock(resource, txnID, lockType) {
			atomic.AddInt64(&elm.metrics.fastPathHits, 1)
			atomic.AddInt64(&elm.metrics.locksAcquired, 1)
			elm.addTxnLock(txnID, resource)
			return nil
		}
		atomic.AddInt64(&elm.metrics.fastPathMisses, 1)
	}

	// Fall back to regular locking mechanism
	return elm.acquireRegularLock(txnID, resource, lockType, startTime)
}

// acquireRegularLock handles the regular (non-fast-path) lock acquisition
func (elm *EnhancedLockManager) acquireRegularLock(txnID uint64, resource string, lockType LockType, startTime time.Time) error {
	// Get or create resource lock
	resourceLock := elm.getOrCreateResourceLock(resource)

	// Try immediate acquisition
	if elm.tryAcquireLock(resourceLock, txnID, lockType) {
		atomic.AddInt64(&elm.metrics.locksAcquired, 1)
		elm.addTxnLock(txnID, resource)
		return nil
	}

	// Check wait queue limit
	resourceLock.mutex.RLock()
	queueLen := len(resourceLock.WaitQueue)
	resourceLock.mutex.RUnlock()

	if queueLen >= elm.maxWaitQueue {
		return fmt.Errorf("wait queue full for resource %s (max: %d)", resource, elm.maxWaitQueue)
	}

	// Add to wait queue
	request := elm.lockPool.GetLockRequest()
	request.TxnID = txnID
	request.Resource = resource
	request.LockType = lockType
	request.RequestAt = startTime

	resourceLock.mutex.Lock()
	resourceLock.WaitQueue = append(resourceLock.WaitQueue, request)
	currentQueueDepth := int32(len(resourceLock.WaitQueue))
	resourceLock.mutex.Unlock()

	// Update queue depth metrics
	atomic.AddInt64(&elm.metrics.queueDepthSum, int64(currentQueueDepth))
	for {
		current := atomic.LoadInt32(&elm.metrics.maxQueueDepth)
		if currentQueueDepth <= current || atomic.CompareAndSwapInt32(&elm.metrics.maxQueueDepth, current, currentQueueDepth) {
			break
		}
	}

	// Wait for lock acquisition or timeout
	select {
	case err := <-request.Done:
		waitTime := time.Since(startTime).Nanoseconds()
		atomic.AddInt64(&elm.metrics.totalWaitTime, waitTime)

		// Update max wait time
		for {
			current := atomic.LoadInt64(&elm.metrics.maxWaitTime)
			if waitTime <= current || atomic.CompareAndSwapInt64(&elm.metrics.maxWaitTime, current, waitTime) {
				break
			}
		}

		elm.lockPool.PutLockRequest(request)

		if err == nil {
			atomic.AddInt64(&elm.metrics.locksAcquired, 1)
			elm.addTxnLock(txnID, resource)
		}
		return err

	case <-time.After(elm.lockTimeout):
		// Remove from wait queue on timeout
		elm.removeFromWaitQueue(resourceLock, request)
		elm.lockPool.PutLockRequest(request)
		atomic.AddInt64(&elm.metrics.lockTimeouts, 1)
		return fmt.Errorf("lock acquisition timeout for transaction %d on resource %s", txnID, resource)
	}
}

// ReleaseLock releases a specific lock with optimizations
func (elm *EnhancedLockManager) ReleaseLock(txnID uint64, resource string) error {
	// Try fast path release first
	if elm.enableFastPath && elm.fastPath.ReleaseFastLock(resource, txnID) {
		atomic.AddInt64(&elm.metrics.locksReleased, 1)
		elm.removeTxnLock(txnID, resource)
		return nil
	}

	// Regular release path
	value, exists := elm.locks.Load(resource)
	if !exists {
		return fmt.Errorf("no locks found for resource %s", resource)
	}

	resourceLock := value.(*ResourceLock)
	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	// Check if transaction holds the lock
	if _, holds := resourceLock.Holders[txnID]; !holds {
		return fmt.Errorf("transaction %d does not hold lock on resource %s", txnID, resource)
	}

	// Remove the lock
	delete(resourceLock.Holders, txnID)
	elm.removeTxnLock(txnID, resource)
	atomic.AddInt64(&elm.metrics.locksReleased, 1)

	// Process waiting requests
	elm.processWaitQueue(resourceLock)

	// Clean up empty resource lock
	if len(resourceLock.Holders) == 0 && len(resourceLock.WaitQueue) == 0 {
		elm.locks.Delete(resource)
		elm.lockPool.PutResourceLock(resourceLock)
	}

	return nil
}

// ReleaseAllLocks releases all locks held by a transaction
func (elm *EnhancedLockManager) ReleaseAllLocks(txnID uint64) error {
	resources := elm.getCurrentLocks(txnID)
	if len(resources) == 0 {
		return nil
	}

	var lastError error
	for _, resource := range resources {
		if err := elm.ReleaseLock(txnID, resource); err != nil {
			lastError = err
		}
	}

	return lastError
}

// Helper methods

func (elm *EnhancedLockManager) getCurrentLocks(txnID uint64) []string {
	value, exists := elm.txnLocks.Load(txnID)
	if !exists {
		return nil
	}

	locks := value.([]string)
	result := make([]string, len(locks))
	copy(result, locks)
	return result
}

func (elm *EnhancedLockManager) addTxnLock(txnID uint64, resource string) {
	for {
		value, exists := elm.txnLocks.Load(txnID)
		var currentLocks []string

		if exists {
			currentLocks = value.([]string)
			// Check if resource already exists
			for _, r := range currentLocks {
				if r == resource {
					return // Already exists
				}
			}
		}

		newLocks := make([]string, len(currentLocks)+1)
		copy(newLocks, currentLocks)
		newLocks[len(currentLocks)] = resource

		if !exists {
			if _, loaded := elm.txnLocks.LoadOrStore(txnID, newLocks); !loaded {
				return // Successfully stored
			}
		} else {
			if elm.txnLocks.CompareAndSwap(txnID, currentLocks, newLocks) {
				return // Successfully updated
			}
		}
		// Retry if CAS failed
		runtime.Gosched()
	}
}

func (elm *EnhancedLockManager) removeTxnLock(txnID uint64, resource string) {
	for {
		value, exists := elm.txnLocks.Load(txnID)
		if !exists {
			return
		}

		currentLocks := value.([]string)
		newLocks := make([]string, 0, len(currentLocks))
		found := false

		for _, r := range currentLocks {
			if r != resource {
				newLocks = append(newLocks, r)
			} else {
				found = true
			}
		}

		if !found {
			return // Resource not found
		}

		if len(newLocks) == 0 {
			if elm.txnLocks.CompareAndSwap(txnID, currentLocks, nil) {
				elm.txnLocks.Delete(txnID)
				return
			}
		} else {
			if elm.txnLocks.CompareAndSwap(txnID, currentLocks, newLocks) {
				return
			}
		}
		// Retry if CAS failed
		runtime.Gosched()
	}
}

func (elm *EnhancedLockManager) getOrCreateResourceLock(resource string) *ResourceLock {
	// Try to load existing lock
	if value, exists := elm.locks.Load(resource); exists {
		return value.(*ResourceLock)
	}

	// Create new lock
	newLock := elm.lockPool.GetResourceLock()
	newLock.Resource = resource

	// Try to store it
	if actual, loaded := elm.locks.LoadOrStore(resource, newLock); loaded {
		// Someone else created it, return theirs and put ours back
		elm.lockPool.PutResourceLock(newLock)
		return actual.(*ResourceLock)
	}

	return newLock
}

func (elm *EnhancedLockManager) tryAcquireLock(resourceLock *ResourceLock, txnID uint64, lockType LockType) bool {
	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	// Check if transaction already holds a lock on this resource
	if existingLockType, holds := resourceLock.Holders[txnID]; holds {
		// If already holds the same or stronger lock, return true
		if existingLockType == lockType || (existingLockType == ExclusiveLock && lockType == ReadLock) {
			return true
		}
		// If trying to upgrade from read to exclusive, check if only holder
		if existingLockType == ReadLock && lockType == ExclusiveLock && len(resourceLock.Holders) == 1 {
			resourceLock.Holders[txnID] = ExclusiveLock
			return true
		}
	}

	// If no holders, grant the lock
	if len(resourceLock.Holders) == 0 {
		resourceLock.Holders[txnID] = lockType
		return true
	}

	// If requesting read lock and all holders have read locks
	if lockType == ReadLock {
		for _, holderType := range resourceLock.Holders {
			if holderType == ExclusiveLock {
				return false
			}
		}
		resourceLock.Holders[txnID] = lockType
		return true
	}

	return false
}

func (elm *EnhancedLockManager) processWaitQueue(resourceLock *ResourceLock) {
	i := 0
	for i < len(resourceLock.WaitQueue) {
		request := resourceLock.WaitQueue[i]

		if elm.canGrantLock(resourceLock, request.LockType) {
			// Grant the lock
			resourceLock.Holders[request.TxnID] = request.LockType

			// Remove from wait queue
			resourceLock.WaitQueue = append(resourceLock.WaitQueue[:i], resourceLock.WaitQueue[i+1:]...)

			// Signal completion
			select {
			case request.Done <- nil:
			default:
			}
		} else {
			i++
		}
	}
}

func (elm *EnhancedLockManager) canGrantLock(resourceLock *ResourceLock, lockType LockType) bool {
	if lockType == ReadLock {
		// Can grant read lock if no exclusive locks
		for _, holderType := range resourceLock.Holders {
			if holderType == ExclusiveLock {
				return false
			}
		}
		return true
	}

	// Can grant exclusive lock only if no other holders
	return len(resourceLock.Holders) == 0
}

func (elm *EnhancedLockManager) removeFromWaitQueue(resourceLock *ResourceLock, request *LockRequest) {
	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	for i, r := range resourceLock.WaitQueue {
		if r == request {
			resourceLock.WaitQueue = append(resourceLock.WaitQueue[:i], resourceLock.WaitQueue[i+1:]...)
			break
		}
	}
}

// GetMetrics returns current lock metrics
func (elm *EnhancedLockManager) GetMetrics() *LockMetrics {
	// Create a copy to avoid race conditions
	return &LockMetrics{
		locksAcquired:     atomic.LoadInt64(&elm.metrics.locksAcquired),
		locksReleased:     atomic.LoadInt64(&elm.metrics.locksReleased),
		lockTimeouts:      atomic.LoadInt64(&elm.metrics.lockTimeouts),
		deadlocksDetected: atomic.LoadInt64(&elm.metrics.deadlocksDetected),
		fastPathHits:      atomic.LoadInt64(&elm.metrics.fastPathHits),
		fastPathMisses:    atomic.LoadInt64(&elm.metrics.fastPathMisses),
		totalWaitTime:     atomic.LoadInt64(&elm.metrics.totalWaitTime),
		maxWaitTime:       atomic.LoadInt64(&elm.metrics.maxWaitTime),
		contentionEvents:  atomic.LoadInt64(&elm.metrics.contentionEvents),
		queueDepthSum:     atomic.LoadInt64(&elm.metrics.queueDepthSum),
		maxQueueDepth:     atomic.LoadInt32(&elm.metrics.maxQueueDepth),
	}
}

// Close closes the enhanced lock manager
func (elm *EnhancedLockManager) Close() error {
	if !atomic.CompareAndSwapInt32(&elm.closed, 0, 1) {
		return nil // Already closed
	}

	// Cancel all waiting requests
	elm.locks.Range(func(key, value interface{}) bool {
		resourceLock := value.(*ResourceLock)
		resourceLock.mutex.Lock()
		for _, request := range resourceLock.WaitQueue {
			select {
			case request.Done <- fmt.Errorf("lock manager is closing"):
			default:
			}
		}
		resourceLock.WaitQueue = nil
		resourceLock.mutex.Unlock()
		return true
	})

	return nil
}

// ResourceLock represents locks on a specific resource (reused from existing code)
type ResourceLock struct {
	Resource  string
	Holders   map[uint64]LockType
	WaitQueue []*LockRequest
	mutex     sync.RWMutex
}

// LockRequest represents a pending lock request (reused from existing code)
type LockRequest struct {
	TxnID     uint64
	Resource  string
	LockType  LockType
	RequestAt time.Time
	Done      chan error
}

// Note: LockType is already defined in the interfaces.go file
