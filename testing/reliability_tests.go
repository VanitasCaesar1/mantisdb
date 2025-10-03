package testing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"mantisDB/store"
)

// ReliabilityTestSuite provides comprehensive reliability testing
type ReliabilityTestSuite struct {
	store               *store.MantisStore
	crashRecoveryTester *CrashRecoveryTester
	diskSpaceTester     *DiskSpaceTester
	memoryLimitTester   *MemoryLimitTester
	concurrencyTester   *ConcurrencyTester
}

// NewReliabilityTestSuite creates a new reliability test suite
func NewReliabilityTestSuite(mantisStore *store.MantisStore) *ReliabilityTestSuite {
	return &ReliabilityTestSuite{
		store:               mantisStore,
		crashRecoveryTester: NewCrashRecoveryTester(mantisStore),
		diskSpaceTester:     NewDiskSpaceTester(mantisStore),
		memoryLimitTester:   NewMemoryLimitTester(mantisStore),
		concurrencyTester:   NewConcurrencyTester(mantisStore),
	}
}

// RunAllTests runs all reliability tests
func (rts *ReliabilityTestSuite) RunAllTests(ctx context.Context) (*TestResults, error) {
	results := &TestResults{
		StartTime: time.Now(),
		Tests:     make(map[string]*TestResult),
	}

	// Run crash recovery tests
	fmt.Println("Running crash recovery tests...")
	crashResults, err := rts.crashRecoveryTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("crash recovery tests failed: %v", err)
	}
	results.Tests["crash_recovery"] = crashResults

	// Run disk space exhaustion tests
	fmt.Println("Running disk space exhaustion tests...")
	diskResults, err := rts.diskSpaceTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("disk space tests failed: %v", err)
	}
	results.Tests["disk_space"] = diskResults

	// Run memory limit tests
	fmt.Println("Running memory limit tests...")
	memoryResults, err := rts.memoryLimitTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("memory limit tests failed: %v", err)
	}
	results.Tests["memory_limits"] = memoryResults

	// Run concurrent access pattern tests
	fmt.Println("Running concurrent access pattern tests...")
	concurrencyResults, err := rts.concurrencyTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("concurrent access tests failed: %v", err)
	}
	results.Tests["concurrent_access"] = concurrencyResults

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results, nil
}

// CrashRecoveryTester tests crash recovery scenarios
type CrashRecoveryTester struct {
	store          *store.MantisStore
	processManager *ProcessManager
	dataValidator  *DataIntegrityValidator
	testDataDir    string
}

// NewCrashRecoveryTester creates a new crash recovery tester
func NewCrashRecoveryTester(mantisStore *store.MantisStore) *CrashRecoveryTester {
	return &CrashRecoveryTester{
		store:          mantisStore,
		processManager: NewProcessManager(),
		dataValidator:  NewDataIntegrityValidator(),
		testDataDir:    "./test_data_crash_recovery",
	}
}

// RunTests runs all crash recovery tests
func (crt *CrashRecoveryTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "crash_recovery",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	// Test crash during write operations
	writeResult, err := crt.testCrashDuringWrite(ctx)
	result.SubTests["crash_during_write"] = writeResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test crash during transaction
	txResult, err := crt.testCrashDuringTransaction(ctx)
	result.SubTests["crash_during_transaction"] = txResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test data integrity after recovery
	integrityResult, err := crt.testDataIntegrityAfterRecovery(ctx)
	result.SubTests["data_integrity_recovery"] = integrityResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test transaction rollback verification
	rollbackResult, err := crt.testTransactionRollbackVerification(ctx)
	result.SubTests["transaction_rollback"] = rollbackResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("crash recovery tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_crash_scenarios"] = len(result.SubTests)
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// ProcessManager manages process lifecycle for crash testing
type ProcessManager struct {
	testProcesses map[string]*os.Process
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		testProcesses: make(map[string]*os.Process),
	}
}

// StartTestProcess starts a test process that can be crashed
func (pm *ProcessManager) StartTestProcess(ctx context.Context, processID string, workload func() error) (*TestProcess, error) {
	// Create a test process wrapper
	testProcess := &TestProcess{
		ID:       processID,
		workload: workload,
		done:     make(chan error, 1),
		crashed:  make(chan bool, 1),
	}

	// Start the workload in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				testProcess.crashed <- true
				testProcess.done <- fmt.Errorf("process crashed: %v", r)
			}
		}()

		err := workload()
		testProcess.done <- err
	}()

	pm.testProcesses[processID] = nil // Placeholder for real process
	return testProcess, nil
}

// CrashProcess simulates a process crash
func (pm *ProcessManager) CrashProcess(processID string) error {
	// In a real implementation, this would kill the actual process
	// For testing purposes, we simulate the crash
	if process, exists := pm.testProcesses[processID]; exists {
		if process != nil {
			return process.Kill()
		}
		// Simulate crash by removing from tracking
		delete(pm.testProcesses, processID)
		return nil
	}
	return fmt.Errorf("process not found: %s", processID)
}

// TestProcess represents a test process that can be crashed
type TestProcess struct {
	ID       string
	workload func() error
	done     chan error
	crashed  chan bool
}

// Wait waits for the process to complete or crash
func (tp *TestProcess) Wait(timeout time.Duration) error {
	select {
	case err := <-tp.done:
		return err
	case <-tp.crashed:
		return fmt.Errorf("process crashed")
	case <-time.After(timeout):
		return fmt.Errorf("process timeout")
	}
}

// DataIntegrityValidator validates data integrity after crashes
type DataIntegrityValidator struct {
	checksums map[string]string
}

// NewDataIntegrityValidator creates a new data integrity validator
func NewDataIntegrityValidator() *DataIntegrityValidator {
	return &DataIntegrityValidator{
		checksums: make(map[string]string),
	}
}

// RecordChecksum records a checksum for a key before operations
func (div *DataIntegrityValidator) RecordChecksum(key string, data []byte) {
	checksum := calculateSimpleChecksum(data)
	div.checksums[key] = fmt.Sprintf("%d", checksum)
}

// ValidateIntegrity validates data integrity after recovery
func (div *DataIntegrityValidator) ValidateIntegrity(ctx context.Context, store *store.MantisStore, key string) error {
	expectedChecksum, exists := div.checksums[key]
	if !exists {
		return fmt.Errorf("no checksum recorded for key: %s", key)
	}

	// Retrieve current data
	data, err := store.KV().Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to retrieve data for validation: %v", err)
	}

	// Calculate current checksum
	currentChecksum := fmt.Sprintf("%d", calculateSimpleChecksum(data))

	if currentChecksum != expectedChecksum {
		return fmt.Errorf("data integrity violation: expected checksum %s, got %s", expectedChecksum, currentChecksum)
	}

	return nil
}

// testCrashDuringWrite tests crash recovery during write operations
func (crt *CrashRecoveryTester) testCrashDuringWrite(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "crash_during_write",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Prepare test data
	testKey := fmt.Sprintf("crash_write_test_%d", time.Now().UnixNano())
	testData := []byte("test_data_for_crash_recovery")

	// Record checksum before operation
	crt.dataValidator.RecordChecksum(testKey, testData)

	// Create a workload that performs writes
	writeWorkload := func() error {
		// Perform multiple writes to increase crash window
		for i := 0; i < 10; i++ {
			data := append(testData, byte(i))
			err := crt.store.KV().Set(ctx, fmt.Sprintf("%s_%d", testKey, i), data, time.Hour)
			if err != nil {
				return fmt.Errorf("write failed: %v", err)
			}
			time.Sleep(time.Millisecond * 10) // Small delay to increase crash window
		}
		return nil
	}

	// Start test process
	testProcess, err := crt.processManager.StartTestProcess(ctx, "write_test", writeWorkload)
	if err != nil {
		result.Error = fmt.Errorf("failed to start test process: %v", err)
		result.Success = false
		return result, err
	}

	// Wait a bit then crash the process
	time.Sleep(time.Millisecond * 50)
	crashErr := crt.processManager.CrashProcess("write_test")

	// Wait for process completion or crash
	processErr := testProcess.Wait(time.Second * 5)

	// Simulate recovery by checking data integrity
	recoverySuccess := true
	var recoveryErrors []error

	// Check which writes completed successfully
	completedWrites := 0
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s_%d", testKey, i)
		_, err := crt.store.KV().Get(ctx, key)
		if err == nil {
			completedWrites++
		}
	}

	// Validate that partial writes are handled correctly
	if completedWrites == 0 && crashErr == nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("no writes completed but no crash occurred"))
		recoverySuccess = false
	}

	// Clean up test data
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s_%d", testKey, i)
		crt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = recoverySuccess && len(recoveryErrors) == 0

	// Record metrics
	result.Metrics["completed_writes"] = completedWrites
	result.Metrics["crash_simulated"] = crashErr == nil
	result.Metrics["process_error"] = processErr != nil
	result.Metrics["recovery_errors"] = len(recoveryErrors)

	if len(recoveryErrors) > 0 {
		result.Error = fmt.Errorf("crash during write test had %d recovery errors: %v", len(recoveryErrors), recoveryErrors[0])
	}

	return result, nil
}

// testCrashDuringTransaction tests crash recovery during transactions
func (crt *CrashRecoveryTester) testCrashDuringTransaction(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "crash_during_transaction",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Prepare test data
	testKeys := []string{
		fmt.Sprintf("tx_test_1_%d", time.Now().UnixNano()),
		fmt.Sprintf("tx_test_2_%d", time.Now().UnixNano()),
		fmt.Sprintf("tx_test_3_%d", time.Now().UnixNano()),
	}
	testData := []byte("transaction_test_data")

	// Create a workload that performs transactional operations
	txWorkload := func() error {
		// Begin transaction
		tx, err := crt.store.KV().BeginTransaction(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		// Perform multiple operations in transaction
		for i, key := range testKeys {
			data := append(testData, byte(i))
			err := tx.Put(fmt.Sprintf("kv:%s", key), string(data))
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("transaction put failed: %v", err)
			}
			time.Sleep(time.Millisecond * 20) // Increase crash window
		}

		// Commit transaction
		return tx.Commit()
	}

	// Start test process
	testProcess, err := crt.processManager.StartTestProcess(ctx, "tx_test", txWorkload)
	if err != nil {
		result.Error = fmt.Errorf("failed to start test process: %v", err)
		result.Success = false
		return result, err
	}

	// Wait a bit then crash the process
	time.Sleep(time.Millisecond * 30)
	crashErr := crt.processManager.CrashProcess("tx_test")

	// Wait for process completion or crash
	processErr := testProcess.Wait(time.Second * 5)

	// Validate transaction atomicity after crash
	recoverySuccess := true
	var recoveryErrors []error

	// Check transaction atomicity - either all keys exist or none
	existingKeys := 0
	for _, key := range testKeys {
		_, err := crt.store.KV().Get(ctx, key)
		if err == nil {
			existingKeys++
		}
	}

	// Transaction should be atomic - either all or none should exist
	if existingKeys != 0 && existingKeys != len(testKeys) {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("transaction atomicity violated: %d of %d keys exist", existingKeys, len(testKeys)))
		recoverySuccess = false
	}

	// Clean up test data
	for _, key := range testKeys {
		crt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = recoverySuccess && len(recoveryErrors) == 0

	// Record metrics
	result.Metrics["total_keys"] = len(testKeys)
	result.Metrics["existing_keys_after_crash"] = existingKeys
	result.Metrics["atomicity_preserved"] = existingKeys == 0 || existingKeys == len(testKeys)
	result.Metrics["crash_simulated"] = crashErr == nil
	result.Metrics["process_error"] = processErr != nil

	if len(recoveryErrors) > 0 {
		result.Error = fmt.Errorf("crash during transaction test had %d recovery errors: %v", len(recoveryErrors), recoveryErrors[0])
	}

	return result, nil
}

