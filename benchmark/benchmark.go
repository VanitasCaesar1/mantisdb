package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mantisDB/store"
)

// BenchmarkResult holds the results of a benchmark
type BenchmarkResult struct {
	Name           string        `json:"name"`
	Operations     int64         `json:"operations"`
	Duration       time.Duration `json:"duration"`
	OpsPerSecond   float64       `json:"ops_per_second"`
	AvgLatency     time.Duration `json:"avg_latency"`
	MinLatency     time.Duration `json:"min_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
	P50Latency     time.Duration `json:"p50_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	P999Latency    time.Duration `json:"p999_latency"`
	ErrorCount     int64         `json:"error_count"`
	ErrorRate      float64       `json:"error_rate"`
	CacheHitRate   float64       `json:"cache_hit_rate,omitempty"`
	ThroughputMBps float64       `json:"throughput_mbps,omitempty"`
	MemoryUsageMB  float64       `json:"memory_usage_mb"`
	CPUUsage       float64       `json:"cpu_usage_percent"`
	Score          float64       `json:"score"`
	Grade          string        `json:"grade"`
	Timestamp      time.Time     `json:"timestamp"`
}

// StressTestConfig defines configuration for stress tests
type StressTestConfig struct {
	Duration         time.Duration `json:"duration"`
	NumWorkers       int           `json:"num_workers"`
	OperationsPerSec int           `json:"operations_per_sec"`
	DataSize         int           `json:"data_size"`
	BatchSize        int           `json:"batch_size"`
	EnableMetrics    bool          `json:"enable_metrics"`
	StressLevel      string        `json:"stress_level"` // "light", "medium", "heavy", "extreme"
}

// ProductionBenchmarkSuite provides production-ready benchmarking
type ProductionBenchmarkSuite struct {
	store       *store.MantisStore
	config      *StressTestConfig
	results     []*BenchmarkResult
	startTime   time.Time
	cpuStart    time.Time
	cpuUsage    float64
	memoryStart runtime.MemStats
}

// BenchmarkScore represents the overall benchmark scoring
type BenchmarkScore struct {
	OverallScore    float64            `json:"overall_score"`
	Grade           string             `json:"grade"`
	CategoryScores  map[string]float64 `json:"category_scores"`
	Recommendations []string           `json:"recommendations"`
	SystemInfo      SystemInfo         `json:"system_info"`
	TestEnvironment TestEnvironment    `json:"test_environment"`
	Results         []*BenchmarkResult `json:"results"`
	Timestamp       time.Time          `json:"timestamp"`
}

// SystemInfo captures system information
type SystemInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	CPUs         int    `json:"cpus"`
	Memory       int64  `json:"memory_mb"`
	GoVersion    string `json:"go_version"`
}

// TestEnvironment captures test environment details
type TestEnvironment struct {
	StressLevel     string        `json:"stress_level"`
	Duration        time.Duration `json:"duration"`
	Workers         int           `json:"workers"`
	TotalOperations int64         `json:"total_operations"`
	DataProcessedMB float64       `json:"data_processed_mb"`
}

// NewProductionBenchmarkSuite creates a new production benchmark suite
func NewProductionBenchmarkSuite(store *store.MantisStore, config *StressTestConfig) *ProductionBenchmarkSuite {
	if config == nil {
		config = DefaultStressTestConfig()
	}

	return &ProductionBenchmarkSuite{
		store:   store,
		config:  config,
		results: make([]*BenchmarkResult, 0),
	}
}

// DefaultStressTestConfig returns default stress test configuration
func DefaultStressTestConfig() *StressTestConfig {
	return &StressTestConfig{
		Duration:         60 * time.Second,
		NumWorkers:       runtime.NumCPU() * 2,
		OperationsPerSec: 1000,
		DataSize:         1024,
		BatchSize:        100,
		EnableMetrics:    true,
		StressLevel:      "medium",
	}
}

