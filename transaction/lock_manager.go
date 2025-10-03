package transaction

import (
	"fmt"
	"sync"
	"time"
)

// LockRequest represents a pending lock request
type LockRequest struct {
	TxnID     uint64
	Resource  string
	LockType  LockType
	RequestAt time.Time
	Done      chan error // Channel to signal completion
}

// ResourceLock represents locks on a specific resource
type ResourceLock struct {
	Resource  string
	Holders   map[uint64]LockType // txnID -> lockType for current holders
	WaitQueue []*LockRequest      // Queue of waiting lock requests
	mutex     sync.RWMutex
}

// DefaultLockManager implements the LockManager interface
type DefaultLockManager struct {
	locks       map[string]*ResourceLock // resource -> ResourceLock
	txnLocks    map[uint64][]string      // txnID -> list of resources locked
	mutex       sync.RWMutex
	lockTimeout time.Duration
	closed      bool
}

// NewLockManager creates a new lock manager
func NewLockManager(lockTimeout time.Duration) *DefaultLockManager {
	if lockTimeout <= 0 {
		lockTimeout = 30 * time.Second // Default timeout
	}

	return &DefaultLockManager{
		locks:       make(map[string]*ResourceLock),
		txnLocks:    make(map[uint64][]string),
		lockTimeout: lockTimeout,
	}
}

// AcquireLock attempts to acquire a lock on a resource
func (lm *DefaultLockManager) AcquireLock(txnID uint64, resource string, lockType LockType) error {
	if lm.closed {
		return fmt.Errorf("lock manager is closed")
	}

	// Get or create resource lock
	resourceLock := lm.getOrCreateResourceLock(resource)

	// Try to acquire the lock immediately
	if lm.tryAcquireLock(resourceLock, txnID, lockType) {
		lm.addTxnLock(txnID, resource)
		return nil
	}

	// If immediate acquisition fails, add to wait queue
	request := &LockRequest{
		TxnID:     txnID,
		Resource:  resource,
		LockType:  lockType,
		RequestAt: time.Now(),
		Done:      make(chan error, 1),
	}

	resourceLock.mutex.Lock()
	resourceLock.WaitQueue = append(resourceLock.WaitQueue, request)
	resourceLock.mutex.Unlock()

	// Wait for lock acquisition or timeout
	select {
	case err := <-request.Done:
		if err == nil {
			lm.addTxnLock(txnID, resource)
		}
		return err
	case <-time.After(lm.lockTimeout):
		// Remove from wait queue on timeout
		lm.removeFromWaitQueue(resourceLock, request)
		return fmt.Errorf("lock acquisition timeout for transaction %d on resource %s", txnID, resource)
	}
}

// ReleaseLock releases a specific lock held by a transaction
func (lm *DefaultLockManager) ReleaseLock(txnID uint64, resource string) error {
	lm.mutex.RLock()
	resourceLock, exists := lm.locks[resource]
	lm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no locks found for resource %s", resource)
	}

	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	// Check if transaction holds the lock
	if _, holds := resourceLock.Holders[txnID]; !holds {
		return fmt.Errorf("transaction %d does not hold lock on resource %s", txnID, resource)
	}

	// Remove the lock
	delete(resourceLock.Holders, txnID)
	lm.removeTxnLock(txnID, resource)

	// Process waiting requests
	lm.processWaitQueue(resourceLock)

	return nil
}

// ReleaseAllLocks releases all locks held by a transaction
func (lm *DefaultLockManager) ReleaseAllLocks(txnID uint64) error {
	lm.mutex.RLock()
	resources, exists := lm.txnLocks[txnID]
	lm.mutex.RUnlock()

	if !exists {
		return nil // No locks held
	}

	// Make a copy to avoid modification during iteration
	resourcesCopy := make([]string, len(resources))
	copy(resourcesCopy, resources)

	var lastError error
	for _, resource := range resourcesCopy {
		if err := lm.ReleaseLock(txnID, resource); err != nil {
			lastError = err
		}
	}

	return lastError
}

