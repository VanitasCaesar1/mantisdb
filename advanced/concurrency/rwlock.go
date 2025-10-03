package concurrency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// LockPriority defines the priority level for lock requests
type LockPriority int

const (
	LowPriority LockPriority = iota
	NormalPriority
	HighPriority
)

// LockRequest represents a pending lock request with priority and timeout
type LockRequest struct {
	ID        uint64
	Resource  string
	IsWrite   bool
	Priority  LockPriority
	Timeout   time.Duration
	RequestAt time.Time
	Done      chan error
	Context   context.Context
}

// RWLock represents a read-write lock with writer preference
type RWLock struct {
	resource  string
	readers   int32          // Number of active readers
	writers   int32          // Number of active writers (0 or 1)
	waitingW  int32          // Number of waiting writers
	readerSem chan struct{}  // Semaphore for readers
	writerSem chan struct{}  // Semaphore for writers
	waitQueue []*LockRequest // Priority queue for waiting requests
	mutex     sync.Mutex     // Protects the lock state
	metrics   *LockMetrics   // Performance metrics
}

// LockMetrics tracks performance metrics for locks
type LockMetrics struct {
	AcquisitionCount int64
	ContentionCount  int64
	AverageWaitTime  time.Duration
	MaxWaitTime      time.Duration
	TimeoutCount     int64
	DeadlockCount    int64
	mutex            sync.RWMutex
	totalWaitTime    time.Duration
	waitTimeCount    int64
}

// RWLockManager manages read-write locks with deadlock detection
type RWLockManager struct {
	locks       map[string]*RWLock
	lockMetrics map[string]*LockMetrics
	mutex       sync.RWMutex
	nextReqID   uint64
	closed      bool

	// Configuration
	defaultTimeout    time.Duration
	enablePriority    bool
	enableDeadlockDet bool

	// Deadlock detection
	deadlockDetector *DeadlockDetector
}

// DeadlockDetector handles deadlock detection and resolution
type DeadlockDetector struct {
	manager     *RWLockManager
	checkPeriod time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewRWLockManager creates a new read-write lock manager
func NewRWLockManager() *RWLockManager {
	manager := &RWLockManager{
		locks:             make(map[string]*RWLock),
		lockMetrics:       make(map[string]*LockMetrics),
		defaultTimeout:    30 * time.Second,
		enablePriority:    true,
		enableDeadlockDet: true,
	}

	if manager.enableDeadlockDet {
		manager.deadlockDetector = &DeadlockDetector{
			manager:     manager,
			checkPeriod: 5 * time.Second,
			stopCh:      make(chan struct{}),
		}
		manager.deadlockDetector.start()
	}

	return manager
}

// AcquireReadLock acquires a read lock on the specified resource
func (rm *RWLockManager) AcquireReadLock(ctx context.Context, resource string) error {
	return rm.acquireLock(ctx, resource, false, NormalPriority, rm.defaultTimeout)
}

// AcquireWriteLock acquires a write lock on the specified resource
func (rm *RWLockManager) AcquireWriteLock(ctx context.Context, resource string) error {
	return rm.acquireLock(ctx, resource, true, NormalPriority, rm.defaultTimeout)
}

// AcquireReadLockWithPriority acquires a read lock with specified priority and timeout
func (rm *RWLockManager) AcquireReadLockWithPriority(ctx context.Context, resource string, priority LockPriority, timeout time.Duration) error {
	return rm.acquireLock(ctx, resource, false, priority, timeout)
}

// AcquireWriteLockWithPriority acquires a write lock with specified priority and timeout
func (rm *RWLockManager) AcquireWriteLockWithPriority(ctx context.Context, resource string, priority LockPriority, timeout time.Duration) error {
	return rm.acquireLock(ctx, resource, true, priority, timeout)
}

// ReleaseLock releases a lock on the specified resource
func (rm *RWLockManager) ReleaseLock(resource string, isWrite bool) error {
	if rm.closed {
		return fmt.Errorf("lock manager is closed")
	}

	rm.mutex.RLock()
	rwlock, exists := rm.locks[resource]
	rm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no lock found for resource: %s", resource)
	}

	return rm.releaseLock(rwlock, isWrite)
}