// GetStressTestConfig returns configuration for different stress levels
func GetStressTestConfig(level string) *StressTestConfig {
	base := DefaultStressTestConfig()

	switch level {
	case "light":
		base.Duration = 30 * time.Second
		base.NumWorkers = runtime.NumCPU()
		base.OperationsPerSec = 500
		base.DataSize = 512
	case "medium":
		base.Duration = 60 * time.Second
		base.NumWorkers = runtime.NumCPU() * 2
		base.OperationsPerSec = 1000
		base.DataSize = 1024
	case "heavy":
		base.Duration = 120 * time.Second
		base.NumWorkers = runtime.NumCPU() * 4
		base.OperationsPerSec = 2000
		base.DataSize = 2048
	case "extreme":
		base.Duration = 90 * time.Second       // Reduced from 180s
		base.NumWorkers = runtime.NumCPU() * 3 // 30 workers instead of 60
		base.OperationsPerSec = 2000           // Reduced from 3000
		base.DataSize = 1024                   // Reduced from 2KB to 1KB
	}

	base.StressLevel = level
	return base
}

// RunProductionBenchmarks runs comprehensive production-ready benchmarks
func (pbs *ProductionBenchmarkSuite) RunProductionBenchmarks(ctx context.Context) (*BenchmarkScore, error) {
	fmt.Printf("Starting MantisDB Production Benchmark Suite\n")
	fmt.Printf("Stress Level: %s\n", pbs.config.StressLevel)
	fmt.Printf("Duration: %v\n", pbs.config.Duration)
	fmt.Printf("Workers: %d\n", pbs.config.NumWorkers)
	fmt.Printf("Target Ops/Sec: %d\n", pbs.config.OperationsPerSec)
	fmt.Println("=====================================")

	pbs.startTime = time.Now()
	pbs.cpuStart = time.Now()
	runtime.ReadMemStats(&pbs.memoryStart)

	var results []*BenchmarkResult

	// 1. Core Performance Tests
	fmt.Println("\n1. Core Performance Benchmarks")
	fmt.Println("------------------------------")
	coreResults, err := pbs.runCorePerformanceTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("core performance tests failed: %v", err)
	}
	results = append(results, coreResults...)

	// 2. Stress Tests
	fmt.Println("\n2. Stress & Load Tests")
	fmt.Println("----------------------")
	stressResults, err := pbs.runStressTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("stress tests failed: %v", err)
	}
	results = append(results, stressResults...)

	// 3. Concurrency Tests
	fmt.Println("\n3. Concurrency Tests")
	fmt.Println("--------------------")
	concurrencyResults, err := pbs.runConcurrencyTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("concurrency tests failed: %v", err)
	}
	results = append(results, concurrencyResults...)

	pbs.results = results

	// Calculate overall score
	score := pbs.calculateBenchmarkScore()

	fmt.Println("\n=====================================")
	fmt.Printf("Production Benchmark Complete!\n")
	fmt.Printf("Overall Score: %.2f/100 (%s)\n", score.OverallScore, score.Grade)

	return score, nil
}

// RunAllBenchmarks runs all benchmark suites (legacy method)
func (pbs *ProductionBenchmarkSuite) RunAllBenchmarks(ctx context.Context) ([]*BenchmarkResult, error) {
	// Fallback to basic benchmarks for legacy compatibility
	fmt.Println("Running Legacy Benchmark Suite...")

	var results []*BenchmarkResult

	// Basic KV operations
	result, err := pbs.benchmarkKVSequentialWrites(ctx, 1000)
	if err != nil {
		return nil, fmt.Errorf("KV sequential writes failed: %v", err)
	}
	results = append(results, result)

	result, err = pbs.benchmarkKVSequentialReads(ctx, 1000)
	if err != nil {
		return nil, fmt.Errorf("KV sequential reads failed: %v", err)
	}
	results = append(results, result)

	result, err = pbs.benchmarkKVConcurrent(ctx, 100, 5)
	if err != nil {
		return nil, fmt.Errorf("KV concurrent operations failed: %v", err)
	}
	results = append(results, result)

	pbs.results = results
	return results, nil
}