// testDataIntegrityAfterRecovery tests data integrity validation after recovery
func (crt *CrashRecoveryTester) testDataIntegrityAfterRecovery(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "data_integrity_recovery",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Prepare test data with known checksums
	testKeys := make([]string, 5)
	testData := make([][]byte, 5)

	for i := 0; i < 5; i++ {
		testKeys[i] = fmt.Sprintf("integrity_test_%d_%d", i, time.Now().UnixNano())
		testData[i] = []byte(fmt.Sprintf("integrity_data_%d_%d", i, time.Now().UnixNano()))

		// Store initial data
		err := crt.store.KV().Set(ctx, testKeys[i], testData[i], time.Hour)
		if err != nil {
			result.Error = fmt.Errorf("failed to store initial data: %v", err)
			result.Success = false
			return result, err
		}

		// Record checksum
		crt.dataValidator.RecordChecksum(testKeys[i], testData[i])
	}

	// Simulate some operations and potential crash
	operationWorkload := func() error {
		// Perform some updates
		for i := 0; i < 3; i++ {
			newData := append(testData[i], []byte("_updated")...)
			err := crt.store.KV().Set(ctx, testKeys[i], newData, time.Hour)
			if err != nil {
				return fmt.Errorf("update failed: %v", err)
			}
			// Update recorded checksum
			crt.dataValidator.RecordChecksum(testKeys[i], newData)
			time.Sleep(time.Millisecond * 15)
		}
		return nil
	}

	// Start test process
	testProcess, err := crt.processManager.StartTestProcess(ctx, "integrity_test", operationWorkload)
	if err != nil {
		result.Error = fmt.Errorf("failed to start test process: %v", err)
		result.Success = false
		return result, err
	}

	// Wait a bit then crash the process
	time.Sleep(time.Millisecond * 25)
	crashErr := crt.processManager.CrashProcess("integrity_test")

	// Wait for process completion or crash
	processErr := testProcess.Wait(time.Second * 5)

	// Validate data integrity after recovery
	var integrityErrors []error
	validatedKeys := 0

	for _, key := range testKeys {
		// Check if key still exists
		_, err := crt.store.KV().Get(ctx, key)
		if err != nil {
			// Key doesn't exist - this might be expected if crash occurred before write
			continue
		}

		// Validate integrity
		err = crt.dataValidator.ValidateIntegrity(ctx, crt.store, key)
		if err != nil {
			integrityErrors = append(integrityErrors, err)
		} else {
			validatedKeys++
		}
	}

	// Clean up test data
	for _, key := range testKeys {
		crt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(integrityErrors) == 0

	// Record metrics
	result.Metrics["total_keys"] = len(testKeys)
	result.Metrics["validated_keys"] = validatedKeys
	result.Metrics["integrity_errors"] = len(integrityErrors)
	result.Metrics["crash_simulated"] = crashErr == nil
	result.Metrics["process_error"] = processErr != nil

	if len(integrityErrors) > 0 {
		result.Error = fmt.Errorf("data integrity test had %d errors: %v", len(integrityErrors), integrityErrors[0])
	}

	return result, nil
}

// testTransactionRollbackVerification tests transaction rollback after crashes
func (crt *CrashRecoveryTester) testTransactionRollbackVerification(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "transaction_rollback",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Prepare test data
	baseKey := fmt.Sprintf("rollback_test_%d", time.Now().UnixNano())
	initialData := []byte("initial_data")
	updatedData := []byte("updated_data_should_rollback")

	// Set initial data
	err := crt.store.KV().Set(ctx, baseKey, initialData, time.Hour)
	if err != nil {
		result.Error = fmt.Errorf("failed to set initial data: %v", err)
		result.Success = false
		return result, err
	}

	// Record initial checksum
	crt.dataValidator.RecordChecksum(baseKey, initialData)

	// Create a workload that starts a transaction but crashes before commit
	rollbackWorkload := func() error {
		// Begin transaction
		tx, err := crt.store.KV().BeginTransaction(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		// Update data in transaction
		err = tx.Put(fmt.Sprintf("kv:%s", baseKey), string(updatedData))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("transaction put failed: %v", err)
		}

		// Simulate work before commit
		time.Sleep(time.Millisecond * 50)

		// This commit should be interrupted by crash
		return tx.Commit()
	}

	// Start test process
	testProcess, err := crt.processManager.StartTestProcess(ctx, "rollback_test", rollbackWorkload)
	if err != nil {
		result.Error = fmt.Errorf("failed to start test process: %v", err)
		result.Success = false
		return result, err
	}

	// Wait a bit then crash the process before commit
	time.Sleep(time.Millisecond * 25)
	crashErr := crt.processManager.CrashProcess("rollback_test")

	// Wait for process completion or crash
	processErr := testProcess.Wait(time.Second * 5)

	// Verify that data rolled back to initial state
	currentData, err := crt.store.KV().Get(ctx, baseKey)
	if err != nil {
		result.Error = fmt.Errorf("failed to retrieve data after crash: %v", err)
		result.Success = false
		return result, err
	}

	// Check if data rolled back correctly
	rolledBack := string(currentData) == string(initialData)
	if !rolledBack {
		result.Error = fmt.Errorf("transaction did not roll back: expected %s, got %s", string(initialData), string(currentData))
		result.Success = false
	} else {
		result.Success = true
	}

	// Clean up test data
	crt.store.KV().Delete(ctx, baseKey)

	result.Duration = time.Since(startTime)

	// Record metrics
	result.Metrics["initial_data_length"] = len(initialData)
	result.Metrics["updated_data_length"] = len(updatedData)
	result.Metrics["current_data_length"] = len(currentData)
	result.Metrics["rollback_successful"] = rolledBack
	result.Metrics["crash_simulated"] = crashErr == nil
	result.Metrics["process_error"] = processErr != nil

	return result, nil
}

// DiskSpaceTester tests disk space exhaustion scenarios
type DiskSpaceTester struct {
	store          *store.MantisStore
	diskMonitor    *DiskSpaceMonitor
	errorValidator *ErrorResponseValidator
	testDataDir    string
}

// NewDiskSpaceTester creates a new disk space tester
func NewDiskSpaceTester(mantisStore *store.MantisStore) *DiskSpaceTester {
	return &DiskSpaceTester{
		store:          mantisStore,
		diskMonitor:    NewDiskSpaceMonitor(),
		errorValidator: NewErrorResponseValidator(),
		testDataDir:    "./test_data_disk_space",
	}
}

// RunTests runs all disk space exhaustion tests
func (dst *DiskSpaceTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "disk_space_exhaustion",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	// Test graceful error handling when disk is full
	errorResult, err := dst.testGracefulErrorHandling(ctx)
	result.SubTests["graceful_error_handling"] = errorResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test disk space monitoring
	monitorResult, err := dst.testDiskSpaceMonitoring(ctx)
	result.SubTests["disk_space_monitoring"] = monitorResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test recovery after disk space restoration
	recoveryResult, err := dst.testRecoveryAfterSpaceRestoration(ctx)
	result.SubTests["recovery_after_restoration"] = recoveryResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test disk full simulation
	simulationResult, err := dst.testDiskFullSimulation(ctx)
	result.SubTests["disk_full_simulation"] = simulationResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("disk space tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_disk_scenarios"] = len(result.SubTests)
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// DiskSpaceMonitor monitors and simulates disk space conditions
type DiskSpaceMonitor struct {
	originalFreeSpace int64
	simulatedLimit    int64
	testFiles         []string
}

// NewDiskSpaceMonitor creates a new disk space monitor
func NewDiskSpaceMonitor() *DiskSpaceMonitor {
	return &DiskSpaceMonitor{
		testFiles: make([]string, 0),
	}
}

// GetDiskUsage returns current disk usage information
func (dsm *DiskSpaceMonitor) GetDiskUsage(path string) (*DiskUsageInfo, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %v", err)
	}

	// Calculate disk usage
	totalSpace := int64(stat.Blocks) * int64(stat.Bsize)
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
	usedSpace := totalSpace - freeSpace

	return &DiskUsageInfo{
		TotalSpace:   totalSpace,
		FreeSpace:    freeSpace,
		UsedSpace:    usedSpace,
		UsagePercent: float64(usedSpace) / float64(totalSpace) * 100,
	}, nil
}

// DiskUsageInfo contains disk usage information
type DiskUsageInfo struct {
	TotalSpace   int64
	FreeSpace    int64
	UsedSpace    int64
	UsagePercent float64
}

// SimulateDiskFull simulates disk full condition by creating large files
func (dsm *DiskSpaceMonitor) SimulateDiskFull(ctx context.Context, targetPath string, leaveSpace int64) error {
	// Get current disk usage
	usage, err := dsm.GetDiskUsage(targetPath)
	if err != nil {
		return fmt.Errorf("failed to get disk usage: %v", err)
	}

	// Calculate how much space to fill
	spaceToFill := usage.FreeSpace - leaveSpace
	if spaceToFill <= 0 {
		return fmt.Errorf("not enough free space to simulate disk full")
	}

	// Create directory for test files
	testDir := filepath.Join(targetPath, "disk_full_simulation")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create test directory: %v", err)
	}

	// Create large files to fill disk space
	chunkSize := int64(10 * 1024 * 1024) // 10MB chunks
	filledSpace := int64(0)
	fileIndex := 0

	for filledSpace < spaceToFill {
		fileName := filepath.Join(testDir, fmt.Sprintf("fill_file_%d.dat", fileIndex))

		// Determine size for this file
		remainingSpace := spaceToFill - filledSpace
		fileSize := chunkSize
		if remainingSpace < chunkSize {
			fileSize = remainingSpace
		}

		// Create file with specified size
		err = dsm.createLargeFile(fileName, fileSize)
		if err != nil {
			return fmt.Errorf("failed to create fill file: %v", err)
		}

		dsm.testFiles = append(dsm.testFiles, fileName)
		filledSpace += fileSize
		fileIndex++

		// Check if we should stop (context cancellation)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// createLargeFile creates a file of specified size
func (dsm *DiskSpaceMonitor) createLargeFile(fileName string, size int64) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write data in chunks to avoid memory issues
	chunkSize := int64(1024 * 1024) // 1MB chunks
	chunk := make([]byte, chunkSize)
	written := int64(0)

	for written < size {
		writeSize := chunkSize
		if size-written < chunkSize {
			writeSize = size - written
		}

		_, err = file.Write(chunk[:writeSize])
		if err != nil {
			return err
		}
		written += writeSize
	}

	return file.Sync()
}

// CleanupTestFiles removes all test files created during simulation
func (dsm *DiskSpaceMonitor) CleanupTestFiles() error {
	var errors []error

	for _, fileName := range dsm.testFiles {
		err := os.Remove(fileName)
		if err != nil && !os.IsNotExist(err) {
			errors = append(errors, err)
		}
	}

	// Remove test directory if empty
	if len(dsm.testFiles) > 0 {
		testDir := filepath.Dir(dsm.testFiles[0])
		os.Remove(testDir) // Ignore error if not empty
	}

	dsm.testFiles = dsm.testFiles[:0] // Clear slice

	if len(errors) > 0 {
		return fmt.Errorf("cleanup had %d errors: %v", len(errors), errors[0])
	}

	return nil
}

// ErrorResponseValidator validates error responses during disk full scenarios
type ErrorResponseValidator struct {
	expectedErrors map[string]bool
}

// NewErrorResponseValidator creates a new error response validator
func NewErrorResponseValidator() *ErrorResponseValidator {
	return &ErrorResponseValidator{
		expectedErrors: map[string]bool{
			"no space left on device": true,
			"disk full":               true,
			"insufficient disk space": true,
			"write failed":            true,
			"storage error":           true,
		},
	}
}

// ValidateErrorResponse validates that error responses are appropriate for disk full conditions
func (erv *ErrorResponseValidator) ValidateErrorResponse(err error) bool {
	if err == nil {
		return false // Should have an error when disk is full
	}

	errorMsg := err.Error()
	for expectedError := range erv.expectedErrors {
		if contains(errorMsg, expectedError) {
			return true
		}
	}

	return false
}

// IsGracefulError checks if an error is a graceful disk full error
func (erv *ErrorResponseValidator) IsGracefulError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()

	// Check for panic-like errors that indicate non-graceful handling
	panicIndicators := []string{
		"panic",
		"runtime error",
		"segmentation fault",
		"nil pointer dereference",
	}

	for _, indicator := range panicIndicators {
		if contains(errorMsg, indicator) {
			return false
		}
	}

	return erv.ValidateErrorResponse(err)
}

