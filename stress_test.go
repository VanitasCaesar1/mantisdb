package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mantisDB/durability"
	"mantisDB/integrity"
	"mantisDB/transaction"
	"mantisDB/wal"
)

// StressTestConfig defines configuration for stress tests
type StressTestConfig struct {
	Duration         time.Duration
	NumGoroutines    int
	OperationsPerSec int
	DataSize         int
	EnableMetrics    bool
}

// DefaultStressTestConfig returns a default stress test configuration
func DefaultStressTestConfig() *StressTestConfig {
	return &StressTestConfig{
		Duration:         60 * time.Second,
		NumGoroutines:    runtime.NumCPU() * 2,
		OperationsPerSec: 1000,
		DataSize:         1024,
		EnableMetrics:    true,
	}
}

// StressTestMetrics holds metrics for stress tests
type StressTestMetrics struct {
	TotalOperations  int64
	SuccessfulOps    int64
	FailedOps        int64
	AverageLatency   time.Duration
	MaxLatency       time.Duration
	MinLatency       time.Duration
	ThroughputPerSec float64
	ErrorRate        float64
	MemoryUsageMB    float64
}

// StressTestRunner manages stress test execution
type StressTestRunner struct {
	config  *StressTestConfig
	metrics *StressTestMetrics
	mutex   sync.RWMutex
}

// NewStressTestRunner creates a new stress test runner
func NewStressTestRunner(config *StressTestConfig) *StressTestRunner {
	if config == nil {
		config = DefaultStressTestConfig()
	}

	return &StressTestRunner{
		config: config,
		metrics: &StressTestMetrics{
			MinLatency: time.Hour, // Initialize to high value
		},
	}
}

// UpdateMetrics updates stress test metrics
func (str *StressTestRunner) UpdateMetrics(latency time.Duration, success bool) {
	str.mutex.Lock()
	defer str.mutex.Unlock()

	atomic.AddInt64(&str.metrics.TotalOperations, 1)

	if success {
		atomic.AddInt64(&str.metrics.SuccessfulOps, 1)
	} else {
		atomic.AddInt64(&str.metrics.FailedOps, 1)
	}

	if latency > str.metrics.MaxLatency {
		str.metrics.MaxLatency = latency
	}
	if latency < str.metrics.MinLatency {
		str.metrics.MinLatency = latency
	}
}

// GetMetrics returns current stress test metrics
func (str *StressTestRunner) GetMetrics() StressTestMetrics {
	str.mutex.RLock()
	defer str.mutex.RUnlock()

	metrics := *str.metrics

	if metrics.TotalOperations > 0 {
		metrics.ErrorRate = float64(metrics.FailedOps) / float64(metrics.TotalOperations) * 100
		metrics.ThroughputPerSec = float64(metrics.TotalOperations) / str.config.Duration.Seconds()
	}

	// Get memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics.MemoryUsageMB = float64(m.Alloc) / 1024 / 1024

	return metrics
}

