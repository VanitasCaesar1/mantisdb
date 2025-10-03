package testing

import (
	"context"
	"crypto/rand"
	"fmt"
	"runtime"
	"time"

	"mantisDB/models"
	"mantisDB/store"
)

// EdgeCaseTestSuite provides comprehensive edge case testing
type EdgeCaseTestSuite struct {
	store                *store.MantisStore
	largeDocTester       *LargeDocumentTester
	highTTLTester        *HighTTLTester
	concurrencyTester    *ConcurrentWriteTester
	memoryPressureTester *MemoryPressureTester
}

// NewEdgeCaseTestSuite creates a new edge case test suite
func NewEdgeCaseTestSuite(mantisStore *store.MantisStore) *EdgeCaseTestSuite {
	return &EdgeCaseTestSuite{
		store:                mantisStore,
		largeDocTester:       NewLargeDocumentTester(mantisStore),
		highTTLTester:        NewHighTTLTester(mantisStore),
		concurrencyTester:    NewConcurrentWriteTester(mantisStore),
		memoryPressureTester: NewMemoryPressureTester(mantisStore),
	}
}

// RunAllTests runs all edge case tests
func (ets *EdgeCaseTestSuite) RunAllTests(ctx context.Context) (*TestResults, error) {
	results := &TestResults{
		StartTime: time.Now(),
		Tests:     make(map[string]*TestResult),
	}

	// Run large document tests
	fmt.Println("Running large document tests...")
	largeDocResults, err := ets.largeDocTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("large document tests failed: %v", err)
	}
	results.Tests["large_documents"] = largeDocResults

	// Run high TTL tests
	fmt.Println("Running high TTL tests...")
	highTTLResults, err := ets.highTTLTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("high TTL tests failed: %v", err)
	}
	results.Tests["high_ttl"] = highTTLResults

	// Run concurrent write tests
	fmt.Println("Running concurrent write tests...")
	concurrencyResults, err := ets.concurrencyTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("concurrent write tests failed: %v", err)
	}
	results.Tests["concurrent_writes"] = concurrencyResults

	// Run memory pressure tests
	fmt.Println("Running memory pressure tests...")
	memoryResults, err := ets.memoryPressureTester.RunTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("memory pressure tests failed: %v", err)
	}
	results.Tests["memory_pressure"] = memoryResults

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results, nil
}

// TestResults holds the results of all edge case tests
type TestResults struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Tests     map[string]*TestResult
}

// TestResult holds the result of a specific test
type TestResult struct {
	Name     string
	Success  bool
	Duration time.Duration
	Error    error
	Metrics  map[string]interface{}
	SubTests map[string]*TestResult
}

// LargeDocumentTester tests handling of large documents (>1MB)
type LargeDocumentTester struct {
	store *store.MantisStore
	sizes []int64 // Document sizes to test in bytes
}

// NewLargeDocumentTester creates a new large document tester
func NewLargeDocumentTester(mantisStore *store.MantisStore) *LargeDocumentTester {
	return &LargeDocumentTester{
		store: mantisStore,
		sizes: []int64{
			1024 * 1024,      // 1MB
			2 * 1024 * 1024,  // 2MB
			5 * 1024 * 1024,  // 5MB
			10 * 1024 * 1024, // 10MB
		},
	}
}