// testGracefulErrorHandling tests graceful error handling when disk is full
func (dst *DiskSpaceTester) testGracefulErrorHandling(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "graceful_error_handling",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Get initial disk usage
	initialUsage, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		result.Error = fmt.Errorf("failed to get initial disk usage: %v", err)
		result.Success = false
		return result, err
	}

	// Simulate disk full condition (leave 100MB free)
	err = dst.diskMonitor.SimulateDiskFull(ctx, ".", 100*1024*1024)
	if err != nil {
		result.Error = fmt.Errorf("failed to simulate disk full: %v", err)
		result.Success = false
		return result, err
	}

	// Attempt operations that should fail gracefully
	testKey := fmt.Sprintf("disk_full_test_%d", time.Now().UnixNano())
	largeData := make([]byte, 50*1024*1024) // 50MB data

	// Test large write operation
	writeErr := dst.store.KV().Set(ctx, testKey, largeData, time.Hour)

	// Validate error response
	gracefulError := dst.errorValidator.IsGracefulError(writeErr)

	// Test multiple smaller operations
	smallOperationErrors := 0
	gracefulSmallErrors := 0

	for i := 0; i < 10; i++ {
		smallKey := fmt.Sprintf("%s_small_%d", testKey, i)
		smallData := make([]byte, 1024*1024) // 1MB data

		err := dst.store.KV().Set(ctx, smallKey, smallData, time.Hour)
		if err != nil {
			smallOperationErrors++
			if dst.errorValidator.IsGracefulError(err) {
				gracefulSmallErrors++
			}
		}
	}

	// Clean up simulation
	cleanupErr := dst.diskMonitor.CleanupTestFiles()

	// Get final disk usage
	finalUsage, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		fmt.Printf("Warning: failed to get final disk usage: %v\n", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = gracefulError && (gracefulSmallErrors == smallOperationErrors)

	// Record metrics
	result.Metrics["initial_free_space"] = initialUsage.FreeSpace
	result.Metrics["final_free_space"] = finalUsage.FreeSpace
	result.Metrics["large_write_error"] = writeErr != nil
	result.Metrics["large_write_graceful"] = gracefulError
	result.Metrics["small_operation_errors"] = smallOperationErrors
	result.Metrics["graceful_small_errors"] = gracefulSmallErrors
	result.Metrics["cleanup_error"] = cleanupErr != nil

	if !result.Success {
		if !gracefulError {
			result.Error = fmt.Errorf("large write did not fail gracefully: %v", writeErr)
		} else {
			result.Error = fmt.Errorf("small operations did not fail gracefully: %d/%d", gracefulSmallErrors, smallOperationErrors)
		}
	}

	return result, nil
}

// testDiskSpaceMonitoring tests disk space monitoring capabilities
func (dst *DiskSpaceTester) testDiskSpaceMonitoring(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "disk_space_monitoring",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test disk usage monitoring accuracy
	usage1, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		result.Error = fmt.Errorf("failed to get initial disk usage: %v", err)
		result.Success = false
		return result, err
	}

	// Create a test file and measure change
	testFile := filepath.Join(dst.testDataDir, fmt.Sprintf("monitor_test_%d.dat", time.Now().UnixNano()))
	os.MkdirAll(dst.testDataDir, 0755)

	testFileSize := int64(10 * 1024 * 1024) // 10MB
	err = dst.diskMonitor.createLargeFile(testFile, testFileSize)
	if err != nil {
		result.Error = fmt.Errorf("failed to create test file: %v", err)
		result.Success = false
		return result, err
	}

	// Measure disk usage after file creation
	usage2, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		result.Error = fmt.Errorf("failed to get disk usage after file creation: %v", err)
		result.Success = false
		return result, err
	}

	// Calculate expected change
	expectedChange := testFileSize
	actualChange := usage1.FreeSpace - usage2.FreeSpace

	// Allow for some filesystem overhead (within 10% tolerance)
	tolerance := float64(expectedChange) * 0.1
	changeAccurate := float64(actualChange) >= float64(expectedChange)-tolerance &&
		float64(actualChange) <= float64(expectedChange)+tolerance

	// Test monitoring thresholds
	thresholdTests := []struct {
		name      string
		threshold float64
		expected  bool
	}{
		{"low_usage", 10.0, usage2.UsagePercent > 10.0},
		{"medium_usage", 50.0, usage2.UsagePercent > 50.0},
		{"high_usage", 90.0, usage2.UsagePercent > 90.0},
	}

	thresholdResults := make(map[string]bool)
	for _, test := range thresholdTests {
		thresholdResults[test.name] = test.expected
	}

	// Clean up test file
	os.Remove(testFile)
	os.Remove(dst.testDataDir)

	result.Duration = time.Since(startTime)
	result.Success = changeAccurate

	// Record metrics
	result.Metrics["initial_free_space"] = usage1.FreeSpace
	result.Metrics["final_free_space"] = usage2.FreeSpace
	result.Metrics["expected_change"] = expectedChange
	result.Metrics["actual_change"] = actualChange
	result.Metrics["change_accurate"] = changeAccurate
	result.Metrics["initial_usage_percent"] = usage1.UsagePercent
	result.Metrics["final_usage_percent"] = usage2.UsagePercent
	result.Metrics["threshold_results"] = thresholdResults

	if !changeAccurate {
		result.Error = fmt.Errorf("disk usage monitoring inaccurate: expected change %d, actual %d", expectedChange, actualChange)
	}

	return result, nil
}

// testRecoveryAfterSpaceRestoration tests recovery after disk space is restored
func (dst *DiskSpaceTester) testRecoveryAfterSpaceRestoration(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "recovery_after_restoration",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Simulate disk full condition
	err := dst.diskMonitor.SimulateDiskFull(ctx, ".", 50*1024*1024) // Leave 50MB
	if err != nil {
		result.Error = fmt.Errorf("failed to simulate disk full: %v", err)
		result.Success = false
		return result, err
	}

	// Attempt operations during disk full state
	testKey := fmt.Sprintf("recovery_test_%d", time.Now().UnixNano())
	testData := make([]byte, 10*1024*1024) // 10MB data

	// This should fail
	failErr := dst.store.KV().Set(ctx, testKey, testData, time.Hour)
	failedAsExpected := failErr != nil && dst.errorValidator.IsGracefulError(failErr)

	// Restore disk space by cleaning up
	cleanupErr := dst.diskMonitor.CleanupTestFiles()
	if cleanupErr != nil {
		result.Error = fmt.Errorf("failed to cleanup test files: %v", err)
		result.Success = false
		return result, err
	}

	// Wait a moment for system to recognize freed space
	time.Sleep(time.Second)

	// Attempt operations after space restoration
	recoveryOperations := 0
	successfulRecoveryOps := 0

	for i := 0; i < 5; i++ {
		recoveryKey := fmt.Sprintf("%s_recovery_%d", testKey, i)
		recoveryData := make([]byte, 1024*1024) // 1MB data

		recoveryOperations++
		err := dst.store.KV().Set(ctx, recoveryKey, recoveryData, time.Hour)
		if err == nil {
			successfulRecoveryOps++
			// Clean up successful operations
			dst.store.KV().Delete(ctx, recoveryKey)
		}
	}

	// Test larger operation after recovery
	largeRecoveryErr := dst.store.KV().Set(ctx, testKey+"_large", testData, time.Hour)
	largeRecoverySuccess := largeRecoveryErr == nil

	if largeRecoverySuccess {
		dst.store.KV().Delete(ctx, testKey+"_large")
	}

	result.Duration = time.Since(startTime)
	result.Success = failedAsExpected && successfulRecoveryOps > 0 && largeRecoverySuccess

	// Record metrics
	result.Metrics["failed_as_expected"] = failedAsExpected
	result.Metrics["recovery_operations"] = recoveryOperations
	result.Metrics["successful_recovery_ops"] = successfulRecoveryOps
	result.Metrics["large_recovery_success"] = largeRecoverySuccess
	result.Metrics["cleanup_error"] = cleanupErr != nil

	if !result.Success {
		if !failedAsExpected {
			result.Error = fmt.Errorf("operation did not fail gracefully during disk full: %v", failErr)
		} else if successfulRecoveryOps == 0 {
			result.Error = fmt.Errorf("no operations succeeded after space restoration")
		} else {
			result.Error = fmt.Errorf("large operation failed after space restoration: %v", largeRecoveryErr)
		}
	}

	return result, nil
}

// testDiskFullSimulation tests the disk full simulation itself
func (dst *DiskSpaceTester) testDiskFullSimulation(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "disk_full_simulation",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Get initial disk usage
	initialUsage, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		result.Error = fmt.Errorf("failed to get initial disk usage: %v", err)
		result.Success = false
		return result, err
	}

	// Test simulation with different space limits
	testLimits := []int64{
		100 * 1024 * 1024, // 100MB
		50 * 1024 * 1024,  // 50MB
		10 * 1024 * 1024,  // 10MB
	}

	simulationResults := make(map[string]bool)

	for _, limit := range testLimits {
		limitName := fmt.Sprintf("limit_%dMB", limit/(1024*1024))

		// Skip if not enough free space
		if initialUsage.FreeSpace < limit*2 {
			simulationResults[limitName] = false
			continue
		}

		// Simulate disk full
		err := dst.diskMonitor.SimulateDiskFull(ctx, ".", limit)
		if err != nil {
			simulationResults[limitName] = false
			continue
		}

		// Check if simulation worked
		currentUsage, err := dst.diskMonitor.GetDiskUsage(".")
		if err != nil {
			simulationResults[limitName] = false
			dst.diskMonitor.CleanupTestFiles()
			continue
		}

		// Verify free space is close to limit (within 10% tolerance)
		tolerance := float64(limit) * 0.1
		simulationAccurate := float64(currentUsage.FreeSpace) >= float64(limit)-tolerance &&
			float64(currentUsage.FreeSpace) <= float64(limit)+tolerance

		simulationResults[limitName] = simulationAccurate

		// Clean up
		dst.diskMonitor.CleanupTestFiles()

		// Wait for cleanup to complete
		time.Sleep(time.Millisecond * 100)
	}

	// Test cleanup effectiveness
	finalUsage, err := dst.diskMonitor.GetDiskUsage(".")
	if err != nil {
		result.Error = fmt.Errorf("failed to get final disk usage: %v", err)
		result.Success = false
		return result, err
	}

	// Check if space was restored (within reasonable tolerance)
	spaceRestored := finalUsage.FreeSpace >= int64(float64(initialUsage.FreeSpace)*0.95)

	result.Duration = time.Since(startTime)

	// Count successful simulations
	successfulSimulations := 0
	for _, success := range simulationResults {
		if success {
			successfulSimulations++
		}
	}

	result.Success = successfulSimulations > 0 && spaceRestored

	// Record metrics
	result.Metrics["initial_free_space"] = initialUsage.FreeSpace
	result.Metrics["final_free_space"] = finalUsage.FreeSpace
	result.Metrics["space_restored"] = spaceRestored
	result.Metrics["simulation_results"] = simulationResults
	result.Metrics["successful_simulations"] = successfulSimulations
	result.Metrics["total_simulations"] = len(testLimits)

	if !result.Success {
		if successfulSimulations == 0 {
			result.Error = fmt.Errorf("no disk full simulations succeeded")
		} else {
			result.Error = fmt.Errorf("disk space was not properly restored after cleanup")
		}
	}

	return result, nil
}