// TestHighThroughputWAL tests WAL system under high throughput
func TestHighThroughputWAL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high throughput WAL test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "high_throughput_wal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL manager with optimized settings
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = tempDir
	walConfig.MaxFileSize = 10 * 1024 * 1024 // 10MB files
	walConfig.BufferSize = 64 * 1024         // 64KB buffer
	walConfig.SyncMode = wal.SyncModeAsync   // Async for throughput
	walConfig.SyncInterval = 100 * time.Millisecond

	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer walManager.Close()

	// Stress test configuration
	config := &StressTestConfig{
		Duration:         10 * time.Second, // Shorter for CI
		NumGoroutines:    20,
		OperationsPerSec: 5000,
		DataSize:         512,
		EnableMetrics:    true,
	}

	runner := NewStressTestRunner(config)

	t.Logf("Starting high throughput WAL test:")
	t.Logf("  Duration: %v", config.Duration)
	t.Logf("  Goroutines: %d", config.NumGoroutines)
	t.Logf("  Target ops/sec: %d", config.OperationsPerSec)

	var wg sync.WaitGroup
	stopChan := make(chan bool)
	startTime := time.Now()

	// Start worker goroutines
	for i := 0; i < config.NumGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			opID := 0
			ticker := time.NewTicker(time.Second / time.Duration(config.OperationsPerSec/config.NumGoroutines))
			defer ticker.Stop()

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					opID++
					opStart := time.Now()

					// Create WAL entry
					data := make([]byte, config.DataSize)
					rand.Read(data)

					entry := &wal.WALEntry{
						TxnID: uint64(workerID*10000 + opID),
						Operation: wal.Operation{
							Type:  wal.OpInsert,
							Key:   fmt.Sprintf("throughput_key_%d_%d", workerID, opID),
							Value: data,
						},
						Timestamp: time.Now(),
					}

					// Write entry
					err := walManager.WriteEntry(entry)
					latency := time.Since(opStart)

					runner.UpdateMetrics(latency, err == nil)

					if err != nil {
						t.Logf("Worker %d: Write failed: %v", workerID, err)
					}
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(config.Duration)
	close(stopChan)
	wg.Wait()

	// Force final sync
	if err := walManager.Sync(); err != nil {
		t.Logf("Final sync failed: %v", err)
	}

	duration := time.Since(startTime)
	metrics := runner.GetMetrics()

	t.Logf("High throughput WAL test results:")
	t.Logf("  Actual duration: %v", duration)
	t.Logf("  Total operations: %d", metrics.TotalOperations)
	t.Logf("  Successful: %d", metrics.SuccessfulOps)
	t.Logf("  Failed: %d", metrics.FailedOps)
	t.Logf("  Throughput: %.2f ops/sec", metrics.ThroughputPerSec)
	t.Logf("  Error rate: %.2f%%", metrics.ErrorRate)
	t.Logf("  Max latency: %v", metrics.MaxLatency)
	t.Logf("  Min latency: %v", metrics.MinLatency)
	t.Logf("  Memory usage: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Final LSN: %d", walManager.GetCurrentLSN())

	// Verify system is still functional
	testEntry := &wal.WALEntry{
		TxnID: 999999,
		Operation: wal.Operation{
			Type:  wal.OpInsert,
			Key:   "final_throughput_test",
			Value: []byte("final_value"),
		},
		Timestamp: time.Now(),
	}

	if err := walManager.WriteEntry(testEntry); err != nil {
		t.Errorf("System should be functional after throughput test: %v", err)
	}
}