// DetectDeadlocks detects deadlocks using wait-for graph
func (lm *DefaultLockManager) DetectDeadlocks() []DeadlockInfo {
	graph := lm.BuildWaitForGraph()
	cycles := lm.findCycles(graph)

	deadlocks := make([]DeadlockInfo, 0, len(cycles))
	for _, cycle := range cycles {
		if len(cycle) > 1 {
			// Choose victim (youngest transaction)
			victim := lm.chooseVictim(cycle)
			deadlocks = append(deadlocks, DeadlockInfo{
				Cycle:       cycle,
				VictimTxnID: victim,
				DetectedAt:  time.Now(),
			})
		}
	}

	return deadlocks
}

// BuildWaitForGraph builds a wait-for graph for deadlock detection
func (lm *DefaultLockManager) BuildWaitForGraph() *WaitForGraph {
	graph := &WaitForGraph{
		Edges: make(map[uint64][]uint64),
	}

	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	for _, resourceLock := range lm.locks {
		resourceLock.mutex.RLock()

		// For each waiting transaction, add edges to all holding transactions
		for _, request := range resourceLock.WaitQueue {
			waitingTxn := request.TxnID

			for holderTxn := range resourceLock.Holders {
				if waitingTxn != holderTxn {
					if graph.Edges[waitingTxn] == nil {
						graph.Edges[waitingTxn] = make([]uint64, 0)
					}
					graph.Edges[waitingTxn] = append(graph.Edges[waitingTxn], holderTxn)
				}
			}
		}

		resourceLock.mutex.RUnlock()
	}

	return graph
}

// GetLockInfo returns information about locks on a resource
func (lm *DefaultLockManager) GetLockInfo(resource string) *LockInfo {
	lm.mutex.RLock()
	resourceLock, exists := lm.locks[resource]
	lm.mutex.RUnlock()

	if !exists {
		return nil
	}

	resourceLock.mutex.RLock()
	defer resourceLock.mutex.RUnlock()

	info := &LockInfo{
		Resource:    resource,
		WaitingTxns: make([]uint64, 0, len(resourceLock.WaitQueue)),
	}

	// Get holder information (assuming single holder for simplicity)
	for txnID, lockType := range resourceLock.Holders {
		info.HolderTxnID = txnID
		info.LockType = lockType
		break // Take first holder
	}

	// Get waiting transactions
	for _, request := range resourceLock.WaitQueue {
		info.WaitingTxns = append(info.WaitingTxns, request.TxnID)
	}

	return info
}

// GetBlockedTransactions returns list of blocked transaction IDs
func (lm *DefaultLockManager) GetBlockedTransactions() []uint64 {
	blocked := make([]uint64, 0)

	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	for _, resourceLock := range lm.locks {
		resourceLock.mutex.RLock()
		for _, request := range resourceLock.WaitQueue {
			blocked = append(blocked, request.TxnID)
		}
		resourceLock.mutex.RUnlock()
	}

	return blocked
}

// Close closes the lock manager
func (lm *DefaultLockManager) Close() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.closed {
		return nil
	}

	// Cancel all waiting requests
	for _, resourceLock := range lm.locks {
		resourceLock.mutex.Lock()
		for _, request := range resourceLock.WaitQueue {
			select {
			case request.Done <- fmt.Errorf("lock manager is closing"):
			default:
			}
		}
		resourceLock.WaitQueue = nil
		resourceLock.mutex.Unlock()
	}

	lm.closed = true
	return nil
}

// Helper methods

func (lm *DefaultLockManager) getOrCreateResourceLock(resource string) *ResourceLock {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if resourceLock, exists := lm.locks[resource]; exists {
		return resourceLock
	}

	resourceLock := &ResourceLock{
		Resource:  resource,
		Holders:   make(map[uint64]LockType),
		WaitQueue: make([]*LockRequest, 0),
	}

	lm.locks[resource] = resourceLock
	return resourceLock
}