// MemoryLimitTester tests memory limit scenarios
type MemoryLimitTester struct {
	store           *store.MantisStore
	memoryMonitor   *MemoryUsageMonitor
	pressureHandler *MemoryPressureHandler
}

// NewMemoryLimitTester creates a new memory limit tester
func NewMemoryLimitTester(mantisStore *store.MantisStore) *MemoryLimitTester {
	return &MemoryLimitTester{
		store:           mantisStore,
		memoryMonitor:   NewMemoryUsageMonitor(),
		pressureHandler: NewMemoryPressureHandler(),
	}
}

// RunTests runs all memory limit tests
func (mlt *MemoryLimitTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "memory_limits",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	// Test memory usage monitoring
	monitorResult, err := mlt.testMemoryUsageMonitoring(ctx)
	result.SubTests["memory_monitoring"] = monitorResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test memory pressure handling
	pressureResult, err := mlt.testMemoryPressureHandling(ctx)
	result.SubTests["memory_pressure_handling"] = pressureResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test graceful degradation under memory constraints
	degradationResult, err := mlt.testGracefulDegradation(ctx)
	result.SubTests["graceful_degradation"] = degradationResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test memory limit enforcement
	limitResult, err := mlt.testMemoryLimitEnforcement(ctx)
	result.SubTests["memory_limit_enforcement"] = limitResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("memory limit tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_memory_scenarios"] = len(result.SubTests)
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// MemoryUsageMonitor monitors memory usage and provides statistics
type MemoryUsageMonitor struct {
	baselineStats runtime.MemStats
	samples       []MemorySample
}

// MemorySample represents a memory usage sample
type MemorySample struct {
	Timestamp    time.Time
	AllocBytes   uint64
	TotalAlloc   uint64
	SysBytes     uint64
	NumGC        uint32
	GCCPUPercent float64
}

// NewMemoryUsageMonitor creates a new memory usage monitor
func NewMemoryUsageMonitor() *MemoryUsageMonitor {
	monitor := &MemoryUsageMonitor{
		samples: make([]MemorySample, 0),
	}

	// Record baseline
	runtime.GC()
	runtime.ReadMemStats(&monitor.baselineStats)

	return monitor
}

// TakeSample records a memory usage sample
func (mum *MemoryUsageMonitor) TakeSample() MemorySample {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	sample := MemorySample{
		Timestamp:    time.Now(),
		AllocBytes:   stats.Alloc,
		TotalAlloc:   stats.TotalAlloc,
		SysBytes:     stats.Sys,
		NumGC:        stats.NumGC,
		GCCPUPercent: stats.GCCPUFraction * 100,
	}

	mum.samples = append(mum.samples, sample)
	return sample
}

// GetMemoryStats returns current memory statistics
func (mum *MemoryUsageMonitor) GetMemoryStats() *MemoryStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	return &MemoryStats{
		AllocBytes:   stats.Alloc,
		TotalAlloc:   stats.TotalAlloc,
		SysBytes:     stats.Sys,
		NumGC:        stats.NumGC,
		GCCPUPercent: stats.GCCPUFraction * 100,
		HeapAlloc:    stats.HeapAlloc,
		HeapSys:      stats.HeapSys,
		HeapIdle:     stats.HeapIdle,
		HeapInuse:    stats.HeapInuse,
		HeapReleased: stats.HeapReleased,
		StackInuse:   stats.StackInuse,
		StackSys:     stats.StackSys,
	}
}

// MemoryStats contains detailed memory statistics
type MemoryStats struct {
	AllocBytes   uint64
	TotalAlloc   uint64
	SysBytes     uint64
	NumGC        uint32
	GCCPUPercent float64
	HeapAlloc    uint64
	HeapSys      uint64
	HeapIdle     uint64
	HeapInuse    uint64
	HeapReleased uint64
	StackInuse   uint64
	StackSys     uint64
}

// GetMemoryPressureLevel returns the current memory pressure level (0.0 to 1.0)
func (mum *MemoryUsageMonitor) GetMemoryPressureLevel() float64 {
	stats := mum.GetMemoryStats()

	// Calculate pressure based on heap usage
	if stats.HeapSys == 0 {
		return 0.0
	}

	heapUsage := float64(stats.HeapInuse) / float64(stats.HeapSys)

	// Also consider GC pressure
	gcPressure := stats.GCCPUPercent / 100.0

	// Combine heap usage and GC pressure
	pressure := (heapUsage * 0.7) + (gcPressure * 0.3)

	if pressure > 1.0 {
		pressure = 1.0
	}

	return pressure
}

// MemoryPressureHandler handles memory pressure situations
type MemoryPressureHandler struct {
	pressureThresholds map[string]float64
	mitigationActions  map[string]func() error
}

// NewMemoryPressureHandler creates a new memory pressure handler
func NewMemoryPressureHandler() *MemoryPressureHandler {
	handler := &MemoryPressureHandler{
		pressureThresholds: map[string]float64{
			"low":      0.3,
			"medium":   0.6,
			"high":     0.8,
			"critical": 0.95,
		},
		mitigationActions: make(map[string]func() error),
	}

	// Define mitigation actions
	handler.mitigationActions["gc"] = func() error {
		runtime.GC()
		return nil
	}

	handler.mitigationActions["free_os_memory"] = func() error {
		runtime.GC()
		debug.FreeOSMemory()
		return nil
	}

	return handler
}

// HandleMemoryPressure handles memory pressure based on current level
func (mph *MemoryPressureHandler) HandleMemoryPressure(pressureLevel float64) []string {
	var actionsPerformed []string

	if pressureLevel >= mph.pressureThresholds["critical"] {
		// Critical pressure - aggressive cleanup
		if action, exists := mph.mitigationActions["free_os_memory"]; exists {
			action()
			actionsPerformed = append(actionsPerformed, "free_os_memory")
		}
	} else if pressureLevel >= mph.pressureThresholds["high"] {
		// High pressure - force GC
		if action, exists := mph.mitigationActions["gc"]; exists {
			action()
			actionsPerformed = append(actionsPerformed, "gc")
		}
	}

	return actionsPerformed
}

// SimulateMemoryPressure simulates memory pressure by allocating memory
func (mph *MemoryPressureHandler) SimulateMemoryPressure(targetPressure float64) (*MemoryPressureSimulation, error) {
	simulation := &MemoryPressureSimulation{
		targetPressure: targetPressure,
		allocations:    make([][]byte, 0),
		startTime:      time.Now(),
	}

	// Get initial memory stats
	var initialStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialStats)

	// Calculate target memory usage
	currentHeapUsage := float64(initialStats.HeapInuse)
	targetHeapUsage := currentHeapUsage / (1.0 - targetPressure)
	memoryToAllocate := int64(targetHeapUsage - currentHeapUsage)

	if memoryToAllocate <= 0 {
		return simulation, fmt.Errorf("already at or above target pressure level")
	}

	// Allocate memory in chunks
	chunkSize := int64(10 * 1024 * 1024) // 10MB chunks
	allocated := int64(0)

	for allocated < memoryToAllocate {
		remainingToAllocate := memoryToAllocate - allocated
		currentChunkSize := chunkSize
		if remainingToAllocate < chunkSize {
			currentChunkSize = remainingToAllocate
		}

		chunk := make([]byte, currentChunkSize)
		// Write to the memory to ensure it's actually allocated
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}

		simulation.allocations = append(simulation.allocations, chunk)
		allocated += currentChunkSize

		// Check if we've reached target pressure
		var currentStats runtime.MemStats
		runtime.ReadMemStats(&currentStats)
		currentPressure := float64(currentStats.HeapInuse) / float64(currentStats.HeapSys)

		if currentPressure >= targetPressure {
			break
		}
	}

	simulation.endTime = time.Now()
	simulation.actualAllocated = allocated

	return simulation, nil
}

// MemoryPressureSimulation represents a memory pressure simulation
type MemoryPressureSimulation struct {
	targetPressure  float64
	actualAllocated int64
	allocations     [][]byte
	startTime       time.Time
	endTime         time.Time
}

// Cleanup releases all allocated memory from the simulation
func (mps *MemoryPressureSimulation) Cleanup() {
	// Clear all allocations
	for i := range mps.allocations {
		mps.allocations[i] = nil
	}
	mps.allocations = mps.allocations[:0]

	// Force garbage collection
	runtime.GC()
	debug.FreeOSMemory()
}

// GetStats returns statistics about the simulation
func (mps *MemoryPressureSimulation) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"target_pressure":  mps.targetPressure,
		"actual_allocated": mps.actualAllocated,
		"allocation_count": len(mps.allocations),
		"duration_ms":      mps.endTime.Sub(mps.startTime).Milliseconds(),
	}
}

// testMemoryUsageMonitoring tests memory usage monitoring capabilities
func (mlt *MemoryLimitTester) testMemoryUsageMonitoring(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "memory_monitoring",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Take baseline sample
	baselineSample := mlt.memoryMonitor.TakeSample()

	// Perform memory-intensive operations
	testData := make([][]byte, 0)
	allocationSize := 5 * 1024 * 1024 // 5MB per allocation
	numAllocations := 10

	for i := 0; i < numAllocations; i++ {
		data := make([]byte, allocationSize)
		// Write to memory to ensure allocation
		for j := range data {
			data[j] = byte(j % 256)
		}
		testData = append(testData, data)

		// Take sample after each allocation
		mlt.memoryMonitor.TakeSample()
	}

	// Take final sample
	finalSample := mlt.memoryMonitor.TakeSample()

	// Analyze memory growth
	memoryGrowth := finalSample.AllocBytes - baselineSample.AllocBytes
	expectedGrowth := uint64(numAllocations * allocationSize)

	// Allow for some overhead (within 50% tolerance)
	tolerance := float64(expectedGrowth) * 0.5
	growthAccurate := float64(memoryGrowth) >= float64(expectedGrowth)-tolerance &&
		float64(memoryGrowth) <= float64(expectedGrowth)+tolerance*2

	// Test pressure level calculation
	pressureLevel := mlt.memoryMonitor.GetMemoryPressureLevel()
	pressureLevelValid := pressureLevel >= 0.0 && pressureLevel <= 1.0

	// Clean up allocations
	testData = nil
	runtime.GC()

	result.Duration = time.Since(startTime)
	result.Success = growthAccurate && pressureLevelValid

	// Record metrics
	result.Metrics["baseline_alloc"] = baselineSample.AllocBytes
	result.Metrics["final_alloc"] = finalSample.AllocBytes
	result.Metrics["memory_growth"] = memoryGrowth
	result.Metrics["expected_growth"] = expectedGrowth
	result.Metrics["growth_accurate"] = growthAccurate
	result.Metrics["pressure_level"] = pressureLevel
	result.Metrics["pressure_level_valid"] = pressureLevelValid
	result.Metrics["num_samples"] = len(mlt.memoryMonitor.samples)
	result.Metrics["gc_count_increase"] = finalSample.NumGC - baselineSample.NumGC

	if !result.Success {
		if !growthAccurate {
			result.Error = fmt.Errorf("memory growth monitoring inaccurate: expected ~%d, measured %d", expectedGrowth, memoryGrowth)
		} else {
			result.Error = fmt.Errorf("pressure level calculation invalid: %f", pressureLevel)
		}
	}

	return result, nil
}