// TestConcurrentTransactionStress tests transaction system under high concurrency
func TestConcurrentTransactionStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent transaction stress test in short mode")
	}

	// Create transaction system
	lockManager := transaction.NewLockManager(2 * time.Second) // Shorter timeout for stress
	defer lockManager.Close()

	txnSystemConfig := transaction.DefaultTransactionSystemConfig()
	txnSystemConfig.LockTimeout = 2 * time.Second
	txnSystemConfig.DeadlockDetectionInterval = 50 * time.Millisecond
	txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
	if err := txnSystem.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer txnSystem.Stop()

	config := &StressTestConfig{
		Duration:      15 * time.Second,
		NumGoroutines: 30,
		EnableMetrics: true,
	}

	runner := NewStressTestRunner(config)

	t.Logf("Starting concurrent transaction stress test:")
	t.Logf("  Duration: %v", config.Duration)
	t.Logf("  Goroutines: %d", config.NumGoroutines)

	var wg sync.WaitGroup
	stopChan := make(chan bool)
	startTime := time.Now()

	// Shared resources for contention
	sharedKeys := make([]string, 100)
	for i := range sharedKeys {
		sharedKeys[i] = fmt.Sprintf("shared_key_%d", i)
	}

	// Start worker goroutines
	for i := 0; i < config.NumGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			txnCount := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					txnCount++
					txnStart := time.Now()

					// Begin transaction
					txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
					if err != nil {
						runner.UpdateMetrics(time.Since(txnStart), false)
						continue
					}

					success := true

					// Perform multiple operations with potential conflicts
					numOps := rand.Intn(5) + 1
					for op := 0; op < numOps; op++ {
						// Mix of shared and private keys
						var key string
						if rand.Float64() < 0.3 { // 30% chance of shared key
							key = sharedKeys[rand.Intn(len(sharedKeys))]
						} else {
							key = fmt.Sprintf("private_key_%d_%d_%d", workerID, txnCount, op)
						}

						value := []byte(fmt.Sprintf("value_%d_%d_%d", workerID, txnCount, op))

						// Random operation type
						switch rand.Intn(3) {
						case 0: // Insert
							if err := txnSystem.Insert(txn, key, value); err != nil {
								success = false
							}
						case 1: // Update (simulate)
							if err := txnSystem.Write(txn, key, value); err != nil {
								success = false
							}
						case 2: // Read
							if _, err := txnSystem.Read(txn, key); err != nil {
								// Read failures are less critical
							}
						}
					}

					// Commit or abort
					var finalErr error
					if success && rand.Float64() < 0.9 { // 90% commit rate
						finalErr = txnSystem.CommitTransaction(txn)
					} else {
						finalErr = txnSystem.AbortTransaction(txn)
					}

					latency := time.Since(txnStart)
					runner.UpdateMetrics(latency, finalErr == nil && success)

					// Small delay to prevent overwhelming the system
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Monitor deadlocks
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				deadlocks := lockManager.DetectDeadlocks()
				if len(deadlocks) > 0 {
					t.Logf("Detected %d deadlocks", len(deadlocks))
				}
			}
		}
	}()

	// Run for specified duration
	time.Sleep(config.Duration)
	close(stopChan)
	wg.Wait()

	duration := time.Since(startTime)
	metrics := runner.GetMetrics()

	t.Logf("Concurrent transaction stress test results:")
	t.Logf("  Actual duration: %v", duration)
	t.Logf("  Total transactions: %d", metrics.TotalOperations)
	t.Logf("  Successful: %d", metrics.SuccessfulOps)
	t.Logf("  Failed: %d", metrics.FailedOps)
	t.Logf("  Throughput: %.2f txns/sec", metrics.ThroughputPerSec)
	t.Logf("  Error rate: %.2f%%", metrics.ErrorRate)
	t.Logf("  Max latency: %v", metrics.MaxLatency)
	t.Logf("  Min latency: %v", metrics.MinLatency)
	t.Logf("  Memory usage: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Active transactions: %d", txnSystem.GetTransactionCount())

	// Verify no transactions are left hanging
	if txnSystem.GetTransactionCount() != 0 {
		t.Errorf("Expected 0 active transactions, got %d", txnSystem.GetTransactionCount())
	}
}