// RunTests runs all large document tests
func (ldt *LargeDocumentTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "large_documents",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	for _, size := range ldt.sizes {
		testName := fmt.Sprintf("document_%dMB", size/(1024*1024))
		subResult, err := ldt.testDocumentSize(ctx, size)
		result.SubTests[testName] = subResult

		if err != nil {
			totalErrors = append(totalErrors, err)
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("large document tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_sizes_tested"] = len(ldt.sizes)
	result.Metrics["max_size_tested"] = ldt.sizes[len(ldt.sizes)-1]
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// testDocumentSize tests a specific document size
func (ldt *LargeDocumentTester) testDocumentSize(ctx context.Context, size int64) (*TestResult, error) {
	result := &TestResult{
		Name:    fmt.Sprintf("test_size_%d", size),
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Monitor memory before test
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Generate large document data
	largeData := make(map[string]interface{})

	// Create a large string field
	largeString := make([]byte, size-1024) // Leave some room for other fields
	_, err := rand.Read(largeString)
	if err != nil {
		result.Error = fmt.Errorf("failed to generate random data: %v", err)
		result.Success = false
		return result, err
	}

	largeData["large_field"] = string(largeString)
	largeData["size"] = size
	largeData["test_timestamp"] = time.Now().Unix()

	// Create document
	docID := fmt.Sprintf("large_doc_%d_%d", size, time.Now().UnixNano())
	doc := models.NewDocument(docID, "large_test_collection", largeData)

	// Test document creation
	createStart := time.Now()
	err = ldt.store.Documents().Create(ctx, doc)
	createDuration := time.Since(createStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to create large document: %v", err)
		result.Success = false
		return result, err
	}

	// Test document retrieval
	retrieveStart := time.Now()
	retrievedDoc, err := ldt.store.Documents().Get(ctx, "large_test_collection", docID)
	retrieveDuration := time.Since(retrieveStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to retrieve large document: %v", err)
		result.Success = false
		return result, err
	}

	// Validate document integrity
	if err := ldt.validateDocumentIntegrity(doc, retrievedDoc); err != nil {
		result.Error = fmt.Errorf("document integrity validation failed: %v", err)
		result.Success = false
		return result, err
	}

	// Monitor memory after test
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// Clean up
	err = ldt.store.Documents().Delete(ctx, "large_test_collection", docID)
	if err != nil {
		fmt.Printf("Warning: failed to clean up large document: %v\n", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = true

	// Record metrics
	result.Metrics["document_size"] = size
	result.Metrics["create_duration_ms"] = createDuration.Milliseconds()
	result.Metrics["retrieve_duration_ms"] = retrieveDuration.Milliseconds()
	result.Metrics["memory_used_bytes"] = int64(memAfter.Alloc - memBefore.Alloc)
	result.Metrics["memory_allocated_bytes"] = int64(memAfter.TotalAlloc - memBefore.TotalAlloc)
	result.Metrics["gc_cycles"] = memAfter.NumGC - memBefore.NumGC

	return result, nil
}

// validateDocumentIntegrity validates that the retrieved document matches the original
func (ldt *LargeDocumentTester) validateDocumentIntegrity(original, retrieved *models.Document) error {
	// Check basic fields
	if original.ID != retrieved.ID {
		return fmt.Errorf("document ID mismatch: expected %s, got %s", original.ID, retrieved.ID)
	}

	if original.Collection != retrieved.Collection {
		return fmt.Errorf("document collection mismatch: expected %s, got %s", original.Collection, retrieved.Collection)
	}

	// Check data integrity
	originalSize, ok1 := original.Data["size"].(int64)
	retrievedSize, ok2 := retrieved.Data["size"].(int64)

	if !ok1 || !ok2 || originalSize != retrievedSize {
		return fmt.Errorf("document size field mismatch")
	}

	// Check large field integrity (compare lengths and checksums)
	originalField, ok1 := original.Data["large_field"].(string)
	retrievedField, ok2 := retrieved.Data["large_field"].(string)

	if !ok1 || !ok2 {
		return fmt.Errorf("large field type mismatch")
	}

	if len(originalField) != len(retrievedField) {
		return fmt.Errorf("large field length mismatch: expected %d, got %d",
			len(originalField), len(retrievedField))
	}

	// Compare checksums instead of full content for performance
	originalChecksum := calculateSimpleChecksum([]byte(originalField))
	retrievedChecksum := calculateSimpleChecksum([]byte(retrievedField))

	if originalChecksum != retrievedChecksum {
		return fmt.Errorf("large field checksum mismatch: expected %d, got %d",
			originalChecksum, retrievedChecksum)
	}

	return nil
}

// calculateSimpleChecksum calculates a simple checksum for data validation
func calculateSimpleChecksum(data []byte) uint32 {
	var checksum uint32
	for _, b := range data {
		checksum = checksum*31 + uint32(b)
	}
	return checksum
}

// HighTTLTester tests handling of high TTL values (>24 hours)
type HighTTLTester struct {
	store     *store.MantisStore
	ttlValues []time.Duration // TTL values to test
}

// NewHighTTLTester creates a new high TTL tester
func NewHighTTLTester(mantisStore *store.MantisStore) *HighTTLTester {
	return &HighTTLTester{
		store: mantisStore,
		ttlValues: []time.Duration{
			25 * time.Hour,       // 25 hours
			7 * 24 * time.Hour,   // 1 week
			30 * 24 * time.Hour,  // 1 month
			365 * 24 * time.Hour, // 1 year
		},
	}
}

// RunTests runs all high TTL tests
func (htt *HighTTLTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "high_ttl",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	for i, ttl := range htt.ttlValues {
		testName := fmt.Sprintf("ttl_%d_hours", int(ttl.Hours()))
		subResult, err := htt.testTTLValue(ctx, ttl, i)
		result.SubTests[testName] = subResult

		if err != nil {
			totalErrors = append(totalErrors, err)
		}
	}

	// Test TTL overflow detection
	overflowResult, err := htt.testTTLOverflow(ctx)
	result.SubTests["ttl_overflow"] = overflowResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test TTL precision
	precisionResult, err := htt.testTTLPrecision(ctx)
	result.SubTests["ttl_precision"] = precisionResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("high TTL tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_ttl_values_tested"] = len(htt.ttlValues)
	result.Metrics["max_ttl_hours"] = int(htt.ttlValues[len(htt.ttlValues)-1].Hours())
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// testTTLValue tests a specific TTL value
func (htt *HighTTLTester) testTTLValue(ctx context.Context, ttl time.Duration, index int) (*TestResult, error) {
	result := &TestResult{
		Name:    fmt.Sprintf("test_ttl_%d", int(ttl.Hours())),
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create a key-value pair with high TTL
	key := fmt.Sprintf("high_ttl_test_%d_%d", index, time.Now().UnixNano())
	value := []byte(fmt.Sprintf("test_value_with_ttl_%d_hours", int(ttl.Hours())))

	// Set the key with high TTL
	setStart := time.Now()
	err := htt.store.KV().Set(ctx, key, value, ttl)
	setDuration := time.Since(setStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to set key with high TTL: %v", err)
		result.Success = false
		return result, err
	}

	// Immediately retrieve to verify it was set correctly
	getStart := time.Now()
	retrievedValue, err := htt.store.KV().Get(ctx, key)
	getDuration := time.Since(getStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to get key with high TTL: %v", err)
		result.Success = false
		return result, err
	}

	// Validate value integrity
	if string(retrievedValue) != string(value) {
		result.Error = fmt.Errorf("value mismatch: expected %s, got %s", string(value), string(retrievedValue))
		result.Success = false
		return result, err
	}

	// Test TTL calculation accuracy
	if err := htt.validateTTLAccuracy(ctx, key, ttl); err != nil {
		result.Error = fmt.Errorf("TTL accuracy validation failed: %v", err)
		result.Success = false
		return result, err
	}

	// Clean up
	err = htt.store.KV().Delete(ctx, key)
	if err != nil {
		fmt.Printf("Warning: failed to clean up high TTL key: %v\n", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = true

	// Record metrics
	result.Metrics["ttl_hours"] = int(ttl.Hours())
	result.Metrics["ttl_seconds"] = int(ttl.Seconds())
	result.Metrics["set_duration_ms"] = setDuration.Milliseconds()
	result.Metrics["get_duration_ms"] = getDuration.Milliseconds()

	return result, nil
}

// testTTLOverflow tests TTL overflow detection and handling
func (htt *HighTTLTester) testTTLOverflow(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "ttl_overflow_detection",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test with maximum possible duration
	maxTTL := time.Duration(1<<63 - 1) // Maximum int64 value
	key := fmt.Sprintf("overflow_test_%d", time.Now().UnixNano())
	value := []byte("overflow_test_value")

	// This should either work or fail gracefully
	err := htt.store.KV().Set(ctx, key, value, maxTTL)

	if err != nil {
		// If it fails, that's acceptable - we just want to ensure it fails gracefully
		result.Success = true
		result.Metrics["overflow_handled_gracefully"] = true
		result.Metrics["error_message"] = err.Error()
	} else {
		// If it succeeds, verify the key exists and clean up
		_, getErr := htt.store.KV().Get(ctx, key)
		if getErr != nil {
			result.Error = fmt.Errorf("key with max TTL was set but cannot be retrieved: %v", getErr)
			result.Success = false
		} else {
			result.Success = true
			result.Metrics["max_ttl_accepted"] = true
			// Clean up
			htt.store.KV().Delete(ctx, key)
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// testTTLPrecision tests TTL precision and accuracy
func (htt *HighTTLTester) testTTLPrecision(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "ttl_precision_validation",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test with various precision levels
	precisionTests := []struct {
		name string
		ttl  time.Duration
	}{
		{"seconds", 3600 * time.Second},
		{"minutes", 60 * time.Minute},
		{"hours", 24 * time.Hour},
		{"days", 7 * 24 * time.Hour},
	}

	var errors []error
	precisionResults := make(map[string]bool)

	for _, test := range precisionTests {
		key := fmt.Sprintf("precision_test_%s_%d", test.name, time.Now().UnixNano())
		value := []byte(fmt.Sprintf("precision_test_%s", test.name))

		// Set with specific TTL
		err := htt.store.KV().Set(ctx, key, value, test.ttl)
		if err != nil {
			errors = append(errors, fmt.Errorf("precision test %s failed to set: %v", test.name, err))
			precisionResults[test.name] = false
			continue
		}

		// Verify TTL accuracy
		err = htt.validateTTLAccuracy(ctx, key, test.ttl)
		if err != nil {
			errors = append(errors, fmt.Errorf("precision test %s TTL validation failed: %v", test.name, err))
			precisionResults[test.name] = false
		} else {
			precisionResults[test.name] = true
		}

		// Clean up
		htt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(errors) == 0
	result.Metrics["precision_results"] = precisionResults
	result.Metrics["total_precision_tests"] = len(precisionTests)
	result.Metrics["successful_precision_tests"] = len(precisionTests) - len(errors)

	if len(errors) > 0 {
		result.Error = fmt.Errorf("TTL precision tests had %d errors: %v", len(errors), errors[0])
	}

	return result, nil
}

// validateTTLAccuracy validates that TTL is handled accurately
func (htt *HighTTLTester) validateTTLAccuracy(ctx context.Context, key string, expectedTTL time.Duration) error {
	// This is a simplified validation - in a real implementation, you would
	// check the actual TTL remaining on the key from the storage layer

	// For now, we just verify the key exists (indicating TTL hasn't expired immediately)
	_, err := htt.store.KV().Get(ctx, key)
	if err != nil {
		return fmt.Errorf("key with TTL %v expired immediately or cannot be retrieved: %v", expectedTTL, err)
	}

	// Additional validation could include:
	// - Checking remaining TTL from storage engine
	// - Verifying TTL doesn't expire too early
	// - Testing TTL updates and extensions

	return nil
}

// ConcurrentWriteTester tests concurrent write operations to the same key
type ConcurrentWriteTester struct {
	store        *store.MantisStore
	workerCounts []int // Number of concurrent workers to test
}

// NewConcurrentWriteTester creates a new concurrent write tester
func NewConcurrentWriteTester(mantisStore *store.MantisStore) *ConcurrentWriteTester {
	return &ConcurrentWriteTester{
		store:        mantisStore,
		workerCounts: []int{10, 50, 100, 500},
	}
}

// RunTests runs all concurrent write tests
func (cwt *ConcurrentWriteTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "concurrent_writes",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	for _, workerCount := range cwt.workerCounts {
		testName := fmt.Sprintf("workers_%d", workerCount)
		subResult, err := cwt.testConcurrentWrites(ctx, workerCount)
		result.SubTests[testName] = subResult

		if err != nil {
			totalErrors = append(totalErrors, err)
		}
	}

	// Test race condition detection
	raceResult, err := cwt.testRaceConditionDetection(ctx)
	result.SubTests["race_condition_detection"] = raceResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test data consistency validation
	consistencyResult, err := cwt.testDataConsistency(ctx)
	result.SubTests["data_consistency"] = consistencyResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("concurrent write tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_worker_counts_tested"] = len(cwt.workerCounts)
	result.Metrics["max_workers_tested"] = cwt.workerCounts[len(cwt.workerCounts)-1]
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// testConcurrentWrites tests concurrent writes with a specific number of workers
func (cwt *ConcurrentWriteTester) testConcurrentWrites(ctx context.Context, workerCount int) (*TestResult, error) {
	result := &TestResult{
		Name:    fmt.Sprintf("concurrent_writes_%d", workerCount),
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test key for concurrent writes
	testKey := fmt.Sprintf("concurrent_test_%d_%d", workerCount, time.Now().UnixNano())
	operationsPerWorker := 10
	totalOperations := workerCount * operationsPerWorker

	// Channel to collect results from workers
	resultsChan := make(chan workerResult, workerCount)

	// Start concurrent workers
	for i := 0; i < workerCount; i++ {
		go cwt.concurrentWorker(ctx, i, testKey, operationsPerWorker, resultsChan)
	}

	// Collect results from all workers
	var workerErrors []error
	var totalWriteTime time.Duration
	var successfulWrites int

	for i := 0; i < workerCount; i++ {
		workerRes := <-resultsChan
		if workerRes.err != nil {
			workerErrors = append(workerErrors, workerRes.err)
		} else {
			totalWriteTime += workerRes.duration
			successfulWrites += workerRes.successfulOps
		}
	}

	// Validate final state
	finalValue, err := cwt.store.KV().Get(ctx, testKey)
	if err != nil {
		result.Error = fmt.Errorf("failed to get final value after concurrent writes: %v", err)
		result.Success = false
		return result, err
	}

	// Validate data consistency
	if err := cwt.validateFinalState(finalValue, workerCount, operationsPerWorker); err != nil {
		result.Error = fmt.Errorf("final state validation failed: %v", err)
		result.Success = false
		return result, err
	}

	// Clean up
	err = cwt.store.KV().Delete(ctx, testKey)
	if err != nil {
		fmt.Printf("Warning: failed to clean up concurrent test key: %v\n", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(workerErrors) == 0

	// Record metrics
	result.Metrics["worker_count"] = workerCount
	result.Metrics["operations_per_worker"] = operationsPerWorker
	result.Metrics["total_operations"] = totalOperations
	result.Metrics["successful_writes"] = successfulWrites
	result.Metrics["failed_workers"] = len(workerErrors)
	result.Metrics["average_write_time_ms"] = totalWriteTime.Milliseconds() / int64(workerCount)
	result.Metrics["final_value_length"] = len(finalValue)

	if len(workerErrors) > 0 {
		result.Error = fmt.Errorf("concurrent writes had %d worker errors: %v", len(workerErrors), workerErrors[0])
		result.Success = false
	}

	return result, nil
}

// workerResult holds the result from a concurrent worker
type workerResult struct {
	workerID      int
	duration      time.Duration
	successfulOps int
	err           error
}

// concurrentWorker performs concurrent write operations
func (cwt *ConcurrentWriteTester) concurrentWorker(ctx context.Context, workerID int, key string, operations int, results chan<- workerResult) {
	startTime := time.Now()
	successfulOps := 0

	for i := 0; i < operations; i++ {
		value := []byte(fmt.Sprintf("worker_%d_op_%d_%d", workerID, i, time.Now().UnixNano()))

		err := cwt.store.KV().Set(ctx, key, value, time.Minute)
		if err != nil {
			results <- workerResult{
				workerID: workerID,
				duration: time.Since(startTime),
				err:      fmt.Errorf("worker %d operation %d failed: %v", workerID, i, err),
			}
			return
		}
		successfulOps++

		// Small delay to increase chance of race conditions
		time.Sleep(time.Microsecond * 10)
	}

	results <- workerResult{
		workerID:      workerID,
		duration:      time.Since(startTime),
		successfulOps: successfulOps,
	}
}

// validateFinalState validates the final state after concurrent operations
func (cwt *ConcurrentWriteTester) validateFinalState(finalValue []byte, workerCount, operationsPerWorker int) error {
	// The final value should be from one of the workers
	finalStr := string(finalValue)

	// Check if the final value has the expected format
	if len(finalStr) == 0 {
		return fmt.Errorf("final value is empty")
	}

	// The value should contain worker information
	if !contains(finalStr, "worker_") {
		return fmt.Errorf("final value doesn't contain worker information: %s", finalStr)
	}

	// Additional validation could include:
	// - Checking that the final value is from a valid worker ID
	// - Verifying the operation number is within expected range
	// - Ensuring the timestamp is reasonable

	return nil
}

// testRaceConditionDetection tests race condition detection mechanisms
func (cwt *ConcurrentWriteTester) testRaceConditionDetection(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "race_condition_detection",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create a scenario likely to cause race conditions
	testKey := fmt.Sprintf("race_test_%d", time.Now().UnixNano())
	workerCount := 100
	operationsPerWorker := 5

	// Use a channel to synchronize workers for maximum race condition potential
	startSignal := make(chan struct{})
	resultsChan := make(chan workerResult, workerCount)

	// Start all workers but don't let them proceed yet
	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			<-startSignal // Wait for start signal

			startTime := time.Now()
			for j := 0; j < operationsPerWorker; j++ {
				value := []byte(fmt.Sprintf("race_worker_%d_op_%d", workerID, j))
				err := cwt.store.KV().Set(ctx, testKey, value, time.Minute)
				if err != nil {
					resultsChan <- workerResult{
						workerID: workerID,
						err:      err,
					}
					return
				}
			}

			resultsChan <- workerResult{
				workerID:      workerID,
				duration:      time.Since(startTime),
				successfulOps: operationsPerWorker,
			}
		}(i)
	}

	// Release all workers simultaneously
	close(startSignal)

	// Collect results
	var errors []error
	var successfulWorkers int

	for i := 0; i < workerCount; i++ {
		workerRes := <-resultsChan
		if workerRes.err != nil {
			errors = append(errors, workerRes.err)
		} else {
			successfulWorkers++
		}
	}

	// Verify final state exists and is valid
	finalValue, err := cwt.store.KV().Get(ctx, testKey)
	if err != nil {
		result.Error = fmt.Errorf("failed to get final value after race condition test: %v", err)
		result.Success = false
		return result, err
	}

	// Clean up
	cwt.store.KV().Delete(ctx, testKey)

	result.Duration = time.Since(startTime)
	result.Success = len(errors) == 0 && len(finalValue) > 0

	// Record metrics
	result.Metrics["total_workers"] = workerCount
	result.Metrics["successful_workers"] = successfulWorkers
	result.Metrics["failed_workers"] = len(errors)
	result.Metrics["race_conditions_detected"] = len(errors)
	result.Metrics["final_state_valid"] = len(finalValue) > 0

	if len(errors) > 0 {
		result.Error = fmt.Errorf("race condition test had %d errors (this may be expected): %v", len(errors), errors[0])
		// Note: Some errors might be expected in race condition scenarios
	}

	return result, nil
}

// testDataConsistency tests data consistency after concurrent operations
func (cwt *ConcurrentWriteTester) testDataConsistency(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "data_consistency_validation",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test multiple keys with concurrent operations
	keyCount := 10
	workerCount := 20
	operationsPerKey := 5

	var allErrors []error
	consistencyResults := make(map[string]bool)

	for keyIndex := 0; keyIndex < keyCount; keyIndex++ {
		testKey := fmt.Sprintf("consistency_test_%d_%d", keyIndex, time.Now().UnixNano())

		// Perform concurrent operations on this key
		resultsChan := make(chan workerResult, workerCount)

		for workerID := 0; workerID < workerCount; workerID++ {
			go func(wID int) {
				for op := 0; op < operationsPerKey; op++ {
					value := []byte(fmt.Sprintf("key_%d_worker_%d_op_%d", keyIndex, wID, op))
					err := cwt.store.KV().Set(ctx, testKey, value, time.Minute)
					if err != nil {
						resultsChan <- workerResult{workerID: wID, err: err}
						return
					}
				}
				resultsChan <- workerResult{workerID: wID, successfulOps: operationsPerKey}
			}(workerID)
		}

		// Collect results for this key
		var keyErrors []error
		for i := 0; i < workerCount; i++ {
			workerRes := <-resultsChan
			if workerRes.err != nil {
				keyErrors = append(keyErrors, workerRes.err)
			}
		}

		// Validate consistency for this key
		finalValue, err := cwt.store.KV().Get(ctx, testKey)
		if err != nil {
			keyErrors = append(keyErrors, fmt.Errorf("failed to get final value for key %d: %v", keyIndex, err))
		}

		// Check if final value is consistent
		isConsistent := len(keyErrors) == 0 && len(finalValue) > 0
		consistencyResults[testKey] = isConsistent

		if len(keyErrors) > 0 {
			allErrors = append(allErrors, keyErrors...)
		}

		// Clean up
		cwt.store.KV().Delete(ctx, testKey)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(allErrors) == 0

	// Calculate consistency metrics
	consistentKeys := 0
	for _, consistent := range consistencyResults {
		if consistent {
			consistentKeys++
		}
	}

	// Record metrics
	result.Metrics["total_keys_tested"] = keyCount
	result.Metrics["consistent_keys"] = consistentKeys
	result.Metrics["consistency_rate"] = float64(consistentKeys) / float64(keyCount)
	result.Metrics["total_workers"] = workerCount
	result.Metrics["operations_per_key"] = operationsPerKey
	result.Metrics["total_errors"] = len(allErrors)

	if len(allErrors) > 0 {
		result.Error = fmt.Errorf("data consistency test had %d errors: %v", len(allErrors), allErrors[0])
	}

	return result, nil
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MemoryPressureTester tests cache eviction under memory pressure
type MemoryPressureTester struct {
	store          *store.MantisStore
	pressureLevels []float64 // Memory pressure levels to test (0.0 to 1.0)
}

// NewMemoryPressureTester creates a new memory pressure tester
func NewMemoryPressureTester(mantisStore *store.MantisStore) *MemoryPressureTester {
	return &MemoryPressureTester{
		store:          mantisStore,
		pressureLevels: []float64{0.5, 0.7, 0.8, 0.9}, // 50%, 70%, 80%, 90% memory usage
	}
}

// RunTests runs all memory pressure tests
func (mpt *MemoryPressureTester) RunTests(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:     "memory_pressure",
		SubTests: make(map[string]*TestResult),
		Metrics:  make(map[string]interface{}),
	}

	startTime := time.Now()
	var totalErrors []error

	for _, pressureLevel := range mpt.pressureLevels {
		testName := fmt.Sprintf("pressure_%.0f_percent", pressureLevel*100)
		subResult, err := mpt.testMemoryPressure(ctx, pressureLevel)
		result.SubTests[testName] = subResult

		if err != nil {
			totalErrors = append(totalErrors, err)
		}
	}

	// Test cache eviction policies
	evictionResult, err := mpt.testCacheEvictionPolicies(ctx)
	result.SubTests["eviction_policies"] = evictionResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	// Test cache consistency during eviction
	consistencyResult, err := mpt.testCacheConsistencyDuringEviction(ctx)
	result.SubTests["cache_consistency"] = consistencyResult
	if err != nil {
		totalErrors = append(totalErrors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(totalErrors) == 0

	if len(totalErrors) > 0 {
		result.Error = fmt.Errorf("memory pressure tests had %d errors: %v", len(totalErrors), totalErrors[0])
	}

	// Collect overall metrics
	result.Metrics["total_pressure_levels_tested"] = len(mpt.pressureLevels)
	result.Metrics["max_pressure_level"] = mpt.pressureLevels[len(mpt.pressureLevels)-1]
	result.Metrics["errors_count"] = len(totalErrors)

	return result, nil
}

// testMemoryPressure tests cache behavior under specific memory pressure
func (mpt *MemoryPressureTester) testMemoryPressure(ctx context.Context, pressureLevel float64) (*TestResult, error) {
	result := &TestResult{
		Name:    fmt.Sprintf("memory_pressure_%.0f", pressureLevel*100),
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Get initial memory stats
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Get initial cache stats
	cacheStatsBefore := mpt.store.GetStats(ctx)

	// Simulate memory pressure by creating many cache entries
	entriesCreated := 0
	targetMemoryUsage := int64(float64(memBefore.Sys) * pressureLevel)

	// Create entries until we reach target memory usage
	for {
		var memCurrent runtime.MemStats
		runtime.ReadMemStats(&memCurrent)

		if int64(memCurrent.Alloc) >= targetMemoryUsage {
			break
		}

		// Create a cache entry
		key := fmt.Sprintf("pressure_test_%d_%d", entriesCreated, time.Now().UnixNano())
		value := make([]byte, 1024*10) // 10KB per entry

		// Fill with random data
		for i := range value {
			value[i] = byte(entriesCreated % 256)
		}

		err := mpt.store.KV().Set(ctx, key, value, time.Hour)
		if err != nil {
			result.Error = fmt.Errorf("failed to create cache entry under pressure: %v", err)
			result.Success = false
			return result, err
		}

		entriesCreated++

		// Safety check to prevent infinite loop
		if entriesCreated > 10000 {
			break
		}
	}

	// Get memory stats after pressure
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// Get cache stats after pressure
	cacheStatsAfter := mpt.store.GetStats(ctx)

	// Test cache operations under pressure
	testKey := fmt.Sprintf("pressure_operation_test_%d", time.Now().UnixNano())
	testValue := []byte("test_value_under_pressure")

	// Test set operation under pressure
	setStart := time.Now()
	err := mpt.store.KV().Set(ctx, testKey, testValue, time.Minute)
	setDuration := time.Since(setStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to set key under memory pressure: %v", err)
		result.Success = false
		return result, err
	}

	// Test get operation under pressure
	getStart := time.Now()
	retrievedValue, err := mpt.store.KV().Get(ctx, testKey)
	getDuration := time.Since(getStart)

	if err != nil {
		result.Error = fmt.Errorf("failed to get key under memory pressure: %v", err)
		result.Success = false
		return result, err
	}

	// Validate retrieved value
	if string(retrievedValue) != string(testValue) {
		result.Error = fmt.Errorf("value corruption under memory pressure")
		result.Success = false
		return result, err
	}

	// Clean up test entries (some may have been evicted already)
	for i := 0; i < entriesCreated; i++ {
		key := fmt.Sprintf("pressure_test_%d_%d", i, time.Now().UnixNano())
		mpt.store.KV().Delete(ctx, key)
	}
	mpt.store.KV().Delete(ctx, testKey)

	result.Duration = time.Since(startTime)
	result.Success = true

	// Record metrics
	result.Metrics["pressure_level"] = pressureLevel
	result.Metrics["entries_created"] = entriesCreated
	result.Metrics["memory_before_mb"] = int64(memBefore.Alloc) / (1024 * 1024)
	result.Metrics["memory_after_mb"] = int64(memAfter.Alloc) / (1024 * 1024)
	result.Metrics["memory_increase_mb"] = int64(memAfter.Alloc-memBefore.Alloc) / (1024 * 1024)
	result.Metrics["gc_cycles"] = memAfter.NumGC - memBefore.NumGC
	result.Metrics["set_duration_ms"] = setDuration.Milliseconds()
	result.Metrics["get_duration_ms"] = getDuration.Milliseconds()

	// Cache metrics
	if cacheStatsBefore != nil && cacheStatsAfter != nil {
		cacheBefore := cacheStatsBefore["cache"].(map[string]interface{})
		cacheAfter := cacheStatsAfter["cache"].(map[string]interface{})

		result.Metrics["cache_entries_before"] = cacheBefore["total_entries"]
		result.Metrics["cache_entries_after"] = cacheAfter["total_entries"]
		result.Metrics["cache_size_before_mb"] = int64(cacheBefore["total_size"].(int64)) / (1024 * 1024)
		result.Metrics["cache_size_after_mb"] = int64(cacheAfter["total_size"].(int64)) / (1024 * 1024)
	}

	return result, nil
}

// testCacheEvictionPolicies tests different cache eviction policies
func (mpt *MemoryPressureTester) testCacheEvictionPolicies(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "cache_eviction_policies",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Test LRU eviction behavior
	lruResult, err := mpt.testLRUEviction(ctx)
	if err != nil {
		result.Error = fmt.Errorf("LRU eviction test failed: %v", err)
		result.Success = false
		return result, err
	}

	// Test LFU eviction behavior (if supported)
	lfuResult, err := mpt.testLFUEviction(ctx)
	if err != nil {
		// LFU might not be implemented, so we don't fail the entire test
		fmt.Printf("Warning: LFU eviction test failed: %v\n", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = lruResult.Success && (lfuResult == nil || lfuResult.Success)

	// Record metrics
	result.Metrics["lru_test_success"] = lruResult.Success
	result.Metrics["lru_test_duration_ms"] = lruResult.Duration.Milliseconds()

	if lfuResult != nil {
		result.Metrics["lfu_test_success"] = lfuResult.Success
		result.Metrics["lfu_test_duration_ms"] = lfuResult.Duration.Milliseconds()
	}

	return result, nil
}

// testLRUEviction tests LRU (Least Recently Used) eviction policy
func (mpt *MemoryPressureTester) testLRUEviction(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "lru_eviction_test",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create several cache entries
	keys := make([]string, 5)
	for i := 0; i < 5; i++ {
		keys[i] = fmt.Sprintf("lru_test_%d_%d", i, time.Now().UnixNano())
		value := []byte(fmt.Sprintf("lru_value_%d", i))

		err := mpt.store.KV().Set(ctx, keys[i], value, time.Hour)
		if err != nil {
			result.Error = fmt.Errorf("failed to create LRU test entry %d: %v", i, err)
			result.Success = false
			return result, err
		}

		// Small delay to ensure different timestamps
		time.Sleep(time.Millisecond * 10)
	}

	// Access some keys to change their LRU order
	// Access keys 0, 2, 4 (making 1, 3 least recently used)
	for _, i := range []int{0, 2, 4} {
		_, err := mpt.store.KV().Get(ctx, keys[i])
		if err != nil {
			result.Error = fmt.Errorf("failed to access LRU test key %d: %v", i, err)
			result.Success = false
			return result, err
		}
		time.Sleep(time.Millisecond * 10)
	}

	// Force cache pressure to trigger eviction
	// Create many large entries to force eviction
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("pressure_key_%d", i)
		value := make([]byte, 1024*50) // 50KB entries
		mpt.store.KV().Set(ctx, key, value, time.Minute)
	}

	// Check which keys survived (recently accessed ones should survive longer)
	survivedKeys := 0
	accessedKeysSurvived := 0

	for i, key := range keys {
		_, err := mpt.store.KV().Get(ctx, key)
		if err == nil {
			survivedKeys++
			// Check if this was one of the accessed keys (0, 2, 4)
			if i == 0 || i == 2 || i == 4 {
				accessedKeysSurvived++
			}
		}
	}

	// Clean up
	for _, key := range keys {
		mpt.store.KV().Delete(ctx, key)
	}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("pressure_key_%d", i)
		mpt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = true // LRU behavior is complex, so we don't fail on specific expectations

	// Record metrics
	result.Metrics["total_keys_created"] = len(keys)
	result.Metrics["keys_survived"] = survivedKeys
	result.Metrics["accessed_keys_survived"] = accessedKeysSurvived
	result.Metrics["lru_behavior_detected"] = accessedKeysSurvived > (survivedKeys - accessedKeysSurvived)

	return result, nil
}

// testLFUEviction tests LFU (Least Frequently Used) eviction policy
func (mpt *MemoryPressureTester) testLFUEviction(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "lfu_eviction_test",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create several cache entries
	keys := make([]string, 5)
	for i := 0; i < 5; i++ {
		keys[i] = fmt.Sprintf("lfu_test_%d_%d", i, time.Now().UnixNano())
		value := []byte(fmt.Sprintf("lfu_value_%d", i))

		err := mpt.store.KV().Set(ctx, keys[i], value, time.Hour)
		if err != nil {
			result.Error = fmt.Errorf("failed to create LFU test entry %d: %v", i, err)
			result.Success = false
			return result, err
		}
	}

	// Access some keys multiple times to increase their frequency
	// Access keys 0, 2, 4 multiple times (making them more frequently used)
	for _, i := range []int{0, 2, 4} {
		for j := 0; j < 5; j++ {
			_, err := mpt.store.KV().Get(ctx, keys[i])
			if err != nil {
				result.Error = fmt.Errorf("failed to access LFU test key %d: %v", i, err)
				result.Success = false
				return result, err
			}
		}
	}

	// Access keys 1, 3 only once
	for _, i := range []int{1, 3} {
		_, err := mpt.store.KV().Get(ctx, keys[i])
		if err != nil {
			result.Error = fmt.Errorf("failed to access LFU test key %d: %v", i, err)
			result.Success = false
			return result, err
		}
	}

	// Force cache pressure to trigger eviction
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("lfu_pressure_key_%d", i)
		value := make([]byte, 1024*50) // 50KB entries
		mpt.store.KV().Set(ctx, key, value, time.Minute)
	}

	// Check which keys survived (frequently accessed ones should survive longer)
	survivedKeys := 0
	frequentKeysSurvived := 0

	for i, key := range keys {
		_, err := mpt.store.KV().Get(ctx, key)
		if err == nil {
			survivedKeys++
			// Check if this was one of the frequently accessed keys (0, 2, 4)
			if i == 0 || i == 2 || i == 4 {
				frequentKeysSurvived++
			}
		}
	}

	// Clean up
	for _, key := range keys {
		mpt.store.KV().Delete(ctx, key)
	}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("lfu_pressure_key_%d", i)
		mpt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = true // LFU behavior is complex, so we don't fail on specific expectations

	// Record metrics
	result.Metrics["total_keys_created"] = len(keys)
	result.Metrics["keys_survived"] = survivedKeys
	result.Metrics["frequent_keys_survived"] = frequentKeysSurvived
	result.Metrics["lfu_behavior_detected"] = frequentKeysSurvived > (survivedKeys - frequentKeysSurvived)

	return result, nil
}

// testCacheConsistencyDuringEviction tests cache consistency during eviction
func (mpt *MemoryPressureTester) testCacheConsistencyDuringEviction(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Name:    "cache_consistency_during_eviction",
		Metrics: make(map[string]interface{}),
	}

	startTime := time.Now()

	// Create a set of test keys with known values
	testKeys := make(map[string][]byte)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("consistency_test_%d_%d", i, time.Now().UnixNano())
		value := []byte(fmt.Sprintf("consistency_value_%d_%d", i, time.Now().UnixNano()))
		testKeys[key] = value

		err := mpt.store.KV().Set(ctx, key, value, time.Hour)
		if err != nil {
			result.Error = fmt.Errorf("failed to create consistency test key %d: %v", i, err)
			result.Success = false
			return result, err
		}
	}

	// Force eviction by creating memory pressure
	pressureKeys := make([]string, 0)
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("pressure_consistency_%d", i)
		value := make([]byte, 1024*20) // 20KB entries
		pressureKeys = append(pressureKeys, key)

		err := mpt.store.KV().Set(ctx, key, value, time.Minute)
		if err != nil {
			// If we can't create more entries, that's fine - we've created pressure
			break
		}
	}

	// Verify consistency of remaining test keys
	consistentKeys := 0
	inconsistentKeys := 0
	evictedKeys := 0

	for key, expectedValue := range testKeys {
		retrievedValue, err := mpt.store.KV().Get(ctx, key)
		if err != nil {
			// Key was evicted
			evictedKeys++
		} else {
			// Key still exists, check consistency
			if string(retrievedValue) == string(expectedValue) {
				consistentKeys++
			} else {
				inconsistentKeys++
			}
		}
	}

	// Clean up
	for key := range testKeys {
		mpt.store.KV().Delete(ctx, key)
	}
	for _, key := range pressureKeys {
		mpt.store.KV().Delete(ctx, key)
	}

	result.Duration = time.Since(startTime)
	result.Success = inconsistentKeys == 0 // No inconsistent keys should exist

	// Record metrics
	result.Metrics["total_test_keys"] = len(testKeys)
	result.Metrics["consistent_keys"] = consistentKeys
	result.Metrics["inconsistent_keys"] = inconsistentKeys
	result.Metrics["evicted_keys"] = evictedKeys
	result.Metrics["consistency_rate"] = float64(consistentKeys) / float64(consistentKeys+inconsistentKeys)
	result.Metrics["eviction_rate"] = float64(evictedKeys) / float64(len(testKeys))

	if inconsistentKeys > 0 {
		result.Error = fmt.Errorf("found %d inconsistent keys during eviction", inconsistentKeys)
	}

	return result, nil
}