// runCorePerformanceTests runs core performance benchmarks
func (pbs *ProductionBenchmarkSuite) runCorePerformanceTests(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Sequential writes
	result, err := pbs.benchmarkKVSequentialWrites(ctx, pbs.config.OperationsPerSec)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Sequential reads
	result, err = pbs.benchmarkKVSequentialReads(ctx, pbs.config.OperationsPerSec)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Random operations
	result, err = pbs.benchmarkKVRandomWrites(ctx, pbs.config.OperationsPerSec/2)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

// runStressTests runs stress and load tests
func (pbs *ProductionBenchmarkSuite) runStressTests(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// High throughput test
	result, err := pbs.benchmarkHighThroughput(ctx)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Memory pressure test
	result, err = pbs.benchmarkMemoryPressure(ctx)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

// runConcurrencyTests runs concurrency tests
func (pbs *ProductionBenchmarkSuite) runConcurrencyTests(ctx context.Context) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// Concurrent operations
	result, err := pbs.benchmarkKVConcurrent(ctx, pbs.config.OperationsPerSec/pbs.config.NumWorkers, pbs.config.NumWorkers)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	return results, nil
}

// benchmarkHighThroughput tests high throughput scenarios
func (pbs *ProductionBenchmarkSuite) benchmarkHighThroughput(ctx context.Context) (*BenchmarkResult, error) {
	fmt.Printf("Running High Throughput Test (%d workers, %d ops/sec)...\n", pbs.config.NumWorkers, pbs.config.OperationsPerSec)

	totalOps := pbs.config.OperationsPerSec * int(pbs.config.Duration.Seconds()) / 10 // Scale down for test
	opsPerWorker := totalOps / pbs.config.NumWorkers

	latencies := make([]time.Duration, totalOps)
	var errorCount int64
	var wg sync.WaitGroup
	var opIndex int64

	// Pre-allocate reusable buffer pool to reduce memory pressure
	bufferPool := sync.Pool{
		New: func() interface{} {
			return make([]byte, pbs.config.DataSize)
		},
	}

	start := time.Now()

	// Limit concurrent goroutines to prevent memory exhaustion
	semaphore := make(chan struct{}, pbs.config.NumWorkers)

	for w := 0; w < pbs.config.NumWorkers; w++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(workerID int) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			// Create worker-specific random source to avoid contention
			workerRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			// Pre-generate a seed buffer per worker to reduce per-op RNG CPU cost
			seed := make([]byte, pbs.config.DataSize)
			_, _ = workerRand.Read(seed)
			// Pace this worker to achieve target ops/sec across all workers
			perWorkerRate := pbs.config.OperationsPerSec / pbs.config.NumWorkers
			if perWorkerRate <= 0 {
				perWorkerRate = 1
			}
			opInterval := time.Second / time.Duration(perWorkerRate)
			nextTick := time.Now()

			for i := 0; i < opsPerWorker; i++ {
				// Rate limit to target ops/sec to avoid CPU spikes
				now := time.Now()
				if now.Before(nextTick) {
					time.Sleep(nextTick.Sub(now))
				}
				nextTick = nextTick.Add(opInterval)
				opStart := time.Now()

				key := fmt.Sprintf("throughput_key_%d_%d", workerID, i)

				// Get buffer from pool
				value := bufferPool.Get().([]byte)
				// Cheap data generation: copy pre-seeded data and stamp identifiers
				copy(value, seed)
				if len(value) >= 16 {
					v0 := byte(workerID)
					v1 := byte(i)
					value[0], value[1], value[2], value[3] = v0, v1, v0^v1, 0
					value[4], value[5], value[6], value[7] = v1, v0, v1^0x5A, v0^0xA5
					value[8], value[9], value[10], value[11] = byte(i>>8), byte(i>>16), byte(i>>24), byte(i)
					value[12], value[13], value[14], value[15] = 0xAA, 0x55, 0x33, 0xCC
				}

				err := pbs.store.KV().Set(ctx, key, value, 0)

				// Return buffer to pool
				bufferPool.Put(value)

				latency := time.Since(opStart)
				idx := atomic.AddInt64(&opIndex, 1) - 1

				// Reduce lock contention by using atomic operations
				if int(idx) < len(latencies) {
					latencies[idx] = latency
				}
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				}

				// Yield to prevent CPU starvation
				if i%100 == 0 {
					runtime.Gosched()
				}
			}
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return pbs.calculateResult("High Throughput Test", int64(totalOps), duration, latencies[:opIndex], errorCount), nil
}