// TestMassiveDataIntegrityStress tests integrity system with massive data
func TestMassiveDataIntegrityStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping massive data integrity stress test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "massive_integrity_stress_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create integrity system
	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

	integrityConfig := &integrity.IntegrityConfig{
		ChecksumAlgorithm:    integrity.ChecksumCRC32,
		ScanInterval:         10 * time.Millisecond,
		EnableBackgroundScan: true,
	}

	integritySystem := integrity.NewIntegritySystem(integrityConfig)

	config := &StressTestConfig{
		Duration:      20 * time.Second,
		NumGoroutines: 15,
		DataSize:      4096, // 4KB per operation
		EnableMetrics: true,
	}

	runner := NewStressTestRunner(config)

	t.Logf("Starting massive data integrity stress test:")
	t.Logf("  Duration: %v", config.Duration)
	t.Logf("  Goroutines: %d", config.NumGoroutines)
	t.Logf("  Data size per op: %d bytes", config.DataSize)

	var wg sync.WaitGroup
	stopChan := make(chan bool)
	startTime := time.Now()

	// Start worker goroutines
	for i := 0; i < config.NumGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			opCount := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					opCount++
					opStart := time.Now()

					// Generate test data
					data := make([]byte, config.DataSize)
					rand.Read(data)

					// Calculate checksum
					expectedChecksum := checksumEngine.Calculate(data)

					// Randomly corrupt some data (5% corruption rate)
					testData := make([]byte, len(data))
					copy(testData, data)

					if rand.Float64() < 0.05 {
						// Corrupt random byte
						corruptIndex := rand.Intn(len(testData))
						testData[corruptIndex] ^= 0xFF
					}

					// Verify integrity
					err := checksumEngine.Verify(testData, expectedChecksum)
					success := (err == nil && rand.Float64() >= 0.05) || (err != nil && rand.Float64() < 0.05)

					latency := time.Since(opStart)
					runner.UpdateMetrics(latency, success)

					// Batch operations for better performance
					if opCount%100 == 0 {
						// Create batch data
						batchSize := 10
						batchData := make([][]byte, batchSize)
						expectedChecksums := make([]uint32, batchSize)

						for j := 0; j < batchSize; j++ {
							batchData[j] = make([]byte, config.DataSize/4) // Smaller for batch
							rand.Read(batchData[j])
							expectedChecksums[j] = checksumEngine.Calculate(batchData[j])
						}

						// Verify batch
						batchStart := time.Now()
						errors := checksumEngine.VerifyBatch(batchData, expectedChecksums)
						batchLatency := time.Since(batchStart)

						batchSuccess := true
						for _, err := range errors {
							if err != nil {
								batchSuccess = false
								break
							}
						}

						runner.UpdateMetrics(batchLatency, batchSuccess)
					}

					// File-based integrity check (every 50 operations)
					if opCount%50 == 0 {
						filename := fmt.Sprintf("integrity_file_%d_%d.dat", workerID, opCount)
						filePath := filepath.Join(tempDir, filename)

						fileStart := time.Now()

						// Write file
						if err := os.WriteFile(filePath, data, 0644); err != nil {
							runner.UpdateMetrics(time.Since(fileStart), false)
							continue
						}

						// Calculate file checksum
						fileChecksum, err := checksumEngine.CalculateFileChecksum(filePath)
						if err != nil {
							runner.UpdateMetrics(time.Since(fileStart), false)
							continue
						}

						// Verify file checksum
						err = checksumEngine.VerifyFileChecksum(filePath, fileChecksum)
						fileLatency := time.Since(fileStart)
						runner.UpdateMetrics(fileLatency, err == nil)

						// Verify file integrity
						integritySystem.VerifyFileIntegrity(filePath)

						// Clean up file to prevent disk space issues
						os.Remove(filePath)
					}

					// Small delay to prevent overwhelming
					time.Sleep(100 * time.Microsecond)
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(config.Duration)
	close(stopChan)
	wg.Wait()

	duration := time.Since(startTime)
	metrics := runner.GetMetrics()
	integrityMetrics := integritySystem.GetMetrics()

	t.Logf("Massive data integrity stress test results:")
	t.Logf("  Actual duration: %v", duration)
	t.Logf("  Total operations: %d", metrics.TotalOperations)
	t.Logf("  Successful: %d", metrics.SuccessfulOps)
	t.Logf("  Failed: %d", metrics.FailedOps)
	t.Logf("  Throughput: %.2f ops/sec", metrics.ThroughputPerSec)
	t.Logf("  Error rate: %.2f%%", metrics.ErrorRate)
	t.Logf("  Max latency: %v", metrics.MaxLatency)
	t.Logf("  Min latency: %v", metrics.MinLatency)
	t.Logf("  Memory usage: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Integrity system metrics:")
	if integrityMetrics.ChecksumOperations != nil {
		t.Logf("    Checksum operations: %d", integrityMetrics.ChecksumOperations.TotalOperations)
	}
	if integrityMetrics.CorruptionDetection != nil {
		t.Logf("    Corruption checks: %d", integrityMetrics.CorruptionDetection.TotalOperations)
	}
	if integrityMetrics.BackgroundScans != nil {
		t.Logf("    Files scanned: %d", integrityMetrics.BackgroundScans.FilesScanned)
	}
}

