package transaction

import (
	"fmt"
	"sync"
	"time"
)

// DeadlockDetector handles deadlock detection and resolution
type DeadlockDetector struct {
	lockManager       LockManager
	txnManager        TransactionManager
	detectionInterval time.Duration
	stopChan          chan struct{}
	wg                sync.WaitGroup
	mutex             sync.RWMutex
	running           bool
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector(lockManager LockManager, txnManager TransactionManager, detectionInterval time.Duration) *DeadlockDetector {
	if detectionInterval <= 0 {
		detectionInterval = 5 * time.Second // Default detection interval
	}

	return &DeadlockDetector{
		lockManager:       lockManager,
		txnManager:        txnManager,
		detectionInterval: detectionInterval,
		stopChan:          make(chan struct{}),
	}
}

// Start begins the deadlock detection process
func (dd *DeadlockDetector) Start() error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	if dd.running {
		return fmt.Errorf("deadlock detector is already running")
	}

	dd.running = true
	dd.wg.Add(1)

	go dd.detectionLoop()

	return nil
}

// Stop stops the deadlock detection process
func (dd *DeadlockDetector) Stop() error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	if !dd.running {
		return nil
	}

	close(dd.stopChan)
	dd.wg.Wait()
	dd.running = false

	return nil
}

// detectionLoop runs the periodic deadlock detection
func (dd *DeadlockDetector) detectionLoop() {
	defer dd.wg.Done()

	ticker := time.NewTicker(dd.detectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dd.detectAndResolveDeadlocks()
		case <-dd.stopChan:
			return
		}
	}
}

// detectAndResolveDeadlocks performs deadlock detection and resolution
func (dd *DeadlockDetector) detectAndResolveDeadlocks() {
	deadlocks := dd.lockManager.DetectDeadlocks()

	for _, deadlock := range deadlocks {
		if err := dd.resolveDeadlock(deadlock); err != nil {
			// In a real system, this would be logged properly
			fmt.Printf("Failed to resolve deadlock: %v\n", err)
		}
	}
}

// resolveDeadlock resolves a specific deadlock
func (dd *DeadlockDetector) resolveDeadlock(deadlock DeadlockInfo) error {
	// Get the victim transaction
	victimTxn, err := dd.txnManager.GetTransaction(deadlock.VictimTxnID)
	if err != nil {
		return fmt.Errorf("failed to get victim transaction %d: %w", deadlock.VictimTxnID, err)
	}

	// Abort the victim transaction
	if err := dd.txnManager.Abort(victimTxn); err != nil {
		return fmt.Errorf("failed to abort victim transaction %d: %w", deadlock.VictimTxnID, err)
	}

	// Log the deadlock resolution
	fmt.Printf("Resolved deadlock by aborting transaction %d. Cycle: %v\n",
		deadlock.VictimTxnID, deadlock.Cycle)

	return nil
}

// WaitForGraphAnalyzer provides advanced wait-for graph analysis
type WaitForGraphAnalyzer struct {
	graph *WaitForGraph
}

// NewWaitForGraphAnalyzer creates a new graph analyzer
func NewWaitForGraphAnalyzer(graph *WaitForGraph) *WaitForGraphAnalyzer {
	return &WaitForGraphAnalyzer{graph: graph}
}

