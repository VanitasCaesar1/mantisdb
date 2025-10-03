package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mantisDB/durability"
	"mantisDB/errors"
	"mantisDB/integrity"
	"mantisDB/transaction"
	"mantisDB/wal"
)

// ChaosConfig defines configuration for chaos engineering tests
type ChaosConfig struct {
	Duration              time.Duration
	FailureRate           float64 // 0.0 to 1.0
	MaxConcurrentOps      int
	EnableDiskFailures    bool
	EnableMemoryFailures  bool
	EnableNetworkFailures bool
	EnableCorruption      bool
	RandomSeed            int64
}

// DefaultChaosConfig returns a default chaos configuration
func DefaultChaosConfig() *ChaosConfig {
	return &ChaosConfig{
		Duration:              30 * time.Second,
		FailureRate:           0.1, // 10% failure rate
		MaxConcurrentOps:      50,
		EnableDiskFailures:    true,
		EnableMemoryFailures:  true,
		EnableNetworkFailures: false, // Network failures not applicable for local tests
		EnableCorruption:      true,
		RandomSeed:            time.Now().UnixNano(),
	}
}

// ChaosMonkey implements chaos engineering for MantisDB
type ChaosMonkey struct {
	config   *ChaosConfig
	random   *rand.Rand
	failures map[string]int
	mutex    sync.RWMutex
	stopChan chan bool
	running  bool
}

// NewChaosMonkey creates a new chaos monkey instance
func NewChaosMonkey(config *ChaosConfig) *ChaosMonkey {
	if config == nil {
		config = DefaultChaosConfig()
	}

	return &ChaosMonkey{
		config:   config,
		random:   rand.New(rand.NewSource(config.RandomSeed)),
		failures: make(map[string]int),
		stopChan: make(chan bool),
	}
}

// Start begins chaos engineering
func (cm *ChaosMonkey) Start() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.running {
		return
	}

	cm.running = true
	go cm.chaosLoop()
}

// Stop ends chaos engineering
func (cm *ChaosMonkey) Stop() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.running {
		return
	}

	cm.running = false
	cm.stopChan <- true
}

// ShouldFail returns true if an operation should fail based on failure rate
func (cm *ChaosMonkey) ShouldFail(operation string) bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if !cm.running {
		return false
	}

	shouldFail := cm.random.Float64() < cm.config.FailureRate
	if shouldFail {
		cm.failures[operation]++
	}

	return shouldFail
}

// GetFailureStats returns failure statistics
func (cm *ChaosMonkey) GetFailureStats() map[string]int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	stats := make(map[string]int)
	for op, count := range cm.failures {
		stats[op] = count
	}
	return stats
}

// chaosLoop runs the main chaos engineering loop
func (cm *ChaosMonkey) chaosLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(cm.config.Duration)

	for {
		select {
		case <-cm.stopChan:
			return
		case <-timeout:
			cm.Stop()
			return
		case <-ticker.C:
			// Perform random chaos operations
			cm.performChaosOperation()
		}
	}
}

// performChaosOperation performs a random chaos operation
func (cm *ChaosMonkey) performChaosOperation() {
	operations := []string{}

	if cm.config.EnableDiskFailures {
		operations = append(operations, "disk_failure")
	}
	if cm.config.EnableMemoryFailures {
		operations = append(operations, "memory_failure")
	}
	if cm.config.EnableCorruption {
		operations = append(operations, "corruption")
	}

	if len(operations) == 0 {
		return
	}

	operation := operations[cm.random.Intn(len(operations))]
	cm.ShouldFail(operation)
}