// TestMemoryLeakDetection tests for memory leaks under stress
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak detection test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "memory_leak_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Record initial memory stats
	var initialStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialStats)

	t.Logf("Initial memory stats:")
	t.Logf("  Alloc: %.2f MB", float64(initialStats.Alloc)/1024/1024)
	t.Logf("  TotalAlloc: %.2f MB", float64(initialStats.TotalAlloc)/1024/1024)
	t.Logf("  Sys: %.2f MB", float64(initialStats.Sys)/1024/1024)

	// Create components
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = tempDir
	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer walManager.Close()

	lockManager := transaction.NewLockManager(5 * time.Second)
	defer lockManager.Close()

	txnSystemConfig := transaction.DefaultTransactionSystemConfig()
	txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
	if err := txnSystem.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer txnSystem.Stop()

	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

	// Run operations that might cause memory leaks
	const numIterations = 1000
	const operationsPerIteration = 100

	for iteration := 0; iteration < numIterations; iteration++ {
		// WAL operations
		for i := 0; i < operationsPerIteration; i++ {
			entry := &wal.WALEntry{
				TxnID: uint64(iteration*operationsPerIteration + i),
				Operation: wal.Operation{
					Type:  wal.OpInsert,
					Key:   fmt.Sprintf("leak_test_key_%d_%d", iteration, i),
					Value: make([]byte, 1024), // 1KB per entry
				},
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(entry); err != nil {
				t.Logf("WAL write failed: %v", err)
			}
		}

		// Transaction operations
		for i := 0; i < operationsPerIteration/10; i++ {
			txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
			if err != nil {
				continue
			}

			key := fmt.Sprintf("leak_txn_key_%d_%d", iteration, i)
			value := make([]byte, 512)

			if err := txnSystem.Insert(txn, key, value); err == nil {
				txnSystem.CommitTransaction(txn)
			} else {
				txnSystem.AbortTransaction(txn)
			}
		}

		// Integrity operations
		for i := 0; i < operationsPerIteration/5; i++ {
			data := make([]byte, 2048)
			rand.Read(data)

			checksum := checksumEngine.Calculate(data)
			checksumEngine.Verify(data, checksum)
		}

		// Check memory every 100 iterations
		if iteration%100 == 0 {
			runtime.GC()
			var currentStats runtime.MemStats
			runtime.ReadMemStats(&currentStats)

			allocMB := float64(currentStats.Alloc) / 1024 / 1024
			sysMB := float64(currentStats.Sys) / 1024 / 1024

			t.Logf("Iteration %d memory stats:", iteration)
			t.Logf("  Alloc: %.2f MB", allocMB)
			t.Logf("  Sys: %.2f MB", sysMB)

			// Check for excessive memory growth
			if allocMB > 500 { // 500MB threshold
				t.Errorf("Potential memory leak detected: allocation at %.2f MB", allocMB)
				break
			}
		}
	}

	// Final memory check
	runtime.GC()
	var finalStats runtime.MemStats
	runtime.ReadMemStats(&finalStats)

	t.Logf("Final memory stats:")
	t.Logf("  Alloc: %.2f MB", float64(finalStats.Alloc)/1024/1024)
	t.Logf("  TotalAlloc: %.2f MB", float64(finalStats.TotalAlloc)/1024/1024)
	t.Logf("  Sys: %.2f MB", float64(finalStats.Sys)/1024/1024)
	t.Logf("  NumGC: %d", finalStats.NumGC)

	// Calculate memory growth
	allocGrowth := float64(finalStats.Alloc-initialStats.Alloc) / 1024 / 1024
	sysGrowth := float64(finalStats.Sys-initialStats.Sys) / 1024 / 1024

	t.Logf("Memory growth:")
	t.Logf("  Alloc growth: %.2f MB", allocGrowth)
	t.Logf("  Sys growth: %.2f MB", sysGrowth)

	// Check for reasonable memory growth
	if allocGrowth > 100 { // 100MB growth threshold
		t.Errorf("Excessive memory growth detected: %.2f MB", allocGrowth)
	}
}

