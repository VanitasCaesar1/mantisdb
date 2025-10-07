package concurrency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// EnhancedDeadlockDetector provides advanced deadlock detection with better algorithms
type EnhancedDeadlockDetector struct {
	lockManager       *EnhancedLockManager
	detectionInterval time.Duration
	adaptiveTimeout   *AdaptiveTimeout
	victimSelector    *VictimSelector

	// Detection state
	running  int32 // atomic
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Metrics
	metrics *DeadlockMetrics

	// Configuration
	maxCycleLength    int
	enableAdaptive    bool
	detectionStrategy DetectionStrategy
}

// DeadlockMetrics tracks deadlock detection performance
type DeadlockMetrics struct {
	detectionsRun     int64 // atomic
	deadlocksFound    int64 // atomic
	deadlocksResolved int64 // atomic
	avgDetectionTime  int64 // atomic, nanoseconds
	maxDetectionTime  int64 // atomic, nanoseconds
	falsePositives    int64 // atomic
	victimSelections  int64 // atomic
}

// DetectionStrategy defines different deadlock detection strategies
type DetectionStrategy int

const (
	StrategyDFS DetectionStrategy = iota
	StrategyBFS
	StrategyTarjan
	StrategyAdaptive
)

// AdaptiveTimeout manages dynamic timeout adjustment based on system load
type AdaptiveTimeout struct {
	baseTimeout    time.Duration
	currentTimeout time.Duration
	loadFactor     float64
	mutex          sync.RWMutex

	// Load tracking
	recentTimeouts   []time.Duration
	timeoutHistory   int
	adjustmentFactor float64
}

// NewAdaptiveTimeout creates a new adaptive timeout manager
func NewAdaptiveTimeout(baseTimeout time.Duration) *AdaptiveTimeout {
	return &AdaptiveTimeout{
		baseTimeout:      baseTimeout,
		currentTimeout:   baseTimeout,
		loadFactor:       1.0,
		recentTimeouts:   make([]time.Duration, 0, 100),
		timeoutHistory:   100,
		adjustmentFactor: 0.1,
	}
}

// GetTimeout returns the current adaptive timeout
func (at *AdaptiveTimeout) GetTimeout() time.Duration {
	at.mutex.RLock()
	defer at.mutex.RUnlock()
	return at.currentTimeout
}

// RecordTimeout records a timeout event and adjusts the timeout
func (at *AdaptiveTimeout) RecordTimeout(duration time.Duration, timedOut bool) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	// Add to recent timeouts
	at.recentTimeouts = append(at.recentTimeouts, duration)
	if len(at.recentTimeouts) > at.timeoutHistory {
		at.recentTimeouts = at.recentTimeouts[1:]
	}

	// Calculate average recent timeout
	var sum time.Duration
	timeoutCount := 0
	for _, t := range at.recentTimeouts {
		sum += t
		if t >= at.currentTimeout {
			timeoutCount++
		}
	}

	if len(at.recentTimeouts) == 0 {
		return
	}

	avgTimeout := sum / time.Duration(len(at.recentTimeouts))
	timeoutRate := float64(timeoutCount) / float64(len(at.recentTimeouts))

	// Adjust timeout based on timeout rate
	if timeoutRate > 0.1 { // More than 10% timeouts
		// Increase timeout
		adjustment := time.Duration(float64(at.currentTimeout) * at.adjustmentFactor)
		at.currentTimeout += adjustment
	} else if timeoutRate < 0.05 && avgTimeout < at.currentTimeout/2 {
		// Decrease timeout if very few timeouts and average is much lower
		adjustment := time.Duration(float64(at.currentTimeout) * at.adjustmentFactor)
		at.currentTimeout -= adjustment

		// Don't go below base timeout
		if at.currentTimeout < at.baseTimeout {
			at.currentTimeout = at.baseTimeout
		}
	}

	// Cap maximum timeout
	maxTimeout := at.baseTimeout * 5
	if at.currentTimeout > maxTimeout {
		at.currentTimeout = maxTimeout
	}
}