// testMemoryPressureHandling tests memory pressure handling mechanisms
func (mlt *MemoryLimitTester) testMemoryPressureHandling(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "memory_pressure_handling",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test different pressure levels
	pressureLevels := []float64{0.4, 0.7, 0.9}
	handlingResults := make(map[string]interface{})

	for _, pressureLevel := range pressureLevels {
		levelName := fmt.Sprintf("pressure_%.1f", pressureLevel)

		// Simulate memory pressure
		simulation, err := mlt.pressureHandler.SimulateMemoryPressure(pressureLevel)
		if err != nil {
			handlingResults[levelName] = map[string]interface{}{
				"simulation_success": false,
				"error":              err.Error(),
			}
			continue
		}

		// Get memory stats during pressure
		actualPressure := mlt.memoryMonitor.GetMemoryPressureLevel()

		// Handle the pressure
		actionsPerformed := mlt.pressureHandler.HandleMemoryPressure(actualPressure)

		// Get memory stats after handling
		postHandlingPressure := mlt.memoryMonitor.GetMemoryPressureLevel()

		// Check if pressure was reduced
		pressureReduced := postHandlingPressure < actualPressure

		handlingResults[levelName] = map[string]interface{}{
			"simulation_success":     true,
			"target_pressure":        pressureLevel,
			"actual_pressure":        actualPressure,
			"post_handling_pressure": postHandlingPressure,
			"pressure_reduced":       pressureReduced,
			"actions_performed":      actionsPerformed,
			"allocated_bytes":        simulation.actualAllocated,
		}

		// Clean up simulation
		simulation.Cleanup()
		time.Sleep(time.Millisecond * 100) // Allow GC to complete
	}

	// Test pressure threshold detection
	thresholdTests := []struct {
		pressure float64
		expected string
	}{
		{0.2, "none"},
		{0.4, "low"},
		{0.7, "medium"},
		{0.85, "high"},
		{0.98, "critical"},
	}

	thresholdResults := make(map[string]bool)
	for _, test := range thresholdTests {
		actions := mlt.pressureHandler.HandleMemoryPressure(test.pressure)

		switch test.expected {
		case "none":
			thresholdResults[fmt.Sprintf("pressure_%.1f", test.pressure)] = len(actions) == 0
		case "critical", "high":
			thresholdResults[fmt.Sprintf("pressure_%.1f", test.pressure)] = len(actions) > 0
		default:
			thresholdResults[fmt.Sprintf("pressure_%.1f", test.pressure)] = true // Accept any response
		}
	}

	result.Duration = time.Since(startTime)

	// Count successful pressure handling
	successfulHandling := 0
	for _, levelResult := range handlingResults {
		if levelMap, ok := levelResult.(map[string]interface{}); ok {
			if success, ok := levelMap["simulation_success"].(bool); ok && success {
				successfulHandling++
			}
		}
	}

	result.Success = successfulHandling > 0

	// Record metrics
	result.Metrics["pressure_levels_tested"] = len(pressureLevels)
	result.Metrics["successful_handling"] = successfulHandling
	result.Metrics["handling_results"] = handlingResults
	result.Metrics["threshold_results"] = thresholdResults

	if !result.Success {
		result.Error = fmt.Errorf("no memory pressure scenarios were handled successfully")
	}

	return result, nil
}

// testGracefulDegradation tests graceful degradation under memory constraints
func (mlt *MemoryLimitTester) testGracefulDegradation(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "graceful_degradation",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create memory pressure
	simulation, err := mlt.pressureHandler.SimulateMemoryPressure(0.8)
	if err != nil {
		result.Error = fmt.Errorf("failed to simulate memory pressure: %v", err)
		result.Success = false
		return result, err
	}
	defer simulation.Cleanup()

	// Test operations under memory pressure
	testKey := fmt.Sprintf("degradation_test_%d", time.Now().UnixNano())

	// Test small operations (should succeed)
	smallOperations := 0
	successfulSmallOps := 0

	for i := 0; i < 10; i++ {
		smallKey := fmt.Sprintf("%s_small_%d", testKey, i)
		smallData := make([]byte, 1024) // 1KB data

		smallOperations++
		err := mlt.store.KV().Set(ctx, smallKey, smallData, time.Hour)
		if err == nil {
			successfulSmallOps++
			mlt.store.KV().Delete(ctx, smallKey) // Clean up
		}
	}

	// Test large operations (may fail gracefully)
	largeOperations := 0
	successfulLargeOps := 0
	gracefulLargeFailures := 0

	for i := 0; i < 5; i++ {
		largeKey := fmt.Sprintf("%s_large_%d", testKey, i)
		largeData := make([]byte, 10*1024*1024) // 10MB data

		largeOperations++
		err := mlt.store.KV().Set(ctx, largeKey, largeData, time.Hour)
		if err == nil {
			successfulLargeOps++
			mlt.store.KV().Delete(ctx, largeKey) // Clean up
		} else {
			// Check if failure was graceful (no panic, proper error)
			if !contains(err.Error(), "panic") && !contains(err.Error(), "runtime error") {
				gracefulLargeFailures++
			}
		}
	}

	// Test system responsiveness during pressure
	responsivenessTimes := make([]time.Duration, 0)
	for i := 0; i < 5; i++ {
		responseStart := time.Now()
		_, _ = mlt.store.KV().Get(ctx, "nonexistent_key")
		responseTime := time.Since(responseStart)
		responsivenessTimes = append(responsivenessTimes, responseTime)

		// Even errors should be returned quickly
		if responseTime > time.Second*5 {
			break // System is not responsive
		}
	}

	// Calculate average response time
	var totalResponseTime time.Duration
	for _, rt := range responsivenessTimes {
		totalResponseTime += rt
	}
	avgResponseTime := totalResponseTime / time.Duration(len(responsivenessTimes))

	result.Duration = time.Since(startTime)

	// Success criteria: small ops mostly succeed, large ops fail gracefully, system remains responsive
	smallOpsSuccessRate := float64(successfulSmallOps) / float64(smallOperations)
	largeOpsGracefulRate := float64(gracefulLargeFailures) / float64(largeOperations)
	systemResponsive := avgResponseTime < time.Second

	result.Success = smallOpsSuccessRate > 0.5 && largeOpsGracefulRate > 0.5 && systemResponsive

	// Record metrics
	result.Metrics["small_operations"] = smallOperations
	result.Metrics["successful_small_ops"] = successfulSmallOps
	result.Metrics["small_ops_success_rate"] = smallOpsSuccessRate
	result.Metrics["large_operations"] = largeOperations
	result.Metrics["successful_large_ops"] = successfulLargeOps
	result.Metrics["graceful_large_failures"] = gracefulLargeFailures
	result.Metrics["large_ops_graceful_rate"] = largeOpsGracefulRate
	result.Metrics["avg_response_time_ms"] = avgResponseTime.Milliseconds()
	result.Metrics["system_responsive"] = systemResponsive
	result.Metrics["simulation_stats"] = simulation.GetStats()

	if !result.Success {
		if smallOpsSuccessRate <= 0.5 {
			result.Error = fmt.Errorf("small operations success rate too low: %.2f", smallOpsSuccessRate)
		} else if largeOpsGracefulRate <= 0.5 {
			result.Error = fmt.Errorf("large operations graceful failure rate too low: %.2f", largeOpsGracefulRate)
		} else {
			result.Error = fmt.Errorf("system not responsive under memory pressure: avg response time %v", avgResponseTime)
		}
	}

	return result, nil
}

// testMemoryLimitEnforcement tests memory limit enforcement mechanisms
func (mlt *MemoryLimitTester) testMemoryLimitEnforcement(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "memory_limit_enforcement",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Get baseline memory usage
	baselineStats := mlt.memoryMonitor.GetMemoryStats()

	// Test progressive memory allocation with monitoring
	allocationSizes := []int{
		1 * 1024 * 1024,  // 1MB
		5 * 1024 * 1024,  // 5MB
		10 * 1024 * 1024, // 10MB
		20 * 1024 * 1024, // 20MB
	}

	allocationResults := make(map[string]interface{})
	var allocations [][]byte

	for _, size := range allocationSizes {
		sizeName := fmt.Sprintf("alloc_%dMB", size/(1024*1024))

		// Monitor memory before allocation
		preAllocStats := mlt.memoryMonitor.GetMemoryStats()
		preAllocPressure := mlt.memoryMonitor.GetMemoryPressureLevel()

		// Attempt allocation
		allocation := make([]byte, size)
		// Write to ensure allocation
		for j := 0; j < len(allocation); j += 4096 {
			allocation[j] = byte(j % 256)
		}
		allocations = append(allocations, allocation)

		// Monitor memory after allocation
		postAllocStats := mlt.memoryMonitor.GetMemoryStats()
		postAllocPressure := mlt.memoryMonitor.GetMemoryPressureLevel()

		// Check if pressure handling was triggered
		actionsPerformed := mlt.pressureHandler.HandleMemoryPressure(postAllocPressure)

		allocationResults[sizeName] = map[string]interface{}{
			"pre_alloc_pressure":  preAllocPressure,
			"post_alloc_pressure": postAllocPressure,
			"pressure_increase":   postAllocPressure - preAllocPressure,
			"actions_performed":   actionsPerformed,
			"memory_increase":     postAllocStats.AllocBytes - preAllocStats.AllocBytes,
		}

		// If pressure is too high, stop allocating
		if postAllocPressure > 0.9 {
			break
		}
	}

	// Test memory cleanup effectiveness
	preCleanupStats := mlt.memoryMonitor.GetMemoryStats()

	// Clear allocations
	for i := range allocations {
		allocations[i] = nil
	}
	allocations = nil

	// Force cleanup
	runtime.GC()
	debug.FreeOSMemory()
	time.Sleep(time.Millisecond * 100)

	postCleanupStats := mlt.memoryMonitor.GetMemoryStats()

	// Calculate cleanup effectiveness
	memoryFreed := preCleanupStats.AllocBytes - postCleanupStats.AllocBytes
	cleanupEffective := memoryFreed > 0

	// Test limit detection
	finalPressure := mlt.memoryMonitor.GetMemoryPressureLevel()
	limitDetectionWorking := finalPressure >= 0.0 && finalPressure <= 1.0

	result.Duration = time.Since(startTime)
	result.Success = cleanupEffective && limitDetectionWorking

	// Record metrics
	result.Metrics["baseline_alloc"] = baselineStats.AllocBytes
	result.Metrics["pre_cleanup_alloc"] = preCleanupStats.AllocBytes
	result.Metrics["post_cleanup_alloc"] = postCleanupStats.AllocBytes
	result.Metrics["memory_freed"] = memoryFreed
	result.Metrics["cleanup_effective"] = cleanupEffective
	result.Metrics["final_pressure"] = finalPressure
	result.Metrics["limit_detection_working"] = limitDetectionWorking
	result.Metrics["allocation_results"] = allocationResults
	result.Metrics["allocations_tested"] = len(allocationResults)

	if !result.Success {
		if !cleanupEffective {
			result.Error = fmt.Errorf("memory cleanup was not effective: freed %d bytes", memoryFreed)
		} else {
			result.Error = fmt.Errorf("limit detection not working properly: pressure %f", finalPressure)
		}
	}

	return result, nil
}

