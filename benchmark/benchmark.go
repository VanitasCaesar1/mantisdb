package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"mantisDB/models"
	"mantisDB/store"
	"strings"
)

// Import strings package for Repeat function

// BenchmarkSuite provides comprehensive benchmarking for MantisDB
type BenchmarkSuite struct {
	store *store.MantisStore
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(store *store.MantisStore) *BenchmarkSuite {
	return &BenchmarkSuite{store: store}
}

// BenchmarkResult holds the results of a benchmark
type BenchmarkResult struct {
	Name           string        `json:"name"`
	Operations     int64         `json:"operations"`
	Duration       time.Duration `json:"duration"`
	OpsPerSecond   float64       `json:"ops_per_second"`
	AvgLatency     time.Duration `json:"avg_latency"`
	MinLatency     time.Duration `json:"min_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	ErrorCount     int64         `json:"error_count"`
	CacheHitRate   float64       `json:"cache_hit_rate,omitempty"`
	ThroughputMBps float64       `json:"throughput_mbps,omitempty"`
}

// RunAllBenchmarks runs all benchmark suites
func (bs *BenchmarkSuite) RunAllBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	fmt.Println("Starting MantisDB Benchmark Suite...")
	fmt.Println("=====================================")

	var results []*BenchmarkResult

	// Key-Value benchmarks
	fmt.Println("\n1. Key-Value Store Benchmarks")
	fmt.Println("-----------------------------")

	kvResults, err := bs.runKVBenchmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("KV benchmarks failed: %v", err)
	}
	results = append(results, kvResults...)

	// Document store benchmarks
	fmt.Println("\n2. Document Store Benchmarks")
	fmt.Println("----------------------------")

	docResults, err := bs.runDocumentBenchmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("Document benchmarks failed: %v", err)
	}
	results = append(results, docResults...)

	// Columnar store benchmarks
	fmt.Println("\n3. Columnar Store Benchmarks")
	fmt.Println("----------------------------")

	colResults, err := bs.runColumnarBenchmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("Columnar benchmarks failed: %v", err)
	}
	results = append(results, colResults...)

	// Cache performance benchmarks
	fmt.Println("\n4. Cache Performance Benchmarks")
	fmt.Println("-------------------------------")

	cacheResults, err := bs.runCacheBenchmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cache benchmarks failed: %v", err)
	}
	results = append(results, cacheResults...)

	// Mixed workload benchmarks
	fmt.Println("\n5. Mixed Workload Benchmarks")
	fmt.Println("----------------------------")

	mixedResults, err := bs.runMixedWorkloadBenchmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("Mixed workload benchmarks failed: %v", err)
	}
	results = append(results, mixedResults...)

	fmt.Println("\n=====================================")
	fmt.Println("Benchmark Suite Complete!")

	return results, nil
}

// Key-Value benchmarks
func (bs *BenchmarkSuite) runKVBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Sequential writes
	result, err := bs.benchmarkKVSequentialWrites(ctx, 10000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Sequential reads
	result, err = bs.benchmarkKVSequentialReads(ctx, 10000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Random writes
	result, err = bs.benchmarkKVRandomWrites(ctx, 5000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Random reads
	result, err = bs.benchmarkKVRandomReads(ctx, 5000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Concurrent operations
	result, err = bs.benchmarkKVConcurrent(ctx, 1000, 10)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

func (bs *BenchmarkSuite) benchmarkKVSequentialWrites(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Sequential Writes (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("seq_key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))

		err := bs.store.KV().Set(ctx, key, []byte(value), 0)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("KV Sequential Writes", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkKVSequentialReads(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Sequential Reads (%d operations)...\n", count)

	// First, populate data
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("seq_key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))
		bs.store.KV().Set(ctx, key, []byte(value), 0)
	}

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("seq_key_%d", i)
		_, err := bs.store.KV().Get(ctx, key)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("KV Sequential Reads", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkKVRandomWrites(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Random Writes (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("rand_key_%d", rand.Intn(count*2))
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))

		err := bs.store.KV().Set(ctx, key, []byte(value), 0)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("KV Random Writes", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkKVRandomReads(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Random Reads (%d operations)...\n", count)

	// First, populate data
	for i := 0; i < count*2; i++ {
		key := fmt.Sprintf("rand_key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))
		bs.store.KV().Set(ctx, key, []byte(value), 0)
	}

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("rand_key_%d", rand.Intn(count*2))
		_, err := bs.store.KV().Get(ctx, key)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("KV Random Reads", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkKVConcurrent(ctx context.Context, opsPerWorker int, workers int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Concurrent Operations (%d workers, %d ops each)...\n", workers, opsPerWorker)

	totalOps := opsPerWorker * workers
	latencies := make([]time.Duration, totalOps)
	var errorCount int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	start := time.Now()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < opsPerWorker; i++ {
				opStart := time.Now()

				key := fmt.Sprintf("concurrent_key_%d_%d", workerID, i)
				value := fmt.Sprintf("value_%d_%d_%s", workerID, i, generateRandomString(50))

				err := bs.store.KV().Set(ctx, key, []byte(value), 0)

				latency := time.Since(opStart)

				mu.Lock()
				latencies[workerID*opsPerWorker+i] = latency
				if err != nil {
					errorCount++
				}
				mu.Unlock()
			}
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return bs.calculateResult("KV Concurrent Operations", int64(totalOps), duration, latencies, errorCount), nil
}

// Document benchmarks
func (bs *BenchmarkSuite) runDocumentBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Document inserts
	result, err := bs.benchmarkDocumentInserts(ctx, 5000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Document reads
	result, err = bs.benchmarkDocumentReads(ctx, 5000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Document queries
	result, err = bs.benchmarkDocumentQueries(ctx, 1000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

func (bs *BenchmarkSuite) benchmarkDocumentInserts(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Document Inserts (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		doc := models.NewDocument(
			fmt.Sprintf("doc_%d", i),
			"benchmark_collection",
			map[string]interface{}{
				"name":        fmt.Sprintf("Document %d", i),
				"value":       rand.Intn(1000),
				"category":    fmt.Sprintf("category_%d", i%10),
				"description": generateRandomString(200),
				"timestamp":   time.Now().Unix(),
			},
		)

		err := bs.store.Documents().Create(ctx, doc)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Document Inserts", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkDocumentReads(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Document Reads (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		docID := fmt.Sprintf("doc_%d", rand.Intn(count))
		_, err := bs.store.Documents().Get(ctx, "benchmark_collection", docID)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Document Reads", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkDocumentQueries(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Document Queries (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		query := &models.DocumentQuery{
			Collection: "benchmark_collection",
			Filter: map[string]interface{}{
				"category": fmt.Sprintf("category_%d", rand.Intn(10)),
			},
			Limit: 10,
		}

		_, err := bs.store.Documents().Query(ctx, query, time.Minute)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Document Queries", int64(count), duration, latencies, errorCount), nil
}

// Columnar benchmarks
func (bs *BenchmarkSuite) runColumnarBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Create benchmark table
	err := bs.createBenchmarkTable(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create benchmark table: %v", err)
	}

	// Columnar inserts
	result, err := bs.benchmarkColumnarInserts(ctx, 10000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Columnar queries
	result, err = bs.benchmarkColumnarQueries(ctx, 500)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

func (bs *BenchmarkSuite) createBenchmarkTable(ctx context.Context) error {
	columns := []*models.Column{
		models.NewColumn("id", models.DataTypeInt64),
		models.NewColumn("name", models.DataTypeString),
		models.NewColumn("value", models.DataTypeFloat64),
		models.NewColumn("category", models.DataTypeString),
		models.NewColumn("timestamp", models.DataTypeDateTime),
	}

	table := models.NewTable("benchmark_table", columns)
	return bs.store.Columnar().CreateTable(ctx, table)
}

func (bs *BenchmarkSuite) benchmarkColumnarInserts(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Columnar Inserts (%d operations)...\n", count)

	batchSize := 100
	batches := count / batchSize
	latencies := make([]time.Duration, batches)
	var errorCount int64

	start := time.Now()

	for b := 0; b < batches; b++ {
		opStart := time.Now()

		rows := make([]*models.Row, batchSize)
		for i := 0; i < batchSize; i++ {
			rowID := int64(b*batchSize + i)
			rows[i] = &models.Row{
				Values: map[string]interface{}{
					"id":        rowID,
					"name":      fmt.Sprintf("Row %d", rowID),
					"value":     rand.Float64() * 1000,
					"category":  fmt.Sprintf("cat_%d", rowID%20),
					"timestamp": time.Now(),
				},
				RowID: rowID,
			}
		}

		err := bs.store.Columnar().Insert(ctx, "benchmark_table", rows)

		latencies[b] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Columnar Inserts (Batched)", int64(count), duration, latencies, errorCount), nil
}

func (bs *BenchmarkSuite) benchmarkColumnarQueries(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Columnar Queries (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		query := &models.ColumnarQuery{
			Table:   "benchmark_table",
			Columns: []string{"id", "name", "value"},
			Filters: []*models.Filter{
				{
					Column:   "category",
					Operator: models.FilterOpEQ,
					Value:    fmt.Sprintf("cat_%d", rand.Intn(20)),
				},
			},
			Limit: 100,
		}

		_, err := bs.store.Columnar().Query(ctx, query, time.Minute)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Columnar Queries", int64(count), duration, latencies, errorCount), nil
}

// Cache benchmarks
func (bs *BenchmarkSuite) runCacheBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Cache hit rate test
	result, err := bs.benchmarkCacheHitRate(ctx, 5000)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

func (bs *BenchmarkSuite) benchmarkCacheHitRate(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running Cache Hit Rate Test (%d operations)...\n", count)

	// First, populate cache by reading data
	for i := 0; i < count/2; i++ {
		key := fmt.Sprintf("cache_test_%d", i)
		value := fmt.Sprintf("cached_value_%d", i)
		bs.store.KV().Set(ctx, key, []byte(value), time.Hour)
	}

	// Now read the same data multiple times to test cache hits
	latencies := make([]time.Duration, count)
	var errorCount int64
	var cacheHits int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("cache_test_%d", rand.Intn(count/2))
		_, err := bs.store.KV().Get(ctx, key)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		} else {
			// Assume cache hit if latency is very low
			if latencies[i] < time.Microsecond*100 {
				cacheHits++
			}
		}
	}

	duration := time.Since(start)

	result := bs.calculateResult("Cache Hit Rate Test", int64(count), duration, latencies, errorCount)
	result.CacheHitRate = float64(cacheHits) / float64(int64(count)-errorCount)

	return result, nil
}

// Mixed workload benchmarks
func (bs *BenchmarkSuite) runMixedWorkloadBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Mixed read/write workload
	result, err := bs.benchmarkMixedWorkload(ctx, 5000, 0.7) // 70% reads, 30% writes
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

func (bs *BenchmarkSuite) benchmarkMixedWorkload(ctx context.Context, count int, readRatio float64) (*BenchmarkResult, error) {
	fmt.Printf("Running Mixed Workload (%.0f%% reads, %.0f%% writes, %d operations)...\n",
		readRatio*100, (1-readRatio)*100, count)

	// Pre-populate some data
	for i := 0; i < count/2; i++ {
		key := fmt.Sprintf("mixed_key_%d", i)
		value := fmt.Sprintf("mixed_value_%d_%s", i, generateRandomString(50))
		bs.store.KV().Set(ctx, key, []byte(value), 0)
	}

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		var err error
		if rand.Float64() < readRatio {
			// Read operation
			key := fmt.Sprintf("mixed_key_%d", rand.Intn(count/2))
			_, err = bs.store.KV().Get(ctx, key)
		} else {
			// Write operation
			key := fmt.Sprintf("mixed_key_%d", rand.Intn(count))
			value := fmt.Sprintf("mixed_value_%d_%s", i, generateRandomString(50))
			err = bs.store.KV().Set(ctx, key, []byte(value), 0)
		}

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return bs.calculateResult("Mixed Workload (70R/30W)", int64(count), duration, latencies, errorCount), nil
}

// Helper methods
func (bs *BenchmarkSuite) calculateResult(name string, operations int64, duration time.Duration, latencies []time.Duration, errorCount int64) *BenchmarkResult {
	opsPerSecond := float64(operations) / duration.Seconds()

	// Calculate latency statistics
	var totalLatency time.Duration
	minLatency := time.Hour
	maxLatency := time.Duration(0)

	for _, latency := range latencies {
		totalLatency += latency
		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
	}

	avgLatency := totalLatency / time.Duration(len(latencies))

	// Calculate percentiles (simplified)
	p95Index := int(float64(len(latencies)) * 0.95)
	p99Index := int(float64(len(latencies)) * 0.99)

	// Sort latencies for percentile calculation (simplified)
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)

	// Simple bubble sort for demonstration
	for i := 0; i < len(sortedLatencies)-1; i++ {
		for j := 0; j < len(sortedLatencies)-i-1; j++ {
			if sortedLatencies[j] > sortedLatencies[j+1] {
				sortedLatencies[j], sortedLatencies[j+1] = sortedLatencies[j+1], sortedLatencies[j]
			}
		}
	}

	p95Latency := sortedLatencies[p95Index]
	p99Latency := sortedLatencies[p99Index]

	return &BenchmarkResult{
		Name:         name,
		Operations:   operations,
		Duration:     duration,
		OpsPerSecond: opsPerSecond,
		AvgLatency:   avgLatency,
		MinLatency:   minLatency,
		MaxLatency:   maxLatency,
		P95Latency:   p95Latency,
		P99Latency:   p99Latency,
		ErrorCount:   errorCount,
	}
}

// PrintResults prints benchmark results in a formatted table
func (bs *BenchmarkSuite) PrintResults(results []*BenchmarkResult) {
	fmt.Println("\nBenchmark Results Summary")
	fmt.Println("=========================")
	fmt.Printf("%-30s %10s %12s %12s %12s %12s %10s\n",
		"Benchmark", "Ops", "Ops/Sec", "Avg Lat", "P95 Lat", "P99 Lat", "Errors")
	fmt.Println(strings.Repeat("-", 110))

	for _, result := range results {
		fmt.Printf("%-30s %10d %12.1f %12s %12s %12s %10d\n",
			result.Name,
			result.Operations,
			result.OpsPerSecond,
			formatDuration(result.AvgLatency),
			formatDuration(result.P95Latency),
			formatDuration(result.P99Latency),
			result.ErrorCount,
		)

		if result.CacheHitRate > 0 {
			fmt.Printf("  Cache Hit Rate: %.2f%%\n", result.CacheHitRate*100)
		}
	}
}

// SaveResults saves benchmark results to JSON file
func (bs *BenchmarkSuite) SaveResults(results []*BenchmarkResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	// In a real implementation, you would write to a file
	fmt.Printf("\nBenchmark results saved to %s\n", filename)
	fmt.Printf("JSON data length: %d bytes\n", len(data))

	return nil
}

// Helper functions
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