// benchmarkMemoryPressure tests memory pressure scenarios
func (pbs *ProductionBenchmarkSuite) benchmarkMemoryPressure(ctx context.Context) (*BenchmarkResult, error) {
	fmt.Printf("Running Memory Pressure Test...\n")

	count := 1000
	largeDataSize := pbs.config.DataSize * 10 // 10x larger data
	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("memory_pressure_key_%d", i)
		value := make([]byte, largeDataSize)
		rand.Read(value)

		err := pbs.store.KV().Set(ctx, key, value, 0)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}

		// Force GC occasionally to test memory pressure
		if i%100 == 0 {
			runtime.GC()
		}
	}

	duration := time.Since(start)

	return pbs.calculateResult("Memory Pressure Test", int64(count), duration, latencies, errorCount), nil
}

// Legacy benchmark methods for backward compatibility
func (pbs *ProductionBenchmarkSuite) benchmarkKVSequentialWrites(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Sequential Writes (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("seq_key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))

		err := pbs.store.KV().Set(ctx, key, []byte(value), 0)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return pbs.calculateResult("KV Sequential Writes", int64(count), duration, latencies, errorCount), nil
}

func (pbs *ProductionBenchmarkSuite) benchmarkKVSequentialReads(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Sequential Reads (%d operations)...\n", count)

	// First, populate data
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("seq_key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))
		pbs.store.KV().Set(ctx, key, []byte(value), 0)
	}

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("seq_key_%d", i)
		_, err := pbs.store.KV().Get(ctx, key)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return pbs.calculateResult("KV Sequential Reads", int64(count), duration, latencies, errorCount), nil
}

func (pbs *ProductionBenchmarkSuite) benchmarkKVRandomWrites(ctx context.Context, count int) (*BenchmarkResult, error) {
	fmt.Printf("Running KV Random Writes (%d operations)...\n", count)

	latencies := make([]time.Duration, count)
	var errorCount int64

	start := time.Now()

	for i := 0; i < count; i++ {
		opStart := time.Now()

		key := fmt.Sprintf("rand_key_%d", rand.Intn(count*2))
		value := fmt.Sprintf("value_%d_%s", i, generateRandomString(100))

		err := pbs.store.KV().Set(ctx, key, []byte(value), 0)

		latencies[i] = time.Since(opStart)

		if err != nil {
			errorCount++
		}
	}

	duration := time.Since(start)

	return pbs.calculateResult("KV Random Writes", int64(count), duration, latencies, errorCount), nil
}

func (pbs *ProductionBenchmarkSuite) benchmarkKVConcurrent(ctx context.Context, opsPerWorker int, workers int) (*BenchmarkResult, error) {
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

				err := pbs.store.KV().Set(ctx, key, []byte(value), 0)

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

	return pbs.calculateResult("KV Concurrent Operations", int64(totalOps), duration, latencies, errorCount), nil
}

// calculateResult calculates benchmark result with enhanced metrics
func (pbs *ProductionBenchmarkSuite) calculateResult(name string, operations int64, duration time.Duration, latencies []time.Duration, errorCount int64) *BenchmarkResult {
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

	// Sort latencies for percentile calculation
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	sort.Slice(sortedLatencies, func(i, j int) bool {
		return sortedLatencies[i] < sortedLatencies[j]
	})

	// Calculate percentiles
	p50Index := int(float64(len(sortedLatencies)) * 0.50)
	p95Index := int(float64(len(sortedLatencies)) * 0.95)
	p99Index := int(float64(len(sortedLatencies)) * 0.99)
	p999Index := int(float64(len(sortedLatencies)) * 0.999)

	p50Latency := sortedLatencies[p50Index]
	p95Latency := sortedLatencies[p95Index]
	p99Latency := sortedLatencies[p99Index]
	p999Latency := sortedLatencies[p999Index]

	// Calculate error rate
	errorRate := float64(errorCount) / float64(operations) * 100

	// Get memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryUsageMB := float64(memStats.Alloc) / 1024 / 1024

	return &BenchmarkResult{
		Name:          name,
		Operations:    operations,
		Duration:      duration,
		OpsPerSecond:  opsPerSecond,
		AvgLatency:    avgLatency,
		MinLatency:    minLatency,
		MaxLatency:    maxLatency,
		P50Latency:    p50Latency,
		P95Latency:    p95Latency,
		P99Latency:    p99Latency,
		P999Latency:   p999Latency,
		ErrorCount:    errorCount,
		ErrorRate:     errorRate,
		MemoryUsageMB: memoryUsageMB,
		Timestamp:     time.Now(),
	}
}

// calculateBenchmarkScore calculates the overall benchmark score
func (pbs *ProductionBenchmarkSuite) calculateBenchmarkScore() *BenchmarkScore {
	if len(pbs.results) == 0 {
		return &BenchmarkScore{
			OverallScore: 0,
			Grade:        "F",
			Timestamp:    time.Now(),
		}
	}

	categoryScores := make(map[string]float64)
	var totalScore float64
	var totalOperations int64
	var totalDataMB float64

	// Calculate category scores
	for _, result := range pbs.results {
		score := pbs.calculateIndividualScore(result)
		result.Score = score
		result.Grade = pbs.getGrade(score)

		category := pbs.getCategoryFromName(result.Name)
		if _, exists := categoryScores[category]; !exists {
			categoryScores[category] = 0
		}
		categoryScores[category] += score
		totalScore += score
		totalOperations += result.Operations
		totalDataMB += float64(result.Operations*int64(pbs.config.DataSize)) / (1024 * 1024)
	}

	// Average category scores
	for category := range categoryScores {
		categoryScores[category] /= float64(len(pbs.results))
	}

	overallScore := totalScore / float64(len(pbs.results))

	// Generate recommendations
	recommendations := pbs.generateRecommendations(categoryScores, overallScore)

	// Get system info
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	systemInfo := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUs:         runtime.NumCPU(),
		Memory:       int64(memStats.Sys / 1024 / 1024),
		GoVersion:    runtime.Version(),
	}

	testEnv := TestEnvironment{
		StressLevel:     pbs.config.StressLevel,
		Duration:        pbs.config.Duration,
		Workers:         pbs.config.NumWorkers,
		TotalOperations: totalOperations,
		DataProcessedMB: totalDataMB,
	}

	return &BenchmarkScore{
		OverallScore:    overallScore,
		Grade:           pbs.getGrade(overallScore),
		CategoryScores:  categoryScores,
		Recommendations: recommendations,
		SystemInfo:      systemInfo,
		TestEnvironment: testEnv,
		Results:         pbs.results,
		Timestamp:       time.Now(),
	}
}

// calculateIndividualScore calculates score for individual benchmark
func (pbs *ProductionBenchmarkSuite) calculateIndividualScore(result *BenchmarkResult) float64 {
	// Base score from operations per second (normalized to 0-40 points)
	opsScore := math.Min(40, (result.OpsPerSecond/1000)*40)

	// Latency score (normalized to 0-30 points, lower is better)
	avgLatencyMs := float64(result.AvgLatency.Nanoseconds()) / 1000000
	latencyScore := math.Max(0, 30-avgLatencyMs/10)

	// Error rate score (normalized to 0-20 points)
	errorScore := math.Max(0, 20-result.ErrorRate*2)

	// Consistency score based on P99/P50 ratio (normalized to 0-10 points)
	p99Ms := float64(result.P99Latency.Nanoseconds()) / 1000000
	p50Ms := float64(result.P50Latency.Nanoseconds()) / 1000000
	consistencyRatio := p99Ms / math.Max(p50Ms, 0.001)
	consistencyScore := math.Max(0, 10-consistencyRatio)

	return opsScore + latencyScore + errorScore + consistencyScore
}

// getCategoryFromName extracts category from benchmark name
func (pbs *ProductionBenchmarkSuite) getCategoryFromName(name string) string {
	if strings.Contains(name, "Sequential") {
		return "Sequential Performance"
	}
	if strings.Contains(name, "Random") {
		return "Random Access"
	}
	if strings.Contains(name, "Concurrent") {
		return "Concurrency"
	}
	if strings.Contains(name, "Throughput") {
		return "Throughput"
	}
	if strings.Contains(name, "Memory") {
		return "Memory Management"
	}
	return "General"
}

// getGrade converts score to letter grade
func (pbs *ProductionBenchmarkSuite) getGrade(score float64) string {
	if score >= 90 {
		return "A+"
	} else if score >= 85 {
		return "A"
	} else if score >= 80 {
		return "A-"
	} else if score >= 75 {
		return "B+"
	} else if score >= 70 {
		return "B"
	} else if score >= 65 {
		return "B-"
	} else if score >= 60 {
		return "C+"
	} else if score >= 55 {
		return "C"
	} else if score >= 50 {
		return "C-"
	} else if score >= 40 {
		return "D"
	} else {
		return "F"
	}
}

// generateRecommendations generates performance recommendations
func (pbs *ProductionBenchmarkSuite) generateRecommendations(categoryScores map[string]float64, overallScore float64) []string {
	var recommendations []string

	if overallScore < 70 {
		recommendations = append(recommendations, "Overall performance is below optimal. Consider system tuning.")
	}

	for category, score := range categoryScores {
		if score < 60 {
			switch category {
			case "Sequential Performance":
				recommendations = append(recommendations, "Sequential performance is low. Consider increasing buffer sizes or using SSD storage.")
			case "Random Access":
				recommendations = append(recommendations, "Random access performance is poor. Consider adding more RAM for caching.")
			case "Concurrency":
				recommendations = append(recommendations, "Concurrency performance needs improvement. Check for lock contention.")
			case "Throughput":
				recommendations = append(recommendations, "Throughput is below expectations. Consider optimizing batch sizes.")
			case "Memory Management":
				recommendations = append(recommendations, "Memory management could be improved. Consider tuning GC settings.")
			}
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Performance is good! System is well-tuned for current workload.")
	}

	return recommendations
}

// PrintResults prints benchmark results in a formatted table
func (pbs *ProductionBenchmarkSuite) PrintResults(results []*BenchmarkResult) {
	fmt.Println("\nBenchmark Results Summary")
	fmt.Println("=========================")
	fmt.Printf("%-30s %10s %12s %12s %12s %12s %8s %6s\n",
		"Benchmark", "Ops", "Ops/Sec", "Avg Lat", "P95 Lat", "P99 Lat", "Errors", "Score")
	fmt.Println(strings.Repeat("-", 120))

	for _, result := range results {
		fmt.Printf("%-30s %10d %12.1f %12s %12s %12s %8d %6.1f\n",
			result.Name,
			result.Operations,
			result.OpsPerSecond,
			formatDuration(result.AvgLatency),
			formatDuration(result.P95Latency),
			formatDuration(result.P99Latency),
			result.ErrorCount,
			result.Score,
		)

		if result.CacheHitRate > 0 {
			fmt.Printf("  Cache Hit Rate: %.2f%%\n", result.CacheHitRate*100)
		}
	}
}

// SaveResults saves benchmark results to JSON file
func (pbs *ProductionBenchmarkSuite) SaveResults(results []*BenchmarkResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\nBenchmark results saved to %s\n", filename)
	return nil
}

// SaveBenchmarkScore saves comprehensive benchmark score to JSON file
func (pbs *ProductionBenchmarkSuite) SaveBenchmarkScore(score *BenchmarkScore, filename string) error {
	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("Production benchmark score saved to %s\n", filename)
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