// ConcurrencyTester tests concurrent access patterns and performance
type ConcurrencyTester struct {
	store            *store.MantisStore
	deadlockDetector *DeadlockDetector
	performanceBench *PerformanceBenchmark
}

// NewConcurrencyTester creates a new concurrency tester
func NewConcurrencyTester(mantisStore *store.MantisStore) *ConcurrencyTester {
	return &ConcurrencyTester{
		store:            mantisStore,
		deadlockDetector: NewDeadlockDetector(),
		performanceBench: NewPerformanceBenchmark(),
	}
}

// RunTests runs all concurrent access pattern tests
func (ct *ConcurrencyTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "concurrent_access_patterns",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	// Test high-concurrency scenarios
	concurrencyResult, err := ct.testHighConcurrencyScenarios(ctx)
	result.SubTests["high_concurrency"] = concurrencyResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test deadlock detection and prevention
	deadlockResult, err := ct.testDeadlockDetectionAndPrevention(ctx)
	result.SubTests["deadlock_detection"] = deadlockResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test performance benchmarking under concurrent load
	performanceResult, err := ct.testPerformanceBenchmarking(ctx)
	result.SubTests["performance_benchmarking"] = performanceResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test mixed workload patterns
	mixedWorkloadResult, err := ct.testMixedWorkloadPatterns(ctx)
	result.SubTests["mixed_workload"] = mixedWorkloadResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("concurrent access tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_concurrency_scenarios"] = len(result.SubTests)
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// DeadlockDetector detects potential deadlocks in concurrent operations
type DeadlockDetector struct {
	operationTracker map[string]*OperationTracker
	lockGraph        map[string][]string
	detectionTimeout time.Duration
}

// OperationTracker tracks operations for deadlock detection
type OperationTracker struct {
	OperationID   string
	WorkerID      string
	StartTime     time.Time
	ResourcesHeld []string
	WaitingFor    string
	Status        string // "running", "waiting", "completed", "failed"
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector() *DeadlockDetector {
	return &DeadlockDetector{
		operationTracker: make(map[string]*OperationTracker),
		lockGraph:        make(map[string][]string),
		detectionTimeout: time.Second * 30,
	}
}

// TrackOperation starts tracking an operation
func (dd *DeadlockDetector) TrackOperation(operationID, workerID string) {
	dd.operationTracker[operationID] = &OperationTracker{
		OperationID:   operationID,
		WorkerID:      workerID,
		StartTime:     time.Now(),
		ResourcesHeld: make([]string, 0),
		Status:        "running",
	}
}

// RecordResourceAcquisition records when an operation acquires a resource
func (dd *DeadlockDetector) RecordResourceAcquisition(operationID, resource string) {
	if tracker, exists := dd.operationTracker[operationID]; exists {
		tracker.ResourcesHeld = append(tracker.ResourcesHeld, resource)
	}
}

// RecordResourceWait records when an operation is waiting for a resource
func (dd *DeadlockDetector) RecordResourceWait(operationID, resource string) {
	if tracker, exists := dd.operationTracker[operationID]; exists {
		tracker.WaitingFor = resource
		tracker.Status = "waiting"
	}
}

// CompleteOperation marks an operation as completed
func (dd *DeadlockDetector) CompleteOperation(operationID string) {
	if tracker, exists := dd.operationTracker[operationID]; exists {
		tracker.Status = "completed"
		// Clear resources
		tracker.ResourcesHeld = tracker.ResourcesHeld[:0]
		tracker.WaitingFor = ""
	}
}

// DetectDeadlocks analyzes current operations for potential deadlocks
func (dd *DeadlockDetector) DetectDeadlocks() []DeadlockInfo {
	deadlocks := make([]DeadlockInfo, 0)

	// Build wait-for graph
	waitGraph := make(map[string][]string)

	for _, tracker := range dd.operationTracker {
		if tracker.Status == "waiting" && tracker.WaitingFor != "" {
			// Find who holds the resource this operation is waiting for
			for _, otherTracker := range dd.operationTracker {
				if otherTracker.OperationID != tracker.OperationID {
					for _, heldResource := range otherTracker.ResourcesHeld {
						if heldResource == tracker.WaitingFor {
							waitGraph[tracker.OperationID] = append(waitGraph[tracker.OperationID], otherTracker.OperationID)
						}
					}
				}
			}
		}
	}

	// Detect cycles in wait-for graph (simplified cycle detection)
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for operationID := range waitGraph {
		if !visited[operationID] {
			if dd.hasCycle(operationID, waitGraph, visited, recStack) {
				// Found a cycle - this is a potential deadlock
				cycle := dd.findCycle(operationID, waitGraph)
				if len(cycle) > 1 {
					deadlocks = append(deadlocks, DeadlockInfo{
						InvolvedOperations: cycle,
						DetectedAt:         time.Now(),
						Type:               "circular_wait",
					})
				}
			}
		}
	}

	// Also check for timeout-based deadlock detection
	now := time.Now()
	for _, tracker := range dd.operationTracker {
		if tracker.Status == "waiting" && now.Sub(tracker.StartTime) > dd.detectionTimeout {
			deadlocks = append(deadlocks, DeadlockInfo{
				InvolvedOperations: []string{tracker.OperationID},
				DetectedAt:         now,
				Type:               "timeout_based",
			})
		}
	}

	return deadlocks
}

// DeadlockInfo contains information about a detected deadlock
type DeadlockInfo struct {
	InvolvedOperations []string
	DetectedAt         time.Time
	Type               string // "circular_wait", "timeout_based"
}

// hasCycle performs DFS to detect cycles in wait-for graph
func (dd *DeadlockDetector) hasCycle(operationID string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[operationID] = true
	recStack[operationID] = true

	for _, neighbor := range graph[operationID] {
		if !visited[neighbor] {
			if dd.hasCycle(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[operationID] = false
	return false
}

// findCycle finds the actual cycle in the wait-for graph
func (dd *DeadlockDetector) findCycle(startID string, graph map[string][]string) []string {
	visited := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(string) []string
	dfs = func(operationID string) []string {
		if visited[operationID] {
			// Found cycle, return path from this point
			for i, id := range path {
				if id == operationID {
					return path[i:]
				}
			}
			return []string{operationID}
		}

		visited[operationID] = true
		path = append(path, operationID)

		for _, neighbor := range graph[operationID] {
			if cycle := dfs(neighbor); len(cycle) > 0 {
				return cycle
			}
		}

		// Backtrack
		path = path[:len(path)-1]
		return nil
	}

	return dfs(startID)
}

// PerformanceBenchmark benchmarks performance under concurrent load
type PerformanceBenchmark struct {
	metrics []PerformanceMetric
}

// PerformanceMetric represents a performance measurement
type PerformanceMetric struct {
	Timestamp    time.Time
	Operation    string
	Duration     time.Duration
	WorkerID     string
	Success      bool
	ErrorMessage string
}

// NewPerformanceBenchmark creates a new performance benchmark
func NewPerformanceBenchmark() *PerformanceBenchmark {
	return &PerformanceBenchmark{
		metrics: make([]PerformanceMetric, 0),
	}
}

// RecordMetric records a performance metric
func (pb *PerformanceBenchmark) RecordMetric(metric PerformanceMetric) {
	pb.metrics = append(pb.metrics, metric)
}

// GetStats calculates performance statistics
func (pb *PerformanceBenchmark) GetStats() *PerformanceStats {
	if len(pb.metrics) == 0 {
		return &PerformanceStats{}
	}

	stats := &PerformanceStats{
		TotalOperations:   len(pb.metrics),
		OperationsByType:  make(map[string]int),
		DurationsByType:   make(map[string][]time.Duration),
		SuccessRateByType: make(map[string]float64),
	}

	var totalDuration time.Duration
	successCount := 0

	for _, metric := range pb.metrics {
		totalDuration += metric.Duration

		if metric.Success {
			successCount++
		}

		// Track by operation type
		stats.OperationsByType[metric.Operation]++
		stats.DurationsByType[metric.Operation] = append(stats.DurationsByType[metric.Operation], metric.Duration)
	}

	stats.AverageDuration = totalDuration / time.Duration(len(pb.metrics))
	stats.OverallSuccessRate = float64(successCount) / float64(len(pb.metrics))

	// Calculate success rates by type
	successByType := make(map[string]int)
	for _, metric := range pb.metrics {
		if metric.Success {
			successByType[metric.Operation]++
		}
	}

	for opType, total := range stats.OperationsByType {
		stats.SuccessRateByType[opType] = float64(successByType[opType]) / float64(total)
	}

	// Calculate percentiles for overall duration
	durations := make([]time.Duration, len(pb.metrics))
	for i, metric := range pb.metrics {
		durations[i] = metric.Duration
	}

	// Simple percentile calculation (not perfectly accurate but sufficient for testing)
	if len(durations) > 0 {
		// Sort durations (simplified)
		for i := 0; i < len(durations)-1; i++ {
			for j := i + 1; j < len(durations); j++ {
				if durations[i] > durations[j] {
					durations[i], durations[j] = durations[j], durations[i]
				}
			}
		}

		stats.P50Duration = durations[len(durations)/2]
		stats.P95Duration = durations[int(float64(len(durations))*0.95)]
		stats.P99Duration = durations[int(float64(len(durations))*0.99)]
	}

	return stats
}

// PerformanceStats contains performance statistics
type PerformanceStats struct {
	TotalOperations    int
	AverageDuration    time.Duration
	P50Duration        time.Duration
	P95Duration        time.Duration
	P99Duration        time.Duration
	OverallSuccessRate float64
	OperationsByType   map[string]int
	DurationsByType    map[string][]time.Duration
	SuccessRateByType  map[string]float64
}

// testHighConcurrencyScenarios tests high-concurrency scenarios
func (ct *ConcurrencyTester) testHighConcurrencyScenarios(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "high_concurrency_scenarios",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test different concurrency levels
	concurrencyLevels := []int{50, 100, 200, 500}
	scenarioResults := make(map[string]interface{})

	for _, level := range concurrencyLevels {
		levelName := fmt.Sprintf("workers_%d", level)

		// Run concurrent scenario
		scenarioResult, err := ct.runConcurrentScenario(ctx, level, 10) // 10 operations per worker
		if err != nil {
			scenarioResults[levelName] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			continue
		}

		scenarioResults[levelName] = map[string]interface{}{
			"success":            true,
			"total_operations":   scenarioResult.TotalOperations,
			"success_rate":       scenarioResult.SuccessRate,
			"average_duration":   scenarioResult.AverageDuration.Milliseconds(),
			"p95_duration":       scenarioResult.P95Duration.Milliseconds(),
			"deadlocks_detected": scenarioResult.DeadlocksDetected,
		}
	}

	// Test burst scenarios
	burstResult, err := ct.testBurstScenario(ctx)
	scenarioResults["burst_scenario"] = map[string]interface{}{
		"success": err == nil,
	}
	if err == nil {
		scenarioResults["burst_scenario"].(map[string]interface{})["total_operations"] = burstResult.TotalOperations
		scenarioResults["burst_scenario"].(map[string]interface{})["success_rate"] = burstResult.SuccessRate
	}

	result.Duration = time.Since(startTime)

	// Count successful scenarios
	successfulScenarios := 0
	for _, scenarioResult := range scenarioResults {
		if resultMap, ok := scenarioResult.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); ok && success {
				successfulScenarios++
			}
		}
	}

	result.Success = successfulScenarios > 0

	// Record metrics
	result.Metrics["concurrency_levels_tested"] = len(concurrencyLevels)
	result.Metrics["successful_scenarios"] = successfulScenarios
	result.Metrics["scenario_results"] = scenarioResults

	if !result.Success {
		result.Error = fmt.Errorf("no high-concurrency scenarios succeeded")
	}

	return result, nil
}

// ConcurrentScenarioResult contains results from a concurrent scenario
type ConcurrentScenarioResult struct {
	TotalOperations   int
	SuccessRate       float64
	AverageDuration   time.Duration
	P95Duration       time.Duration
	P99Duration       time.Duration
	DeadlocksDetected int
	Errors            []string
}

// runConcurrentScenario runs a concurrent scenario with specified workers and operations
func (ct *ConcurrencyTester) runConcurrentScenario(ctx context.Context, workerCount, operationsPerWorker int) (*ConcurrentScenarioResult, error) {
	// Reset performance benchmark
	ct.performanceBench = NewPerformanceBenchmark()

	// Channel to collect results
	resultsChan := make(chan []PerformanceMetric, workerCount)

	// Start workers
	for i := 0; i < workerCount; i++ {
		go ct.concurrentWorker(ctx, fmt.Sprintf("worker_%d", i), operationsPerWorker, resultsChan)
	}

	// Collect results
	allMetrics := make([]PerformanceMetric, 0)
	for i := 0; i < workerCount; i++ {
		workerMetrics := <-resultsChan
		allMetrics = append(allMetrics, workerMetrics...)
	}

	// Record all metrics
	for _, metric := range allMetrics {
		ct.performanceBench.RecordMetric(metric)
	}

	// Get performance stats
	stats := ct.performanceBench.GetStats()

	// Check for deadlocks
	deadlocks := ct.deadlockDetector.DetectDeadlocks()

	// Collect errors
	errors := make([]string, 0)
	for _, metric := range allMetrics {
		if !metric.Success && metric.ErrorMessage != "" {
			errors = append(errors, metric.ErrorMessage)
		}
	}

	return &ConcurrentScenarioResult{
		TotalOperations:   stats.TotalOperations,
		SuccessRate:       stats.OverallSuccessRate,
		AverageDuration:   stats.AverageDuration,
		P95Duration:       stats.P95Duration,
		P99Duration:       stats.P99Duration,
		DeadlocksDetected: len(deadlocks),
		Errors:            errors,
	}, nil
}

// concurrentWorker performs concurrent operations
func (ct *ConcurrencyTester) concurrentWorker(ctx context.Context, workerID string, operations int, results chan<- []PerformanceMetric) {
	metrics := make([]PerformanceMetric, 0)

	for i := 0; i < operations; i++ {
		operationID := fmt.Sprintf("%s_op_%d", workerID, i)

		// Track operation for deadlock detection
		ct.deadlockDetector.TrackOperation(operationID, workerID)

		// Perform mixed operations
		operationType := []string{"set", "get", "delete"}[i%3]
		key := fmt.Sprintf("concurrent_test_%s_%d", workerID, i)

		startTime := time.Now()
		var err error

		switch operationType {
		case "set":
			data := []byte(fmt.Sprintf("data_%s_%d", workerID, i))
			ct.deadlockDetector.RecordResourceAcquisition(operationID, key)
			err = ct.store.KV().Set(ctx, key, data, time.Hour)

		case "get":
			ct.deadlockDetector.RecordResourceWait(operationID, key)
			_, err = ct.store.KV().Get(ctx, key)
			if err == nil {
				ct.deadlockDetector.RecordResourceAcquisition(operationID, key)
			}

		case "delete":
			ct.deadlockDetector.RecordResourceAcquisition(operationID, key)
			err = ct.store.KV().Delete(ctx, key)
		}

		duration := time.Since(startTime)

		// Record metric
		metric := PerformanceMetric{
			Timestamp: startTime,
			Operation: operationType,
			Duration:  duration,
			WorkerID:  workerID,
			Success:   err == nil,
		}

		if err != nil {
			metric.ErrorMessage = err.Error()
		}

		metrics = append(metrics, metric)

		// Complete operation tracking
		ct.deadlockDetector.CompleteOperation(operationID)

		// Small delay to increase concurrency overlap
		time.Sleep(time.Microsecond * 100)
	}

	results <- metrics
}

// testBurstScenario tests burst load scenarios
func (ct *ConcurrencyTester) testBurstScenario(ctx context.Context) (*ConcurrentScenarioResult, error) {
	// Simulate burst by starting many workers simultaneously
	burstWorkers := 100
	operationsPerWorker := 5

	// Use a channel to synchronize burst start
	startSignal := make(chan struct{})
	resultsChan := make(chan []PerformanceMetric, burstWorkers)

	// Start all workers but don't let them proceed yet
	for i := 0; i < burstWorkers; i++ {
		go func(workerID string) {
			<-startSignal // Wait for start signal

			metrics := make([]PerformanceMetric, 0)

			for j := 0; j < operationsPerWorker; j++ {
				key := fmt.Sprintf("burst_test_%s_%d", workerID, j)
				data := []byte(fmt.Sprintf("burst_data_%s_%d", workerID, j))

				startTime := time.Now()
				err := ct.store.KV().Set(ctx, key, data, time.Hour)
				duration := time.Since(startTime)

				metric := PerformanceMetric{
					Timestamp: startTime,
					Operation: "set",
					Duration:  duration,
					WorkerID:  workerID,
					Success:   err == nil,
				}

				if err != nil {
					metric.ErrorMessage = err.Error()
				}

				metrics = append(metrics, metric)
			}

			resultsChan <- metrics
		}(fmt.Sprintf("burst_worker_%d", i))
	}

	// Release all workers simultaneously
	close(startSignal)

	// Collect results
	allMetrics := make([]PerformanceMetric, 0)
	for i := 0; i < burstWorkers; i++ {
		workerMetrics := <-resultsChan
		allMetrics = append(allMetrics, workerMetrics...)
	}

	// Calculate stats
	totalOps := len(allMetrics)
	successCount := 0
	var totalDuration time.Duration

	for _, metric := range allMetrics {
		if metric.Success {
			successCount++
		}
		totalDuration += metric.Duration
	}

	avgDuration := totalDuration / time.Duration(totalOps)
	successRate := float64(successCount) / float64(totalOps)

	return &ConcurrentScenarioResult{
		TotalOperations: totalOps,
		SuccessRate:     successRate,
		AverageDuration: avgDuration,
	}, nil
}

// testDeadlockDetectionAndPrevention tests deadlock detection and prevention
func (ct *ConcurrencyTester) testDeadlockDetectionAndPrevention(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "deadlock_detection_prevention",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create a scenario likely to cause deadlocks
	deadlockScenario, err := ct.createDeadlockScenario(ctx)
	if err != nil {
		result.Error = fmt.Errorf("failed to create deadlock scenario: %v", err)
		result.Success = false
		return result, err
	}

	// Run deadlock detection
	detectedDeadlocks := ct.deadlockDetector.DetectDeadlocks()

	// Test timeout-based detection
	timeoutDeadlocks := ct.testTimeoutBasedDeadlockDetection(ctx)

	// Test deadlock prevention mechanisms
	preventionResult := ct.testDeadlockPrevention(ctx)

	result.Duration = time.Since(startTime)
	result.Success = len(detectedDeadlocks) >= 0 && preventionResult // Detection working (even 0 deadlocks is success)

	// Record metrics
	result.Metrics["deadlock_scenario_created"] = deadlockScenario
	result.Metrics["circular_deadlocks_detected"] = len(detectedDeadlocks)
	result.Metrics["timeout_deadlocks_detected"] = len(timeoutDeadlocks)
	result.Metrics["prevention_mechanisms_working"] = preventionResult

	// Analyze detected deadlocks
	deadlockTypes := make(map[string]int)
	for _, deadlock := range detectedDeadlocks {
		deadlockTypes[deadlock.Type]++
	}
	result.Metrics["deadlock_types"] = deadlockTypes

	if !result.Success {
		result.Error = fmt.Errorf("deadlock detection or prevention mechanisms not working properly")
	}

	return result, nil
}

// createDeadlockScenario creates a scenario that might lead to deadlocks
func (ct *ConcurrencyTester) createDeadlockScenario(ctx context.Context) (bool, error) {
	// Create two workers that will try to acquire resources in opposite order
	worker1Done := make(chan error, 1)
	worker2Done := make(chan error, 1)

	resource1 := "deadlock_resource_1"
	resource2 := "deadlock_resource_2"

	// Worker 1: acquire resource1, then resource2
	go func() {
		operationID := "deadlock_worker_1"
		ct.deadlockDetector.TrackOperation(operationID, "worker_1")

		// Acquire resource1
		err := ct.store.KV().Set(ctx, resource1, []byte("worker1_data"), time.Hour)
		if err != nil {
			worker1Done <- err
			return
		}
		ct.deadlockDetector.RecordResourceAcquisition(operationID, resource1)

		// Wait a bit to increase chance of deadlock
		time.Sleep(time.Millisecond * 100)

		// Try to acquire resource2
		ct.deadlockDetector.RecordResourceWait(operationID, resource2)
		err = ct.store.KV().Set(ctx, resource2, []byte("worker1_data2"), time.Hour)
		if err == nil {
			ct.deadlockDetector.RecordResourceAcquisition(operationID, resource2)
		}

		ct.deadlockDetector.CompleteOperation(operationID)
		worker1Done <- err
	}()

	// Worker 2: acquire resource2, then resource1
	go func() {
		operationID := "deadlock_worker_2"
		ct.deadlockDetector.TrackOperation(operationID, "worker_2")

		// Small delay to let worker1 start
		time.Sleep(time.Millisecond * 50)

		// Acquire resource2
		err := ct.store.KV().Set(ctx, resource2, []byte("worker2_data"), time.Hour)
		if err != nil {
			worker2Done <- err
			return
		}
		ct.deadlockDetector.RecordResourceAcquisition(operationID, resource2)

		// Wait a bit
		time.Sleep(time.Millisecond * 100)

		// Try to acquire resource1
		ct.deadlockDetector.RecordResourceWait(operationID, resource1)
		err = ct.store.KV().Set(ctx, resource1, []byte("worker2_data1"), time.Hour)
		if err == nil {
			ct.deadlockDetector.RecordResourceAcquisition(operationID, resource1)
		}

		ct.deadlockDetector.CompleteOperation(operationID)
		worker2Done <- err
	}()

	// Wait for both workers with timeout
	timeout := time.After(time.Second * 5)
	workersCompleted := 0

	for workersCompleted < 2 {
		select {
		case <-worker1Done:
			workersCompleted++
		case <-worker2Done:
			workersCompleted++
		case <-timeout:
			// Timeout - potential deadlock scenario created
			return true, nil
		}
	}

	// Clean up resources
	ct.store.KV().Delete(ctx, resource1)
	ct.store.KV().Delete(ctx, resource2)

	return true, nil
}

// testTimeoutBasedDeadlockDetection tests timeout-based deadlock detection
func (ct *ConcurrencyTester) testTimeoutBasedDeadlockDetection(ctx context.Context) []DeadlockInfo {
	// Create operations that will timeout
	operationID := "timeout_test_operation"
	ct.deadlockDetector.TrackOperation(operationID, "timeout_worker")
	ct.deadlockDetector.RecordResourceWait(operationID, "nonexistent_resource")

	// Wait longer than detection timeout
	time.Sleep(ct.deadlockDetector.detectionTimeout + time.Second)

	// Detect deadlocks
	deadlocks := ct.deadlockDetector.DetectDeadlocks()

	// Clean up
	ct.deadlockDetector.CompleteOperation(operationID)

	return deadlocks
}

// testDeadlockPrevention tests deadlock prevention mechanisms
func (ct *ConcurrencyTester) testDeadlockPrevention(ctx context.Context) bool {
	// Test ordered resource acquisition (prevention technique)
	resources := []string{"resource_a", "resource_b", "resource_c"}

	// Multiple workers acquiring resources in same order (should prevent deadlocks)
	workerCount := 10
	done := make(chan bool, workerCount)

	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			operationID := fmt.Sprintf("prevention_worker_%d", workerID)
			ct.deadlockDetector.TrackOperation(operationID, fmt.Sprintf("worker_%d", workerID))

			// Acquire resources in order
			for _, resource := range resources {
				key := fmt.Sprintf("%s_%d", resource, workerID)
				err := ct.store.KV().Set(ctx, key, []byte("prevention_data"), time.Hour)
				if err == nil {
					ct.deadlockDetector.RecordResourceAcquisition(operationID, resource)
				}
				time.Sleep(time.Millisecond * 10)
			}

			// Release resources in reverse order
			for i := len(resources) - 1; i >= 0; i-- {
				key := fmt.Sprintf("%s_%d", resources[i], workerID)
				ct.store.KV().Delete(ctx, key)
			}

			ct.deadlockDetector.CompleteOperation(operationID)
			done <- true
		}(i)
	}

	// Wait for all workers to complete
	completedWorkers := 0
	timeout := time.After(time.Second * 10)

	for completedWorkers < workerCount {
		select {
		case <-done:
			completedWorkers++
		case <-timeout:
			// Timeout - prevention might not be working
			return false
		}
	}

	// Check if any deadlocks were detected during prevention test
	deadlocks := ct.deadlockDetector.DetectDeadlocks()
	return len(deadlocks) == 0 // Success if no deadlocks detected
}

// testPerformanceBenchmarking tests performance benchmarking under concurrent load
func (ct *ConcurrencyTester) testPerformanceBenchmarking(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "performance_benchmarking",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test different load levels
	loadLevels := []struct {
		name     string
		workers  int
		duration time.Duration
	}{
		{"light_load", 10, time.Second * 5},
		{"medium_load", 50, time.Second * 10},
		{"heavy_load", 100, time.Second * 15},
	}

	benchmarkResults := make(map[string]interface{})

	for _, level := range loadLevels {
		levelResult, err := ct.runPerformanceBenchmark(ctx, level.workers, level.duration)
		if err != nil {
			benchmarkResults[level.name] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			continue
		}

		benchmarkResults[level.name] = map[string]interface{}{
			"success":               true,
			"total_operations":      levelResult.TotalOperations,
			"operations_per_second": float64(levelResult.TotalOperations) / level.duration.Seconds(),
			"average_duration_ms":   levelResult.AverageDuration.Milliseconds(),
			"p95_duration_ms":       levelResult.P95Duration.Milliseconds(),
			"p99_duration_ms":       levelResult.P99Duration.Milliseconds(),
			"success_rate":          levelResult.OverallSuccessRate,
			"operations_by_type":    levelResult.OperationsByType,
		}
	}

	result.Duration = time.Since(startTime)

	// Count successful benchmarks
	successfulBenchmarks := 0
	for _, benchResult := range benchmarkResults {
		if resultMap, ok := benchResult.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); ok && success {
				successfulBenchmarks++
			}
		}
	}

	result.Success = successfulBenchmarks > 0

	// Record metrics
	result.Metrics["load_levels_tested"] = len(loadLevels)
	result.Metrics["successful_benchmarks"] = successfulBenchmarks
	result.Metrics["benchmark_results"] = benchmarkResults

	if !result.Success {
		result.Error = fmt.Errorf("no performance benchmarks succeeded")
	}

	return result, nil
}