// TestChaosEngineering runs comprehensive chaos engineering tests
func TestChaosEngineering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos engineering test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "chaos_engineering_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create chaos monkey
	chaosConfig := DefaultChaosConfig()
	chaosConfig.Duration = 10 * time.Second // Shorter duration for tests
	chaosConfig.FailureRate = 0.05          // 5% failure rate
	chaosMonkey := NewChaosMonkey(chaosConfig)

	t.Run("ChaosWithWALOperations", func(t *testing.T) {
		walDir := filepath.Join(tempDir, "chaos_wal")

		// Create WAL manager
		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = walDir
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		// Start chaos monkey
		chaosMonkey.Start()
		defer chaosMonkey.Stop()

		// Perform operations under chaos
		const numOperations = 100
		successCount := 0
		failureCount := 0

		for i := 0; i < numOperations; i++ {
			// Check if chaos monkey wants this operation to fail
			if chaosMonkey.ShouldFail("wal_write") {
				failureCount++
				continue
			}

			entry := &wal.WALEntry{
				TxnID: uint64(i + 1),
				Operation: wal.Operation{
					Type:  wal.OpInsert,
					Key:   fmt.Sprintf("chaos_key_%d", i),
					Value: []byte(fmt.Sprintf("chaos_value_%d", i)),
				},
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(entry); err != nil {
				failureCount++
				t.Logf("WAL write failed (expected under chaos): %v", err)
			} else {
				successCount++
			}

			// Small delay to allow chaos monkey to work
			time.Sleep(time.Millisecond)
		}

		t.Logf("WAL operations under chaos: %d successful, %d failed", successCount, failureCount)

		// Verify system is still functional
		testEntry := &wal.WALEntry{
			TxnID: 999999,
			Operation: wal.Operation{
				Type:  wal.OpInsert,
				Key:   "final_test",
				Value: []byte("final_value"),
			},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(testEntry); err != nil {
			t.Errorf("System should be functional after chaos: %v", err)
		}
	})

	t.Run("ChaosWithTransactionSystem", func(t *testing.T) {
		// Create transaction system
		lockManager := transaction.NewLockManager(5 * time.Second)
		defer lockManager.Close()

		txnSystemConfig := transaction.DefaultTransactionSystemConfig()
		txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
		if err := txnSystem.Start(); err != nil {
			t.Fatalf("Failed to start transaction system: %v", err)
		}
		defer txnSystem.Stop()

		// Start chaos monkey
		chaosMonkey.Start()
		defer chaosMonkey.Stop()

		const numTransactions = 50
		successCount := 0
		failureCount := 0

		for i := 0; i < numTransactions; i++ {
			// Check if chaos monkey wants this transaction to fail
			if chaosMonkey.ShouldFail("transaction_begin") {
				failureCount++
				continue
			}

			txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
			if err != nil {
				failureCount++
				t.Logf("Transaction begin failed (expected under chaos): %v", err)
				continue
			}

			// Perform some operations
			key := fmt.Sprintf("chaos_txn_key_%d", i)
			value := []byte(fmt.Sprintf("chaos_txn_value_%d", i))

			if !chaosMonkey.ShouldFail("transaction_operation") {
				if err := txnSystem.Insert(txn, key, value); err != nil {
					t.Logf("Transaction operation failed: %v", err)
				}
			}

			// Commit or abort based on chaos
			if chaosMonkey.ShouldFail("transaction_commit") {
				if err := txnSystem.AbortTransaction(txn); err != nil {
					t.Logf("Transaction abort failed: %v", err)
				}
				failureCount++
			} else {
				if err := txnSystem.CommitTransaction(txn); err != nil {
					t.Logf("Transaction commit failed: %v", err)
					failureCount++
				} else {
					successCount++
				}
			}

			time.Sleep(time.Millisecond)
		}

		t.Logf("Transactions under chaos: %d successful, %d failed", successCount, failureCount)

		// Verify system is still functional
		testTxn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Errorf("System should be functional after chaos: %v", err)
		} else {
			txnSystem.CommitTransaction(testTxn)
		}
	})

	t.Run("ChaosWithDurabilitySystem", func(t *testing.T) {
		dataDir := filepath.Join(tempDir, "chaos_data")

		// Create durability manager
		durabilityConfig := durability.DefaultDurabilityConfig()
		durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
		if err != nil {
			t.Fatalf("Failed to create durability manager: %v", err)
		}
		defer durabilityManager.Close(context.Background())

		// Start chaos monkey
		chaosMonkey.Start()
		defer chaosMonkey.Stop()

		ctx := context.Background()
		const numWrites = 100
		successCount := 0
		failureCount := 0

		for i := 0; i < numWrites; i++ {
			// Check if chaos monkey wants this write to fail
			if chaosMonkey.ShouldFail("durability_write") {
				failureCount++
				continue
			}

			filename := fmt.Sprintf("chaos_file_%d.dat", i)
			filePath := filepath.Join(dataDir, filename)
			data := []byte(fmt.Sprintf("chaos data %d", i))

			if err := durabilityManager.Write(ctx, filePath, data, 0); err != nil {
				failureCount++
				t.Logf("Durability write failed (expected under chaos): %v", err)
			} else {
				successCount++
			}

			time.Sleep(time.Millisecond)
		}

		t.Logf("Durability writes under chaos: %d successful, %d failed", successCount, failureCount)

		// Force sync to test system stability
		if err := durabilityManager.Sync(ctx); err != nil {
			t.Logf("Sync failed under chaos (may be expected): %v", err)
		}
	})

	// Print chaos statistics
	stats := chaosMonkey.GetFailureStats()
	t.Logf("Chaos engineering statistics:")
	for operation, count := range stats {
		t.Logf("  %s: %d failures", operation, count)
	}
}