func (lm *DefaultLockManager) tryAcquireLock(resourceLock *ResourceLock, txnID uint64, lockType LockType) bool {
	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	// Check if transaction already holds a lock on this resource
	if existingLockType, holds := resourceLock.Holders[txnID]; holds {
		// If already holds the same or stronger lock, return true
		if existingLockType == lockType || (existingLockType == ExclusiveLock && lockType == SharedLock) {
			return true
		}
		// If trying to upgrade from shared to exclusive, check if only holder
		if existingLockType == SharedLock && lockType == ExclusiveLock && len(resourceLock.Holders) == 1 {
			resourceLock.Holders[txnID] = ExclusiveLock
			return true
		}
	}

	// If no holders, grant the lock
	if len(resourceLock.Holders) == 0 {
		resourceLock.Holders[txnID] = lockType
		return true
	}

	// If requesting shared lock and all holders have shared locks
	if lockType == SharedLock {
		for _, holderType := range resourceLock.Holders {
			if holderType == ExclusiveLock {
				return false
			}
		}
		resourceLock.Holders[txnID] = lockType
		return true
	}

	// If requesting exclusive lock, can only grant if no other holders
	if lockType == ExclusiveLock && len(resourceLock.Holders) == 0 {
		resourceLock.Holders[txnID] = lockType
		return true
	}

	return false
}

func (lm *DefaultLockManager) addTxnLock(txnID uint64, resource string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.txnLocks[txnID] == nil {
		lm.txnLocks[txnID] = make([]string, 0)
	}

	// Check if resource is already in the list to avoid duplicates
	for _, r := range lm.txnLocks[txnID] {
		if r == resource {
			return // Already exists
		}
	}

	lm.txnLocks[txnID] = append(lm.txnLocks[txnID], resource)
}

func (lm *DefaultLockManager) removeTxnLock(txnID uint64, resource string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	resources := lm.txnLocks[txnID]
	for i, r := range resources {
		if r == resource {
			lm.txnLocks[txnID] = append(resources[:i], resources[i+1:]...)
			break
		}
	}

	if len(lm.txnLocks[txnID]) == 0 {
		delete(lm.txnLocks, txnID)
	}
}

func (lm *DefaultLockManager) processWaitQueue(resourceLock *ResourceLock) {
	// Try to grant locks to waiting requests
	i := 0
	for i < len(resourceLock.WaitQueue) {
		request := resourceLock.WaitQueue[i]

		if lm.canGrantLock(resourceLock, request.LockType) {
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

func (lm *DefaultLockManager) canGrantLock(resourceLock *ResourceLock, lockType LockType) bool {
	if lockType == SharedLock {
		// Can grant shared lock if no exclusive locks
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

func (lm *DefaultLockManager) removeFromWaitQueue(resourceLock *ResourceLock, request *LockRequest) {
	resourceLock.mutex.Lock()
	defer resourceLock.mutex.Unlock()

	for i, r := range resourceLock.WaitQueue {
		if r == request {
			resourceLock.WaitQueue = append(resourceLock.WaitQueue[:i], resourceLock.WaitQueue[i+1:]...)
			break
		}
	}
}

func (lm *DefaultLockManager) findCycles(graph *WaitForGraph) [][]uint64 {
	visited := make(map[uint64]bool)
	recStack := make(map[uint64]bool)
	cycles := make([][]uint64, 0)

	var dfs func(uint64, []uint64) bool
	dfs = func(node uint64, path []uint64) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, neighbor := range graph.Edges[node] {
			if !visited[neighbor] {
				if dfs(neighbor, path) {
					return true
				}
			} else if recStack[neighbor] {
				// Found cycle
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]uint64, len(path)-cycleStart)
					copy(cycle, path[cycleStart:])
					cycles = append(cycles, cycle)
				}
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for node := range graph.Edges {
		if !visited[node] {
			dfs(node, make([]uint64, 0))
		}
	}

	return cycles
}

func (lm *DefaultLockManager) chooseVictim(cycle []uint64) uint64 {
	// Simple strategy: choose the transaction with highest ID (youngest)
	victim := cycle[0]
	for _, txnID := range cycle[1:] {
		if txnID > victim {
			victim = txnID
		}
	}
	return victim
}