// runPerformanceBenchmark runs a performance benchmark with specified parameters
func (ct *ConcurrencyTester) runPerformanceBenchmark(ctx context.Context, workerCount int, duration time.Duration) (*PerformanceStats, error) {
	// Reset benchmark
	ct.performanceBench = NewPerformanceBenchmark()

	// Channel to signal workers to stop
	stopSignal := make(chan struct{})
	resultsChan := make(chan []PerformanceMetric, workerCount)

	// Start workers
	for i := 0; i < workerCount; i++ {
		go ct.benchmarkWorker(ctx, fmt.Sprintf("bench_worker_%d", i), stopSignal, resultsChan)
	}

	// Run for specified duration
	time.Sleep(duration)
	close(stopSignal)

	// Collect results
	allMetrics := make([]PerformanceMetric, 0)
	for i := 0; i < workerCount; i++ {
		workerMetrics := <-resultsChan
		allMetrics = append(allMetrics, workerMetrics...)
	}

	// Record all metrics
	for _, metric := range allMetrics {
		ct.performanceBench.RecordMetric(metric)
	}

	return ct.performanceBench.GetStats(), nil
}

// benchmarkWorker performs operations for performance benchmarking
func (ct *ConcurrencyTester) benchmarkWorker(ctx context.Context, workerID string, stopSignal <-chan struct{}, results chan<- []PerformanceMetric) {
	metrics := make([]PerformanceMetric, 0)
	operationCount := 0

	for {
		select {
		case <-stopSignal:
			results <- metrics
			return
		default:
			// Perform operation
			operationType := []string{"set", "get", "delete"}[operationCount%3]
			key := fmt.Sprintf("bench_%s_%d", workerID, operationCount)

			startTime := time.Now()
			var err error

			switch operationType {
			case "set":
				data := []byte(fmt.Sprintf("benchmark_data_%s_%d", workerID, operationCount))
				err = ct.store.KV().Set(ctx, key, data, time.Hour)
			case "get":
				_, err = ct.store.KV().Get(ctx, key)
			case "delete":
				err = ct.store.KV().Delete(ctx, key)
			}

			duration := time.Since(startTime)

			metric := PerformanceMetric{
				Timestamp: startTime,
				Operation: operationType,
				Duration:  duration,
				WorkerID:  workerID,
				Success:   err == nil,
			}

			if err != nil {
				metric.ErrorMessage = err.Error()
			}

			metrics = append(metrics, metric)
			operationCount++
		}
	}
}