// VictimSelector implements advanced victim selection strategies
type VictimSelector struct {
	strategy       VictimSelectionStrategy
	lockManager    *EnhancedLockManager
	costCalculator *VictimCostCalculator
}

// VictimSelectionStrategy defines victim selection strategies
type VictimSelectionStrategy int

const (
	VictimYoungest VictimSelectionStrategy = iota
	VictimOldest
	VictimFewestLocks
	VictimMostLocks
	VictimLowestCost
	VictimAdaptive
)

// VictimCostCalculator calculates the cost of aborting a transaction
type VictimCostCalculator struct {
	// Weights for different cost factors
	lockCountWeight float64
	waitTimeWeight  float64
	operationWeight float64
	priorityWeight  float64
}

// NewVictimCostCalculator creates a new cost calculator
func NewVictimCostCalculator() *VictimCostCalculator {
	return &VictimCostCalculator{
		lockCountWeight: 0.3,
		waitTimeWeight:  0.4,
		operationWeight: 0.2,
		priorityWeight:  0.1,
	}
}

// CalculateCost calculates the cost of aborting a transaction
func (vcc *VictimCostCalculator) CalculateCost(txnID uint64, lockManager *EnhancedLockManager) float64 {
	// Get transaction information
	locks := lockManager.getCurrentLocks(txnID)
	lockCount := float64(len(locks))

	// Calculate normalized cost components
	lockCost := lockCount * vcc.lockCountWeight

	// For now, use simplified cost calculation
	// In a full implementation, this would consider:
	// - Transaction wait time
	// - Number of operations performed
	// - Transaction priority
	// - Resource contention level

	return lockCost
}

// NewVictimSelector creates a new victim selector
func NewVictimSelector(strategy VictimSelectionStrategy, lockManager *EnhancedLockManager) *VictimSelector {
	return &VictimSelector{
		strategy:       strategy,
		lockManager:    lockManager,
		costCalculator: NewVictimCostCalculator(),
	}
}

// SelectVictim selects the best victim from a deadlock cycle
func (vs *VictimSelector) SelectVictim(cycle []uint64) (uint64, error) {
	if len(cycle) == 0 {
		return 0, fmt.Errorf("empty deadlock cycle")
	}

	if len(cycle) == 1 {
		return cycle[0], nil
	}

	switch vs.strategy {
	case VictimYoungest:
		return vs.selectYoungest(cycle), nil
	case VictimOldest:
		return vs.selectOldest(cycle), nil
	case VictimFewestLocks:
		return vs.selectFewestLocks(cycle), nil
	case VictimMostLocks:
		return vs.selectMostLocks(cycle), nil
	case VictimLowestCost:
		return vs.selectLowestCost(cycle), nil
	case VictimAdaptive:
		return vs.selectAdaptive(cycle), nil
	default:
		return vs.selectYoungest(cycle), nil
	}
}

func (vs *VictimSelector) selectYoungest(cycle []uint64) uint64 {
	youngest := cycle[0]
	for _, txnID := range cycle[1:] {
		if txnID > youngest {
			youngest = txnID
		}
	}
	return youngest
}

func (vs *VictimSelector) selectOldest(cycle []uint64) uint64 {
	oldest := cycle[0]
	for _, txnID := range cycle[1:] {
		if txnID < oldest {
			oldest = txnID
		}
	}
	return oldest
}

func (vs *VictimSelector) selectFewestLocks(cycle []uint64) uint64 {
	victim := cycle[0]
	minLocks := len(vs.lockManager.getCurrentLocks(victim))

	for _, txnID := range cycle[1:] {
		lockCount := len(vs.lockManager.getCurrentLocks(txnID))
		if lockCount < minLocks {
			victim = txnID
			minLocks = lockCount
		}
	}

	return victim
}

func (vs *VictimSelector) selectMostLocks(cycle []uint64) uint64 {
	victim := cycle[0]
	maxLocks := len(vs.lockManager.getCurrentLocks(victim))

	for _, txnID := range cycle[1:] {
		lockCount := len(vs.lockManager.getCurrentLocks(txnID))
		if lockCount > maxLocks {
			victim = txnID
			maxLocks = lockCount
		}
	}

	return victim
}