// TestRandomFailureInjection tests random failure injection
func TestRandomFailureInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping random failure injection test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "random_failure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create error handler
	errorHandler := errors.NewDefaultErrorHandler(nil)

	// Test random I/O failures
	t.Run("RandomIOFailures", func(t *testing.T) {
		const numOperations = 100
		failureCount := 0

		for i := 0; i < numOperations; i++ {
			// Simulate random I/O error
			if rand.Float64() < 0.1 { // 10% failure rate
				ioError := fmt.Errorf("simulated I/O error %d", i)

				if err := errorHandler.HandleIOError(ioError, i%5); err != nil {
					failureCount++
				}
			}
		}

		t.Logf("Random I/O failures: %d out of %d operations", failureCount, numOperations)
	})

	// Test random memory exhaustion
	t.Run("RandomMemoryExhaustion", func(t *testing.T) {
		const numOperations = 50
		exhaustionCount := 0

		for i := 0; i < numOperations; i++ {
			// Simulate random memory exhaustion
			if rand.Float64() < 0.05 { // 5% exhaustion rate
				operation := fmt.Sprintf("allocation_%d", i)

				if err := errorHandler.HandleMemoryExhaustion(operation); err != nil {
					exhaustionCount++
				}
			}
		}

		t.Logf("Random memory exhaustion events: %d out of %d operations", exhaustionCount, numOperations)
	})

	// Test random disk space exhaustion
	t.Run("RandomDiskExhaustion", func(t *testing.T) {
		const numOperations = 50
		exhaustionCount := 0

		for i := 0; i < numOperations; i++ {
			// Simulate random disk exhaustion
			if rand.Float64() < 0.03 { // 3% exhaustion rate
				operation := fmt.Sprintf("write_%d", i)

				if err := errorHandler.HandleDiskFull(operation); err != nil {
					exhaustionCount++
				}
			}
		}

		t.Logf("Random disk exhaustion events: %d out of %d operations", exhaustionCount, numOperations)
	})

	// Test random corruption
	t.Run("RandomCorruption", func(t *testing.T) {
		checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)
		const numFiles = 20
		corruptionCount := 0

		for i := 0; i < numFiles; i++ {
			filename := fmt.Sprintf("random_file_%d.dat", i)
			filePath := filepath.Join(tempDir, filename)
			originalData := []byte(fmt.Sprintf("original data %d", i))

			// Write original data
			if err := os.WriteFile(filePath, originalData, 0644); err != nil {
				t.Fatalf("Failed to write file %d: %v", i, err)
			}

			originalChecksum := checksumEngine.Calculate(originalData)

			// Randomly corrupt some files
			if rand.Float64() < 0.2 { // 20% corruption rate
				corruptedData := []byte(fmt.Sprintf("corrupted data %d", i))
				if err := os.WriteFile(filePath, corruptedData, 0644); err != nil {
					t.Fatalf("Failed to corrupt file %d: %v", i, err)
				}

				// Try to verify (should fail)
				readData, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read corrupted file %d: %v", i, err)
				}

				if err := checksumEngine.Verify(readData, originalChecksum); err != nil {
					corruptionCount++

					// Handle corruption
					corruptionInfo := errors.CorruptionInfo{
						Location: errors.DataLocation{
							File:   filePath,
							Offset: 0,
							Size:   int64(len(readData)),
						},
						Type:        "random_corruption",
						Description: "Randomly injected corruption",
						Timestamp:   time.Now(),
						Checksum:    originalChecksum,
					}

					if err := errorHandler.HandleCorruption(corruptionInfo); err != nil {
						t.Errorf("Failed to handle corruption for file %d: %v", i, err)
					}
				}
			}
		}

		t.Logf("Random corruption events: %d out of %d files", corruptionCount, numFiles)
	})
}