// TestSystemLimits tests system behavior at various limits
func TestSystemLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping system limits test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "system_limits_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("MaxConcurrentTransactions", func(t *testing.T) {
		lockManager := transaction.NewLockManager(1 * time.Second)
		defer lockManager.Close()

		txnSystemConfig := transaction.DefaultTransactionSystemConfig()
		txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
		if err := txnSystem.Start(); err != nil {
			t.Fatalf("Failed to start transaction system: %v", err)
		}
		defer txnSystem.Stop()

		// Try to create many concurrent transactions
		const maxTransactions = 1000
		transactions := make([]*transaction.Transaction, 0, maxTransactions)

		for i := 0; i < maxTransactions; i++ {
			txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
			if err != nil {
				t.Logf("Failed to create transaction %d: %v", i, err)
				break
			}
			transactions = append(transactions, txn)
		}

		t.Logf("Successfully created %d concurrent transactions", len(transactions))

		// Clean up transactions
		for _, txn := range transactions {
			txnSystem.AbortTransaction(txn)
		}

		if len(transactions) < 100 {
			t.Errorf("Expected to create at least 100 transactions, got %d", len(transactions))
		}
	})

	t.Run("LargeWALEntries", func(t *testing.T) {
		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = tempDir
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		// Test with increasingly large entries
		sizes := []int{1024, 10240, 102400, 1048576} // 1KB to 1MB

		for _, size := range sizes {
			t.Logf("Testing WAL entry size: %d bytes", size)

			data := make([]byte, size)
			rand.Read(data)

			entry := &wal.WALEntry{
				TxnID: uint64(size),
				Operation: wal.Operation{
					Type:  wal.OpInsert,
					Key:   fmt.Sprintf("large_key_%d", size),
					Value: data,
				},
				Timestamp: time.Now(),
			}

			start := time.Now()
			err := walManager.WriteEntry(entry)
			latency := time.Since(start)

			if err != nil {
				t.Errorf("Failed to write %d byte entry: %v", size, err)
			} else {
				t.Logf("Successfully wrote %d byte entry in %v", size, latency)
			}
		}
	})

	t.Run("ManySmallFiles", func(t *testing.T) {
		durabilityConfig := durability.DefaultDurabilityConfig()
		durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
		if err != nil {
			t.Fatalf("Failed to create durability manager: %v", err)
		}
		defer durabilityManager.Close(context.Background())

		ctx := context.Background()
		const numFiles = 10000
		const fileSize = 100

		start := time.Now()
		successCount := 0

		for i := 0; i < numFiles; i++ {
			filename := fmt.Sprintf("small_file_%d.dat", i)
			filePath := filepath.Join(tempDir, filename)
			data := make([]byte, fileSize)
			rand.Read(data)

			if err := durabilityManager.Write(ctx, filePath, data, 0); err != nil {
				t.Logf("Failed to write file %d: %v", i, err)
			} else {
				successCount++
			}

			if i%1000 == 0 {
				t.Logf("Created %d files", i)
			}
		}

		duration := time.Since(start)
		t.Logf("Created %d small files in %v (%.2f files/sec)",
			successCount, duration, float64(successCount)/duration.Seconds())

		if successCount < numFiles/2 {
			t.Errorf("Expected to create at least %d files, got %d", numFiles/2, successCount)
		}
	})
}