func (vs *VictimSelector) selectLowestCost(cycle []uint64) uint64 {
	victim := cycle[0]
	minCost := vs.costCalculator.CalculateCost(victim, vs.lockManager)

	for _, txnID := range cycle[1:] {
		cost := vs.costCalculator.CalculateCost(txnID, vs.lockManager)
		if cost < minCost {
			victim = txnID
			minCost = cost
		}
	}

	return victim
}

func (vs *VictimSelector) selectAdaptive(cycle []uint64) uint64 {
	// Adaptive strategy combines multiple factors
	type candidateInfo struct {
		txnID     uint64
		lockCount int
		cost      float64
		score     float64
	}

	candidates := make([]candidateInfo, len(cycle))

	for i, txnID := range cycle {
		lockCount := len(vs.lockManager.getCurrentLocks(txnID))
		cost := vs.costCalculator.CalculateCost(txnID, vs.lockManager)

		// Calculate composite score (lower is better)
		score := float64(lockCount)*0.4 + cost*0.6

		candidates[i] = candidateInfo{
			txnID:     txnID,
			lockCount: lockCount,
			cost:      cost,
			score:     score,
		}
	}

	// Select candidate with lowest score
	victim := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.score < victim.score {
			victim = candidate
		}
	}

	return victim.txnID
}

// NewEnhancedDeadlockDetector creates a new enhanced deadlock detector
func NewEnhancedDeadlockDetector(lockManager *EnhancedLockManager, config *DeadlockDetectorConfig) *EnhancedDeadlockDetector {
	if config == nil {
		config = DefaultDeadlockDetectorConfig()
	}

	return &EnhancedDeadlockDetector{
		lockManager:       lockManager,
		detectionInterval: config.DetectionInterval,
		adaptiveTimeout:   NewAdaptiveTimeout(config.BaseTimeout),
		victimSelector:    NewVictimSelector(config.VictimStrategy, lockManager),
		maxCycleLength:    config.MaxCycleLength,
		enableAdaptive:    config.EnableAdaptive,
		detectionStrategy: config.DetectionStrategy,
		metrics:           &DeadlockMetrics{},
		stopChan:          make(chan struct{}),
	}
}

// DeadlockDetectorConfig holds configuration for the deadlock detector
type DeadlockDetectorConfig struct {
	DetectionInterval time.Duration
	BaseTimeout       time.Duration
	VictimStrategy    VictimSelectionStrategy
	MaxCycleLength    int
	EnableAdaptive    bool
	DetectionStrategy DetectionStrategy
}

// DefaultDeadlockDetectorConfig returns default configuration
func DefaultDeadlockDetectorConfig() *DeadlockDetectorConfig {
	return &DeadlockDetectorConfig{
		DetectionInterval: 5 * time.Second,
		BaseTimeout:       30 * time.Second,
		VictimStrategy:    VictimAdaptive,
		MaxCycleLength:    10,
		EnableAdaptive:    true,
		DetectionStrategy: StrategyAdaptive,
	}
}

// Start begins the deadlock detection process
func (edd *EnhancedDeadlockDetector) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&edd.running, 0, 1) {
		return fmt.Errorf("deadlock detector is already running")
	}

	edd.wg.Add(1)
	go edd.detectionLoop(ctx)

	return nil
}

// Stop stops the deadlock detection process
func (edd *EnhancedDeadlockDetector) Stop() error {
	if !atomic.CompareAndSwapInt32(&edd.running, 1, 0) {
		return nil // Not running
	}

	close(edd.stopChan)
	edd.wg.Wait()

	return nil
}