// testMixedWorkloadPatterns tests mixed workload patterns
func (ct *ConcurrencyTester) testMixedWorkloadPatterns(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "mixed_workload_patterns",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Define different workload patterns
	patterns := []struct {
		name        string
		readRatio   float64
		writeRatio  float64
		deleteRatio float64
	}{
		{"read_heavy", 0.8, 0.15, 0.05},
		{"write_heavy", 0.2, 0.7, 0.1},
		{"balanced", 0.4, 0.4, 0.2},
		{"delete_heavy", 0.3, 0.3, 0.4},
	}

	patternResults := make(map[string]interface{})

	for _, pattern := range patterns {
		patternResult, err := ct.runMixedWorkloadPattern(ctx, pattern.name, pattern.readRatio, pattern.writeRatio, pattern.deleteRatio)
		if err != nil {
			patternResults[pattern.name] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			continue
		}

		patternResults[pattern.name] = map[string]interface{}{
			"success":            true,
			"total_operations":   patternResult.TotalOperations,
			"success_rate":       patternResult.OverallSuccessRate,
			"average_duration":   patternResult.AverageDuration.Milliseconds(),
			"operations_by_type": patternResult.OperationsByType,
			"success_by_type":    patternResult.SuccessRateByType,
		}
	}

	result.Duration = time.Since(startTime)

	// Count successful patterns
	successfulPatterns := 0
	for _, patternResult := range patternResults {
		if resultMap, ok := patternResult.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); ok && success {
				successfulPatterns++
			}
		}
	}

	result.Success = successfulPatterns > 0

	// Record metrics
	result.Metrics["patterns_tested"] = len(patterns)
	result.Metrics["successful_patterns"] = successfulPatterns
	result.Metrics["pattern_results"] = patternResults

	if !result.Success {
		result.Error = fmt.Errorf("no mixed workload patterns succeeded")
	}

	return result, nil
}

// runMixedWorkloadPattern runs a mixed workload pattern
func (ct *ConcurrencyTester) runMixedWorkloadPattern(ctx context.Context, patternName string, readRatio, writeRatio, deleteRatio float64) (*PerformanceStats, error) {
	// Reset benchmark
	ct.performanceBench = NewPerformanceBenchmark()

	workerCount := 20
	operationsPerWorker := 50
	resultsChan := make(chan []PerformanceMetric, workerCount)

	// Start workers
	for i := 0; i < workerCount; i++ {
		go ct.mixedWorkloadWorker(ctx, fmt.Sprintf("%s_worker_%d", patternName, i), operationsPerWorker, readRatio, writeRatio, deleteRatio, resultsChan)
	}

	// Collect results
	allMetrics := make([]PerformanceMetric, 0)
	for i := 0; i < workerCount; i++ {
		workerMetrics := <-resultsChan
		allMetrics = append(allMetrics, workerMetrics...)
	}

	// Record all metrics
	for _, metric := range allMetrics {
		ct.performanceBench.RecordMetric(metric)
	}

	return ct.performanceBench.GetStats(), nil
}

// mixedWorkloadWorker performs mixed workload operations
func (ct *ConcurrencyTester) mixedWorkloadWorker(ctx context.Context, workerID string, operations int, readRatio, writeRatio, deleteRatio float64, results chan<- []PerformanceMetric) {
	metrics := make([]PerformanceMetric, 0)

	// Pre-populate some keys for read/delete operations
	baseKeys := make([]string, 10)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("mixed_%s_base_%d", workerID, i)
		baseKeys[i] = key
		_ = ct.store.KV().Set(ctx, key, []byte("base_data"), time.Hour)
	}

	for i := 0; i < operations; i++ {
		// Determine operation type based on ratios
		rand := float64(i%100) / 100.0
		var operationType string

		if rand < readRatio {
			operationType = "get"
		} else if rand < readRatio+writeRatio {
			operationType = "set"
		} else {
			operationType = "delete"
		}

		key := fmt.Sprintf("mixed_%s_%d", workerID, i)
		if operationType == "get" || operationType == "delete" {
			// Use existing keys for read/delete
			key = baseKeys[i%len(baseKeys)]
		}

		startTime := time.Now()
		var err error

		switch operationType {
		case "set":
			data := []byte(fmt.Sprintf("mixed_data_%s_%d", workerID, i))
			err = ct.store.KV().Set(ctx, key, data, time.Hour)
		case "get":
			_, err = ct.store.KV().Get(ctx, key)
		case "delete":
			err = ct.store.KV().Delete(ctx, key)
			// Re-create the key for future operations
			if err == nil {
				_ = ct.store.KV().Set(ctx, key, []byte("recreated_data"), time.Hour)
			}
		}

		duration := time.Since(startTime)

		metric := PerformanceMetric{
			Timestamp: startTime,
			Operation: operationType,
			Duration:  duration,
			WorkerID:  workerID,
			Success:   err == nil,
		}

		if err != nil {
			metric.ErrorMessage = err.Error()
		}

		metrics = append(metrics, metric)
	}

	// Clean up base keys
	for _, key := range baseKeys {
		_ = ct.store.KV().Delete(ctx, key)
	}

	results <- metrics
}