// GetLockMetrics returns performance metrics for a resource
func (rm *RWLockManager) GetLockMetrics(resource string) *LockMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if metrics, exists := rm.lockMetrics[resource]; exists {
		// Return a copy to avoid race conditions
		metrics.mutex.RLock()
		defer metrics.mutex.RUnlock()

		return &LockMetrics{
			AcquisitionCount: metrics.AcquisitionCount,
			ContentionCount:  metrics.ContentionCount,
			AverageWaitTime:  metrics.AverageWaitTime,
			MaxWaitTime:      metrics.MaxWaitTime,
			TimeoutCount:     metrics.TimeoutCount,
			DeadlockCount:    metrics.DeadlockCount,
		}
	}

	return nil
}

// GetAllMetrics returns metrics for all resources
func (rm *RWLockManager) GetAllMetrics() map[string]*LockMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	result := make(map[string]*LockMetrics)
	for resource := range rm.lockMetrics {
		result[resource] = rm.GetLockMetrics(resource)
	}

	return result
}

// Close closes the lock manager and releases all resources
func (rm *RWLockManager) Close() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if rm.closed {
		return nil
	}

	rm.closed = true

	// Stop deadlock detector
	if rm.deadlockDetector != nil {
		rm.deadlockDetector.stop()
	}

	// Cancel all waiting requests
	for _, rwlock := range rm.locks {
		rwlock.mutex.Lock()
		for _, request := range rwlock.waitQueue {
			select {
			case request.Done <- fmt.Errorf("lock manager is closing"):
			default:
			}
		}
		rwlock.waitQueue = nil
		rwlock.mutex.Unlock()
	}

	return nil
}

// Private methods

func (rm *RWLockManager) acquireLock(ctx context.Context, resource string, isWrite bool, priority LockPriority, timeout time.Duration) error {
	if rm.closed {
		return fmt.Errorf("lock manager is closed")
	}

	rwlock := rm.getOrCreateLock(resource)

	// Try immediate acquisition
	if rm.tryAcquireLock(rwlock, isWrite) {
		rm.updateMetrics(resource, 0, false)
		return nil
	}

	// Create lock request
	request := &LockRequest{
		ID:        atomic.AddUint64(&rm.nextReqID, 1),
		Resource:  resource,
		IsWrite:   isWrite,
		Priority:  priority,
		Timeout:   timeout,
		RequestAt: time.Now(),
		Done:      make(chan error, 1),
		Context:   ctx,
	}

	// Add to wait queue
	rm.addToWaitQueue(rwlock, request)

	// Wait for acquisition or timeout
	select {
	case err := <-request.Done:
		waitTime := time.Since(request.RequestAt)
		rm.updateMetrics(resource, waitTime, err != nil)
		return err
	case <-time.After(timeout):
		rm.removeFromWaitQueue(rwlock, request)
		rm.updateMetrics(resource, timeout, true)
		return fmt.Errorf("lock acquisition timeout for resource: %s", resource)
	case <-ctx.Done():
		rm.removeFromWaitQueue(rwlock, request)
		return ctx.Err()
	}
}

func (rm *RWLockManager) getOrCreateLock(resource string) *RWLock {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if rwlock, exists := rm.locks[resource]; exists {
		return rwlock
	}

	rwlock := &RWLock{
		resource:  resource,
		readerSem: make(chan struct{}, 1),
		writerSem: make(chan struct{}, 1),
		waitQueue: make([]*LockRequest, 0),
		metrics:   &LockMetrics{},
	}

	rm.locks[resource] = rwlock
	rm.lockMetrics[resource] = rwlock.metrics

	return rwlock
}