// detectionLoop runs the periodic deadlock detection
func (edd *EnhancedDeadlockDetector) detectionLoop(ctx context.Context) {
	defer edd.wg.Done()

	ticker := time.NewTicker(edd.detectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			edd.runDetection(ctx)
		case <-edd.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// runDetection performs a single deadlock detection cycle
func (edd *EnhancedDeadlockDetector) runDetection(ctx context.Context) {
	startTime := time.Now()
	atomic.AddInt64(&edd.metrics.detectionsRun, 1)

	// Build wait-for graph
	graph := edd.buildWaitForGraph()
	if graph == nil || len(graph.Edges) == 0 {
		return
	}

	// Detect cycles using the configured strategy
	var cycles [][]uint64
	switch edd.detectionStrategy {
	case StrategyDFS:
		cycles = edd.detectCyclesDFS(graph)
	case StrategyBFS:
		cycles = edd.detectCyclesBFS(graph)
	case StrategyTarjan:
		cycles = edd.detectCyclesTarjan(graph)
	case StrategyAdaptive:
		cycles = edd.detectCyclesAdaptive(graph)
	default:
		cycles = edd.detectCyclesDFS(graph)
	}

	// Process detected deadlocks
	for _, cycle := range cycles {
		if len(cycle) > 1 && len(cycle) <= edd.maxCycleLength {
			edd.resolveDeadlock(cycle)
		}
	}

	// Update metrics
	detectionTime := time.Since(startTime).Nanoseconds()
	atomic.AddInt64(&edd.metrics.avgDetectionTime, detectionTime)

	for {
		current := atomic.LoadInt64(&edd.metrics.maxDetectionTime)
		if detectionTime <= current || atomic.CompareAndSwapInt64(&edd.metrics.maxDetectionTime, current, detectionTime) {
			break
		}
	}

	if len(cycles) > 0 {
		atomic.AddInt64(&edd.metrics.deadlocksFound, int64(len(cycles)))
	}
}

// buildWaitForGraph builds the wait-for graph from current lock state
func (edd *EnhancedDeadlockDetector) buildWaitForGraph() *WaitForGraph {
	graph := &WaitForGraph{
		Edges: make(map[uint64][]uint64),
	}

	// Iterate through all resource locks
	edd.lockManager.locks.Range(func(key, value interface{}) bool {
		resourceLock := value.(*ResourceLock)
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
		return true
	})

	return graph
}

// detectCyclesDFS detects cycles using depth-first search
func (edd *EnhancedDeadlockDetector) detectCyclesDFS(graph *WaitForGraph) [][]uint64 {
	visited := make(map[uint64]int) // 0: unvisited, 1: visiting, 2: visited
	cycles := make([][]uint64, 0)

	var dfs func(uint64, []uint64)
	dfs = func(node uint64, path []uint64) {
		visited[node] = 1
		path = append(path, node)

		for _, neighbor := range graph.Edges[node] {
			switch visited[neighbor] {
			case 0: // Unvisited
				dfs(neighbor, path)
			case 1: // Currently visiting - cycle detected
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
			}
		}

		visited[node] = 2
	}

	for node := range graph.Edges {
		if visited[node] == 0 {
			dfs(node, make([]uint64, 0))
		}
	}

	return cycles
}

// detectCyclesBFS detects cycles using breadth-first search
func (edd *EnhancedDeadlockDetector) detectCyclesBFS(graph *WaitForGraph) [][]uint64 {
	cycles := make([][]uint64, 0)
	visited := make(map[uint64]bool)

	for startNode := range graph.Edges {
		if visited[startNode] {
			continue
		}

		// BFS to find cycles starting from this node
		queue := [][]uint64{{startNode}}
		nodeVisited := make(map[uint64]bool)

		for len(queue) > 0 {
			path := queue[0]
			queue = queue[1:]

			currentNode := path[len(path)-1]

			if nodeVisited[currentNode] {
				continue
			}
			nodeVisited[currentNode] = true

			for _, neighbor := range graph.Edges[currentNode] {
				// Check if neighbor is already in path (cycle detected)
				for i, node := range path {
					if node == neighbor {
						cycle := make([]uint64, len(path)-i)
						copy(cycle, path[i:])
						cycles = append(cycles, cycle)
						break
					}
				}

				// Add to queue if not visited and path not too long
				if !nodeVisited[neighbor] && len(path) < edd.maxCycleLength {
					newPath := make([]uint64, len(path)+1)
					copy(newPath, path)
					newPath[len(path)] = neighbor
					queue = append(queue, newPath)
				}
			}
		}

		visited[startNode] = true
	}

	return cycles
}

// detectCyclesTarjan detects cycles using Tarjan's strongly connected components algorithm
func (edd *EnhancedDeadlockDetector) detectCyclesTarjan(graph *WaitForGraph) [][]uint64 {
	index := 0
	stack := make([]uint64, 0)
	indices := make(map[uint64]int)
	lowlinks := make(map[uint64]int)
	onStack := make(map[uint64]bool)
	cycles := make([][]uint64, 0)

	var strongConnect func(uint64)
	strongConnect = func(v uint64) {
		indices[v] = index
		lowlinks[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range graph.Edges[v] {
			if _, exists := indices[w]; !exists {
				strongConnect(w)
				lowlinks[v] = min(lowlinks[v], lowlinks[w])
			} else if onStack[w] {
				lowlinks[v] = min(lowlinks[v], indices[w])
			}
		}

		if lowlinks[v] == indices[v] {
			// Found strongly connected component
			component := make([]uint64, 0)
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				component = append(component, w)
				if w == v {
					break
				}
			}

			// If component has more than one node, it's a cycle
			if len(component) > 1 {
				cycles = append(cycles, component)
			}
		}
	}

	for v := range graph.Edges {
		if _, exists := indices[v]; !exists {
			strongConnect(v)
		}
	}

	return cycles
}

// detectCyclesAdaptive uses adaptive strategy based on graph characteristics
func (edd *EnhancedDeadlockDetector) detectCyclesAdaptive(graph *WaitForGraph) [][]uint64 {
	nodeCount := len(graph.Edges)
	edgeCount := 0

	for _, edges := range graph.Edges {
		edgeCount += len(edges)
	}

	// Choose strategy based on graph characteristics
	if nodeCount < 10 {
		// Small graph - use DFS
		return edd.detectCyclesDFS(graph)
	} else if float64(edgeCount)/float64(nodeCount) > 2.0 {
		// Dense graph - use Tarjan's algorithm
		return edd.detectCyclesTarjan(graph)
	} else {
		// Sparse graph - use BFS
		return edd.detectCyclesBFS(graph)
	}
}

// resolveDeadlock resolves a detected deadlock by selecting and aborting a victim
func (edd *EnhancedDeadlockDetector) resolveDeadlock(cycle []uint64) {
	victim, err := edd.victimSelector.SelectVictim(cycle)
	if err != nil {
		return
	}

	// In a real implementation, this would abort the victim transaction
	// For now, we'll just release its locks
	edd.lockManager.ReleaseAllLocks(victim)

	atomic.AddInt64(&edd.metrics.deadlocksResolved, 1)
	atomic.AddInt64(&edd.metrics.victimSelections, 1)
}

// GetMetrics returns current deadlock detection metrics
func (edd *EnhancedDeadlockDetector) GetMetrics() *DeadlockMetrics {
	return &DeadlockMetrics{
		detectionsRun:     atomic.LoadInt64(&edd.metrics.detectionsRun),
		deadlocksFound:    atomic.LoadInt64(&edd.metrics.deadlocksFound),
		deadlocksResolved: atomic.LoadInt64(&edd.metrics.deadlocksResolved),
		avgDetectionTime:  atomic.LoadInt64(&edd.metrics.avgDetectionTime),
		maxDetectionTime:  atomic.LoadInt64(&edd.metrics.maxDetectionTime),
		falsePositives:    atomic.LoadInt64(&edd.metrics.falsePositives),
		victimSelections:  atomic.LoadInt64(&edd.metrics.victimSelections),
	}
}

// WaitForGraph represents the wait-for graph for deadlock detection
type WaitForGraph struct {
	Edges map[uint64][]uint64
	mutex sync.RWMutex
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