// TestHighLoadStress tests system behavior under high load
func TestHighLoadStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load stress test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "high_load_stress_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("HighConcurrencyWAL", func(t *testing.T) {
		walDir := filepath.Join(tempDir, "stress_wal")

		// Create WAL manager
		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = walDir
		walConfig.MaxFileSize = 1024 // Small files to force rotation
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		const numGoroutines = 20
		const entriesPerGoroutine = 100

		var wg sync.WaitGroup
		var successCount, failureCount int64
		var mutex sync.Mutex

		startTime := time.Now()

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				localSuccess := 0
				localFailure := 0

				for i := 0; i < entriesPerGoroutine; i++ {
					entry := &wal.WALEntry{
						TxnID: uint64(goroutineID*1000 + i),
						Operation: wal.Operation{
							Type:  wal.OpInsert,
							Key:   fmt.Sprintf("stress_key_%d_%d", goroutineID, i),
							Value: []byte(fmt.Sprintf("stress_value_%d_%d", goroutineID, i)),
						},
						Timestamp: time.Now(),
					}

					if err := walManager.WriteEntry(entry); err != nil {
						localFailure++
					} else {
						localSuccess++
					}
				}

				mutex.Lock()
				successCount += int64(localSuccess)
				failureCount += int64(localFailure)
				mutex.Unlock()
			}(g)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalOps := successCount + failureCount
		opsPerSecond := float64(totalOps) / duration.Seconds()

		t.Logf("High concurrency WAL stress test:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Total operations: %d", totalOps)
		t.Logf("  Successful: %d", successCount)
		t.Logf("  Failed: %d", failureCount)
		t.Logf("  Operations/second: %.2f", opsPerSecond)

		// Verify final state
		finalLSN := walManager.GetCurrentLSN()
		if finalLSN != uint64(successCount) {
			t.Errorf("Expected final LSN %d, got %d", successCount, finalLSN)
		}
	})

	t.Run("HighConcurrencyTransactions", func(t *testing.T) {
		// Create transaction system
		lockManager := transaction.NewLockManager(1 * time.Second) // Shorter timeout for stress
		defer lockManager.Close()

		txnSystemConfig := transaction.DefaultTransactionSystemConfig()
		txnSystemConfig.LockTimeout = 1 * time.Second
		txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
		if err := txnSystem.Start(); err != nil {
			t.Fatalf("Failed to start transaction system: %v", err)
		}
		defer txnSystem.Stop()

		const numGoroutines = 15
		const transactionsPerGoroutine = 20

		var wg sync.WaitGroup
		var commitCount, abortCount int64
		var mutex sync.Mutex

		startTime := time.Now()

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				localCommits := 0
				localAborts := 0

				for i := 0; i < transactionsPerGoroutine; i++ {
					txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
					if err != nil {
						localAborts++
						continue
					}

					// Perform operations
					key := fmt.Sprintf("stress_txn_%d_%d", goroutineID, i)
					value := []byte(fmt.Sprintf("stress_value_%d_%d", goroutineID, i))

					if err := txnSystem.Insert(txn, key, value); err != nil {
						txnSystem.AbortTransaction(txn)
						localAborts++
						continue
					}

					// Random commit or abort
					if rand.Float64() < 0.8 { // 80% commit rate
						if err := txnSystem.CommitTransaction(txn); err != nil {
							localAborts++
						} else {
							localCommits++
						}
					} else {
						txnSystem.AbortTransaction(txn)
						localAborts++
					}
				}

				mutex.Lock()
				commitCount += int64(localCommits)
				abortCount += int64(localAborts)
				mutex.Unlock()
			}(g)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalTxns := commitCount + abortCount
		txnsPerSecond := float64(totalTxns) / duration.Seconds()

		t.Logf("High concurrency transaction stress test:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Total transactions: %d", totalTxns)
		t.Logf("  Committed: %d", commitCount)
		t.Logf("  Aborted: %d", abortCount)
		t.Logf("  Transactions/second: %.2f", txnsPerSecond)

		// Verify no active transactions
		if txnSystem.GetTransactionCount() != 0 {
			t.Errorf("Expected 0 active transactions, got %d", txnSystem.GetTransactionCount())
		}
	})

	t.Run("HighVolumeIntegrityChecks", func(t *testing.T) {
		checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

		const numGoroutines = 10
		const checksPerGoroutine = 200

		var wg sync.WaitGroup
		var successCount, failureCount int64
		var mutex sync.Mutex

		// Pre-create test data
		testData := make([][]byte, checksPerGoroutine)
		expectedChecksums := make([]uint32, checksPerGoroutine)

		for i := 0; i < checksPerGoroutine; i++ {
			testData[i] = []byte(fmt.Sprintf("integrity test data %d with some additional content to make it longer", i))
			expectedChecksums[i] = checksumEngine.Calculate(testData[i])
		}

		startTime := time.Now()

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				localSuccess := 0
				localFailure := 0

				for i := 0; i < checksPerGoroutine; i++ {
					// Randomly corrupt some data
					data := testData[i]
					if rand.Float64() < 0.05 { // 5% corruption rate
						data = []byte(fmt.Sprintf("corrupted data %d_%d", goroutineID, i))
					}

					if err := checksumEngine.Verify(data, expectedChecksums[i]); err != nil {
						localFailure++
					} else {
						localSuccess++
					}
				}

				mutex.Lock()
				successCount += int64(localSuccess)
				failureCount += int64(localFailure)
				mutex.Unlock()
			}(g)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalChecks := successCount + failureCount
		checksPerSecond := float64(totalChecks) / duration.Seconds()

		t.Logf("High volume integrity check stress test:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Total checks: %d", totalChecks)
		t.Logf("  Successful: %d", successCount)
		t.Logf("  Failed: %d", failureCount)
		t.Logf("  Checks/second: %.2f", checksPerSecond)
	})
}