func (rm *RWLockManager) tryAcquireLock(rwlock *RWLock, isWrite bool) bool {
	rwlock.mutex.Lock()
	defer rwlock.mutex.Unlock()

	if isWrite {
		// Writer can acquire if no readers and no writers
		if atomic.LoadInt32(&rwlock.readers) == 0 &&
			atomic.LoadInt32(&rwlock.writers) == 0 {
			atomic.StoreInt32(&rwlock.writers, 1)
			return true
		}
	} else {
		// Reader can acquire if no writers and no waiting writers (writer preference)
		if atomic.LoadInt32(&rwlock.writers) == 0 &&
			atomic.LoadInt32(&rwlock.waitingW) == 0 {
			atomic.AddInt32(&rwlock.readers, 1)
			return true
		}
	}

	return false
}

func (rm *RWLockManager) addToWaitQueue(rwlock *RWLock, request *LockRequest) {
	rwlock.mutex.Lock()
	defer rwlock.mutex.Unlock()

	if request.IsWrite {
		atomic.AddInt32(&rwlock.waitingW, 1)
	}

	// Insert request based on priority (higher priority first)
	inserted := false
	for i, existing := range rwlock.waitQueue {
		if request.Priority > existing.Priority {
			rwlock.waitQueue = append(rwlock.waitQueue[:i], append([]*LockRequest{request}, rwlock.waitQueue[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		rwlock.waitQueue = append(rwlock.waitQueue, request)
	}
}

func (rm *RWLockManager) removeFromWaitQueue(rwlock *RWLock, request *LockRequest) {
	rwlock.mutex.Lock()
	defer rwlock.mutex.Unlock()

	for i, r := range rwlock.waitQueue {
		if r.ID == request.ID {
			rwlock.waitQueue = append(rwlock.waitQueue[:i], rwlock.waitQueue[i+1:]...)
			if request.IsWrite {
				atomic.AddInt32(&rwlock.waitingW, -1)
			}
			break
		}
	}
}

func (rm *RWLockManager) releaseLock(rwlock *RWLock, isWrite bool) error {
	rwlock.mutex.Lock()
	defer rwlock.mutex.Unlock()

	if isWrite {
		if atomic.LoadInt32(&rwlock.writers) == 0 {
			return fmt.Errorf("no write lock to release")
		}
		atomic.StoreInt32(&rwlock.writers, 0)
	} else {
		if atomic.LoadInt32(&rwlock.readers) == 0 {
			return fmt.Errorf("no read lock to release")
		}
		atomic.AddInt32(&rwlock.readers, -1)
	}

	// Process wait queue
	rm.processWaitQueue(rwlock)

	return nil
}

func (rm *RWLockManager) processWaitQueue(rwlock *RWLock) {
	// Process waiting requests in priority order
	i := 0
	for i < len(rwlock.waitQueue) {
		request := rwlock.waitQueue[i]

		canGrant := false
		if request.IsWrite {
			// Writer can be granted if no readers and no writers
			canGrant = atomic.LoadInt32(&rwlock.readers) == 0 &&
				atomic.LoadInt32(&rwlock.writers) == 0
		} else {
			// Reader can be granted if no writers
			canGrant = atomic.LoadInt32(&rwlock.writers) == 0
		}

		if canGrant {
			// Grant the lock
			if request.IsWrite {
				atomic.StoreInt32(&rwlock.writers, 1)
				atomic.AddInt32(&rwlock.waitingW, -1)
			} else {
				atomic.AddInt32(&rwlock.readers, 1)
			}

			// Remove from wait queue
			rwlock.waitQueue = append(rwlock.waitQueue[:i], rwlock.waitQueue[i+1:]...)

			// Signal completion
			select {
			case request.Done <- nil:
			default:
			}

			// Don't increment i since we removed an element
		} else {
			i++
		}
	}
}

func (rm *RWLockManager) updateMetrics(resource string, waitTime time.Duration, isTimeout bool) {
	rm.mutex.RLock()
	metrics, exists := rm.lockMetrics[resource]
	rm.mutex.RUnlock()

	if !exists {
		return
	}

	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	atomic.AddInt64(&metrics.AcquisitionCount, 1)

	if waitTime > 0 {
		atomic.AddInt64(&metrics.ContentionCount, 1)
		metrics.totalWaitTime += waitTime
		metrics.waitTimeCount++
		metrics.AverageWaitTime = metrics.totalWaitTime / time.Duration(metrics.waitTimeCount)

		if waitTime > metrics.MaxWaitTime {
			metrics.MaxWaitTime = waitTime
		}
	}

	if isTimeout {
		atomic.AddInt64(&metrics.TimeoutCount, 1)
	}
}

// Deadlock detector methods

func (dd *DeadlockDetector) start() {
	dd.wg.Add(1)
	go dd.run()
}

func (dd *DeadlockDetector) stop() {
	close(dd.stopCh)
	dd.wg.Wait()
}

func (dd *DeadlockDetector) run() {
	defer dd.wg.Done()

	ticker := time.NewTicker(dd.checkPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dd.detectAndResolveDeadlocks()
		case <-dd.stopCh:
			return
		}
	}
}

func (dd *DeadlockDetector) detectAndResolveDeadlocks() {
	// Build wait-for graph and detect cycles
	graph := dd.buildWaitForGraph()
	cycles := dd.findCycles(graph)

	for _, cycle := range cycles {
		if len(cycle) > 1 {
			deadlock := DeadlockInfo{
				Cycle:      cycle,
				Resources:  dd.getResourcesInCycle(cycle),
				DetectedAt: time.Now(),
			}

			// Select victim and resolve deadlock
			victim := dd.selectVictim(cycle, YoungestRequest)
			deadlock.VictimReqID = victim

			dd.resolveDeadlock(deadlock)
		}
	}

	// Also check for long-waiting requests that might indicate deadlocks
	dd.checkLongWaitingRequests()
}

// WaitForGraph represents the wait-for graph for deadlock detection
type WaitForGraph struct {
	Edges map[uint64][]uint64 // requestID -> list of requestIDs it's waiting for
	mutex sync.RWMutex
}

// DeadlockInfo contains information about a detected deadlock
type DeadlockInfo struct {
	Cycle       []uint64 // Request IDs in the deadlock cycle
	Resources   []string // Resources involved in the deadlock
	VictimReqID uint64   // Request chosen to be aborted
	DetectedAt  time.Time
}

// VictimSelectionStrategy defines different strategies for selecting deadlock victims
type VictimSelectionStrategy int

const (
	YoungestRequest VictimSelectionStrategy = iota
	OldestRequest
	LowestPriority
	HighestPriority
	RandomRequest
)

// Enhanced deadlock detection methods

func (dd *DeadlockDetector) buildWaitForGraph() *WaitForGraph {
	graph := &WaitForGraph{
		Edges: make(map[uint64][]uint64),
	}

	dd.manager.mutex.RLock()
	defer dd.manager.mutex.RUnlock()

	// Build wait-for relationships
	for _, rwlock := range dd.manager.locks {
		rwlock.mutex.Lock()

		// Map of active holders (readers and writers)
		activeHolders := make([]uint64, 0)

		// Add current readers and writers as "virtual" request IDs
		// In a real implementation, you'd track actual request IDs for active locks
		if atomic.LoadInt32(&rwlock.readers) > 0 {
			// For simplicity, use resource hash as reader ID
			readerID := dd.hashResource(rwlock.resource + "_readers")
			activeHolders = append(activeHolders, readerID)
		}

		if atomic.LoadInt32(&rwlock.writers) > 0 {
			// For simplicity, use resource hash as writer ID
			writerID := dd.hashResource(rwlock.resource + "_writer")
			activeHolders = append(activeHolders, writerID)
		}

		// For each waiting request, add edges to conflicting holders
		for _, request := range rwlock.waitQueue {
			waitingID := request.ID

			for _, holderID := range activeHolders {
				if graph.Edges[waitingID] == nil {
					graph.Edges[waitingID] = make([]uint64, 0)
				}
				graph.Edges[waitingID] = append(graph.Edges[waitingID], holderID)
			}

			// Add edges between conflicting waiting requests
			for _, otherRequest := range rwlock.waitQueue {
				if request.ID != otherRequest.ID && dd.requestsConflict(request, otherRequest) {
					if graph.Edges[request.ID] == nil {
						graph.Edges[request.ID] = make([]uint64, 0)
					}
					graph.Edges[request.ID] = append(graph.Edges[request.ID], otherRequest.ID)
				}
			}
		}

		rwlock.mutex.Unlock()
	}

	return graph
}

func (dd *DeadlockDetector) findCycles(graph *WaitForGraph) [][]uint64 {
	visited := make(map[uint64]int) // 0: unvisited, 1: visiting, 2: visited
	cycles := make([][]uint64, 0)

	var dfs func(uint64, []uint64)
	dfs = func(node uint64, path []uint64) {
		visited[node] = 1 // Mark as visiting
		path = append(path, node)

		for _, neighbor := range graph.Edges[node] {
			switch visited[neighbor] {
			case 0: // Unvisited
				dfs(neighbor, path)
			case 1: // Currently visiting - cycle detected
				// Find the start of the cycle
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
			case 2: // Already visited - no cycle through this path
				continue
			}
		}

		visited[node] = 2 // Mark as visited
	}

	// Start DFS from all unvisited nodes
	for node := range graph.Edges {
		if visited[node] == 0 {
			dfs(node, make([]uint64, 0))
		}
	}

	return cycles
}

func (dd *DeadlockDetector) getResourcesInCycle(cycle []uint64) []string {
	resources := make(map[string]bool)

	dd.manager.mutex.RLock()
	defer dd.manager.mutex.RUnlock()

	for _, reqID := range cycle {
		for _, rwlock := range dd.manager.locks {
			rwlock.mutex.Lock()
			for _, request := range rwlock.waitQueue {
				if request.ID == reqID {
					resources[request.Resource] = true
					break
				}
			}
			rwlock.mutex.Unlock()
		}
	}

	result := make([]string, 0, len(resources))
	for resource := range resources {
		result = append(result, resource)
	}

	return result
}

func (dd *DeadlockDetector) selectVictim(cycle []uint64, strategy VictimSelectionStrategy) uint64 {
	if len(cycle) == 0 {
		return 0
	}

	switch strategy {
	case YoungestRequest:
		return dd.selectYoungestRequest(cycle)
	case OldestRequest:
		return dd.selectOldestRequest(cycle)
	case LowestPriority:
		return dd.selectLowestPriorityRequest(cycle)
	case HighestPriority:
		return dd.selectHighestPriorityRequest(cycle)
	case RandomRequest:
		return dd.selectRandomRequest(cycle)
	default:
		return dd.selectYoungestRequest(cycle)
	}
}

func (dd *DeadlockDetector) selectYoungestRequest(cycle []uint64) uint64 {
	youngest := cycle[0]
	for _, reqID := range cycle[1:] {
		if reqID > youngest {
			youngest = reqID
		}
	}
	return youngest
}

func (dd *DeadlockDetector) selectOldestRequest(cycle []uint64) uint64 {
	oldest := cycle[0]
	for _, reqID := range cycle[1:] {
		if reqID < oldest {
			oldest = reqID
		}
	}
	return oldest
}

func (dd *DeadlockDetector) selectLowestPriorityRequest(cycle []uint64) uint64 {
	victim := cycle[0]
	lowestPriority := dd.getRequestPriority(victim)

	for _, reqID := range cycle[1:] {
		priority := dd.getRequestPriority(reqID)
		if priority < lowestPriority {
			victim = reqID
			lowestPriority = priority
		}
	}

	return victim
}

func (dd *DeadlockDetector) selectHighestPriorityRequest(cycle []uint64) uint64 {
	victim := cycle[0]
	highestPriority := dd.getRequestPriority(victim)

	for _, reqID := range cycle[1:] {
		priority := dd.getRequestPriority(reqID)
		if priority > highestPriority {
			victim = reqID
			highestPriority = priority
		}
	}

	return victim
}

func (dd *DeadlockDetector) selectRandomRequest(cycle []uint64) uint64 {
	index := int(time.Now().UnixNano()) % len(cycle)
	return cycle[index]
}

func (dd *DeadlockDetector) getRequestPriority(reqID uint64) LockPriority {
	dd.manager.mutex.RLock()
	defer dd.manager.mutex.RUnlock()

	for _, rwlock := range dd.manager.locks {
		rwlock.mutex.Lock()
		for _, request := range rwlock.waitQueue {
			if request.ID == reqID {
				rwlock.mutex.Unlock()
				return request.Priority
			}
		}
		rwlock.mutex.Unlock()
	}

	return NormalPriority
}

func (dd *DeadlockDetector) resolveDeadlock(deadlock DeadlockInfo) {
	// Find and abort the victim request
	dd.manager.mutex.RLock()
	defer dd.manager.mutex.RUnlock()

	for resource, rwlock := range dd.manager.locks {
		rwlock.mutex.Lock()
		for i, request := range rwlock.waitQueue {
			if request.ID == deadlock.VictimReqID {
				// Remove from wait queue
				rwlock.waitQueue = append(rwlock.waitQueue[:i], rwlock.waitQueue[i+1:]...)

				if request.IsWrite {
					atomic.AddInt32(&rwlock.waitingW, -1)
				}

				// Signal deadlock error
				select {
				case request.Done <- fmt.Errorf("deadlock detected, request aborted"):
				default:
				}

				// Update metrics
				if metrics, exists := dd.manager.lockMetrics[resource]; exists {
					atomic.AddInt64(&metrics.DeadlockCount, 1)
				}

				rwlock.mutex.Unlock()
				return
			}
		}
		rwlock.mutex.Unlock()
	}
}

func (dd *DeadlockDetector) checkLongWaitingRequests() {
	now := time.Now()
	longWaitThreshold := 30 * time.Second

	dd.manager.mutex.RLock()
	defer dd.manager.mutex.RUnlock()

	for resource, rwlock := range dd.manager.locks {
		rwlock.mutex.Lock()
		for i := len(rwlock.waitQueue) - 1; i >= 0; i-- {
			request := rwlock.waitQueue[i]
			if now.Sub(request.RequestAt) > longWaitThreshold {
				// Remove from wait queue
				rwlock.waitQueue = append(rwlock.waitQueue[:i], rwlock.waitQueue[i+1:]...)

				if request.IsWrite {
					atomic.AddInt32(&rwlock.waitingW, -1)
				}

				// Signal timeout error
				select {
				case request.Done <- fmt.Errorf("potential deadlock detected, request timed out"):
				default:
				}

				// Update metrics
				if metrics, exists := dd.manager.lockMetrics[resource]; exists {
					atomic.AddInt64(&metrics.DeadlockCount, 1)
				}
			}
		}
		rwlock.mutex.Unlock()
	}
}

func (dd *DeadlockDetector) requestsConflict(req1, req2 *LockRequest) bool {
	// Two requests conflict if they're on the same resource and at least one is a write
	return req1.Resource == req2.Resource && (req1.IsWrite || req2.IsWrite)
}

func (dd *DeadlockDetector) hashResource(resource string) uint64 {
	// Simple hash function for resource names
	hash := uint64(0)
	for _, c := range resource {
		hash = hash*31 + uint64(c)
	}
	return hash
}