// FindAllCycles finds all cycles in the wait-for graph using DFS
func (wga *WaitForGraphAnalyzer) FindAllCycles() [][]uint64 {
	wga.graph.mutex.RLock()
	defer wga.graph.mutex.RUnlock()

	visited := make(map[uint64]int) // 0: unvisited, 1: visiting, 2: visited
	cycles := make([][]uint64, 0)

	var dfs func(uint64, []uint64)
	dfs = func(node uint64, path []uint64) {
		visited[node] = 1 // Mark as visiting
		path = append(path, node)

		for _, neighbor := range wga.graph.Edges[node] {
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
	for node := range wga.graph.Edges {
		if visited[node] == 0 {
			dfs(node, make([]uint64, 0))
		}
	}

	return cycles
}

// FindShortestCycle finds the shortest cycle in the graph
func (wga *WaitForGraphAnalyzer) FindShortestCycle() []uint64 {
	cycles := wga.FindAllCycles()
	if len(cycles) == 0 {
		return nil
	}

	shortest := cycles[0]
	for _, cycle := range cycles[1:] {
		if len(cycle) < len(shortest) {
			shortest = cycle
		}
	}

	return shortest
}

// GetTransactionDependencies returns all transactions that a given transaction is waiting for
func (wga *WaitForGraphAnalyzer) GetTransactionDependencies(txnID uint64) []uint64 {
	wga.graph.mutex.RLock()
	defer wga.graph.mutex.RUnlock()

	dependencies, exists := wga.graph.Edges[txnID]
	if !exists {
		return nil
	}

	// Return a copy to avoid external modification
	result := make([]uint64, len(dependencies))
	copy(result, dependencies)
	return result
}

// GetTransactionDependents returns all transactions waiting for a given transaction
func (wga *WaitForGraphAnalyzer) GetTransactionDependents(txnID uint64) []uint64 {
	wga.graph.mutex.RLock()
	defer wga.graph.mutex.RUnlock()

	dependents := make([]uint64, 0)
	for waitingTxn, dependencies := range wga.graph.Edges {
		for _, depTxn := range dependencies {
			if depTxn == txnID {
				dependents = append(dependents, waitingTxn)
				break
			}
		}
	}

	return dependents
}

// IsReachable checks if there's a path from source to target transaction
func (wga *WaitForGraphAnalyzer) IsReachable(source, target uint64) bool {
	wga.graph.mutex.RLock()
	defer wga.graph.mutex.RUnlock()

	visited := make(map[uint64]bool)
	queue := []uint64{source}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == target {
			return true
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, neighbor := range wga.graph.Edges[current] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// VictimSelectionStrategy defines different strategies for selecting deadlock victims
type VictimSelectionStrategy int

const (
	YoungestTransaction VictimSelectionStrategy = iota
	OldestTransaction
	FewestLocks
	MostLocks
	RandomSelection
)

// VictimSelector handles victim selection for deadlock resolution
type VictimSelector struct {
	strategy    VictimSelectionStrategy
	txnManager  TransactionManager
	lockManager LockManager
}

// NewVictimSelector creates a new victim selector
func NewVictimSelector(strategy VictimSelectionStrategy, txnManager TransactionManager, lockManager LockManager) *VictimSelector {
	return &VictimSelector{
		strategy:    strategy,
		txnManager:  txnManager,
		lockManager: lockManager,
	}
}

// SelectVictim selects a victim transaction from a deadlock cycle
func (vs *VictimSelector) SelectVictim(cycle []uint64) (uint64, error) {
	if len(cycle) == 0 {
		return 0, fmt.Errorf("empty cycle")
	}

	switch vs.strategy {
	case YoungestTransaction:
		return vs.selectYoungest(cycle)
	case OldestTransaction:
		return vs.selectOldest(cycle)
	case FewestLocks:
		return vs.selectFewestLocks(cycle)
	case MostLocks:
		return vs.selectMostLocks(cycle)
	case RandomSelection:
		return vs.selectRandom(cycle)
	default:
		return vs.selectYoungest(cycle) // Default to youngest
	}
}

func (vs *VictimSelector) selectYoungest(cycle []uint64) (uint64, error) {
	youngest := cycle[0]
	for _, txnID := range cycle[1:] {
		if txnID > youngest {
			youngest = txnID
		}
	}
	return youngest, nil
}

func (vs *VictimSelector) selectOldest(cycle []uint64) (uint64, error) {
	oldest := cycle[0]
	for _, txnID := range cycle[1:] {
		if txnID < oldest {
			oldest = txnID
		}
	}
	return oldest, nil
}

func (vs *VictimSelector) selectFewestLocks(cycle []uint64) (uint64, error) {
	victim := cycle[0]
	minLocks := vs.countTransactionLocks(victim)

	for _, txnID := range cycle[1:] {
		lockCount := vs.countTransactionLocks(txnID)
		if lockCount < minLocks {
			victim = txnID
			minLocks = lockCount
		}
	}

	return victim, nil
}

func (vs *VictimSelector) selectMostLocks(cycle []uint64) (uint64, error) {
	victim := cycle[0]
	maxLocks := vs.countTransactionLocks(victim)

	for _, txnID := range cycle[1:] {
		lockCount := vs.countTransactionLocks(txnID)
		if lockCount > maxLocks {
			victim = txnID
			maxLocks = lockCount
		}
	}

	return victim, nil
}

func (vs *VictimSelector) selectRandom(cycle []uint64) (uint64, error) {
	// Simple random selection based on current time
	index := int(time.Now().UnixNano()) % len(cycle)
	return cycle[index], nil
}

func (vs *VictimSelector) countTransactionLocks(txnID uint64) int {
	txn, err := vs.txnManager.GetTransaction(txnID)
	if err != nil {
		return 0
	}

	return len(txn.Locks)
}