// TestResourceExhaustionScenarios tests various resource exhaustion scenarios
func TestResourceExhaustionScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "resource_exhaustion_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	errorHandler := errors.NewDefaultErrorHandler(nil)

	t.Run("SimulatedDiskSpaceExhaustion", func(t *testing.T) {
		// Create durability manager
		durabilityConfig := durability.DefaultDurabilityConfig()
		durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
		if err != nil {
			t.Fatalf("Failed to create durability manager: %v", err)
		}
		defer durabilityManager.Close(context.Background())

		ctx := context.Background()
		const numWrites = 100
		diskFullCount := 0

		for i := 0; i < numWrites; i++ {
			// Simulate disk full condition randomly
			if rand.Float64() < 0.1 { // 10% disk full rate
				if err := errorHandler.HandleDiskFull(fmt.Sprintf("write_%d", i)); err != nil {
					diskFullCount++
				}
				continue
			}

			// Normal write operation
			filename := fmt.Sprintf("exhaust_test_%d.dat", i)
			filePath := filepath.Join(tempDir, filename)
			data := []byte(fmt.Sprintf("test data %d", i))

			if err := durabilityManager.Write(ctx, filePath, data, 0); err != nil {
				t.Logf("Write failed (may be due to simulated disk full): %v", err)
			}
		}

		t.Logf("Simulated disk space exhaustion: %d events out of %d operations", diskFullCount, numWrites)
	})

	t.Run("SimulatedMemoryPressure", func(t *testing.T) {
		const numAllocations = 200
		memoryPressureCount := 0

		for i := 0; i < numAllocations; i++ {
			// Simulate memory pressure randomly
			if rand.Float64() < 0.08 { // 8% memory pressure rate
				if err := errorHandler.HandleMemoryExhaustion(fmt.Sprintf("allocation_%d", i)); err != nil {
					memoryPressureCount++
				}
			}
		}

		t.Logf("Simulated memory pressure: %d events out of %d allocations", memoryPressureCount, numAllocations)
	})

	t.Run("CombinedResourceExhaustion", func(t *testing.T) {
		// Test combined resource exhaustion scenarios
		const numOperations = 150
		var exhaustionEvents sync.Map
		var wg sync.WaitGroup

		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()

				// Randomly trigger different exhaustion scenarios
				switch rand.Intn(3) {
				case 0: // Disk exhaustion
					if rand.Float64() < 0.05 {
						if err := errorHandler.HandleDiskFull(fmt.Sprintf("op_%d", opID)); err != nil {
							exhaustionEvents.Store(fmt.Sprintf("disk_%d", opID), true)
						}
					}
				case 1: // Memory exhaustion
					if rand.Float64() < 0.05 {
						if err := errorHandler.HandleMemoryExhaustion(fmt.Sprintf("op_%d", opID)); err != nil {
							exhaustionEvents.Store(fmt.Sprintf("memory_%d", opID), true)
						}
					}
				case 2: // I/O error
					if rand.Float64() < 0.05 {
						ioErr := fmt.Errorf("simulated I/O error for op %d", opID)
						if err := errorHandler.HandleIOError(ioErr, opID%3); err != nil {
							exhaustionEvents.Store(fmt.Sprintf("io_%d", opID), true)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		// Count exhaustion events
		eventCount := 0
		exhaustionEvents.Range(func(key, value interface{}) bool {
			eventCount++
			return true
		})

		t.Logf("Combined resource exhaustion: %d events out of %d operations", eventCount, numOperations)
	})
}

// TestLongRunningStability tests system stability over extended periods
func TestLongRunningStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "long_running_stability_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test duration (reduced for CI/testing)
	testDuration := 30 * time.Second

	t.Run("ContinuousOperations", func(t *testing.T) {
		walDir := filepath.Join(tempDir, "stability_wal")

		// Create WAL manager
		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = walDir
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		// Create transaction system
		lockManager := transaction.NewLockManager(5 * time.Second)
		defer lockManager.Close()

		txnSystemConfig := transaction.DefaultTransactionSystemConfig()
		txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
		if err := txnSystem.Start(); err != nil {
			t.Fatalf("Failed to start transaction system: %v", err)
		}
		defer txnSystem.Stop()

		// Run continuous operations
		stopChan := make(chan bool)
		var operationCount int64
		var errorCount int64

		// Start operation goroutine
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			opID := 0
			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					opID++

					// WAL operation
					entry := &wal.WALEntry{
						TxnID: uint64(opID),
						Operation: wal.Operation{
							Type:  wal.OpInsert,
							Key:   fmt.Sprintf("stability_key_%d", opID),
							Value: []byte(fmt.Sprintf("stability_value_%d", opID)),
						},
						Timestamp: time.Now(),
					}

					if err := walManager.WriteEntry(entry); err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&operationCount, 1)
					}

					// Transaction operation (every 10th operation)
					if opID%10 == 0 {
						txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							continue
						}

						key := fmt.Sprintf("txn_key_%d", opID)
						value := []byte(fmt.Sprintf("txn_value_%d", opID))

						if err := txnSystem.Insert(txn, key, value); err != nil {
							txnSystem.AbortTransaction(txn)
							atomic.AddInt64(&errorCount, 1)
						} else {
							if err := txnSystem.CommitTransaction(txn); err != nil {
								atomic.AddInt64(&errorCount, 1)
							}
						}
					}
				}
			}
		}()

		// Run for test duration
		time.Sleep(testDuration)
		stopChan <- true

		finalOperationCount := atomic.LoadInt64(&operationCount)
		finalErrorCount := atomic.LoadInt64(&errorCount)

		t.Logf("Long-running stability test results:")
		t.Logf("  Duration: %v", testDuration)
		t.Logf("  Operations completed: %d", finalOperationCount)
		t.Logf("  Errors encountered: %d", finalErrorCount)
		t.Logf("  Error rate: %.2f%%", float64(finalErrorCount)/float64(finalOperationCount+finalErrorCount)*100)

		// Verify system is still functional
		testEntry := &wal.WALEntry{
			TxnID: 999999,
			Operation: wal.Operation{
				Type:  wal.OpInsert,
				Key:   "final_stability_test",
				Value: []byte("final_value"),
			},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(testEntry); err != nil {
			t.Errorf("System should be functional after long-running test: %v", err)
		}
	})
}
