package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mantisDB/benchmark"
	"mantisDB/cache"
	"mantisDB/storage"
	"mantisDB/store"
)

func main() {
	var (
		stressLevel        = flag.String("stress", "medium", "Stress level: light, medium, heavy, extreme")
		duration           = flag.Duration("duration", 0, "Override test duration")
		workers            = flag.Int("workers", 0, "Override number of workers")
		opsPerSec          = flag.Int("ops", 0, "Override operations per second")
		dataDir            = flag.String("data-dir", "./stress_test_data", "Data directory for tests")
		outputFile         = flag.String("output", "stress_benchmark_results.json", "Output file for results")
		useCGO             = flag.Bool("use-cgo", false, "Use CGO storage engine")
		showHelp           = flag.Bool("help", false, "Show help")
		continuousMode     = flag.Bool("continuous", false, "Run continuous stress testing")
		continuousInterval = flag.Duration("interval", 5*time.Minute, "Interval between continuous tests")
	)

	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	fmt.Printf("MantisDB Production Stress Benchmark Tool\n")
	fmt.Printf("=========================================\n")
	fmt.Printf("Stress Level: %s\n", *stressLevel)
	fmt.Printf("Data Directory: %s\n", *dataDir)
	fmt.Printf("Output File: %s\n", *outputFile)
	fmt.Printf("Use CGO: %v\n", *useCGO)
	fmt.Printf("Continuous Mode: %v\n", *continuousMode)
	if *continuousMode {
		fmt.Printf("Continuous Interval: %v\n", *continuousInterval)
	}
	fmt.Println()

	// Create data directory
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Get stress test configuration
	config := benchmark.GetStressTestConfig(*stressLevel)

	// Override configuration if specified
	if *duration > 0 {
		config.Duration = *duration
	}
	if *workers > 0 {
		config.NumWorkers = *workers
	}
	if *opsPerSec > 0 {
		config.OperationsPerSec = *opsPerSec
	}

	if *continuousMode {
		runContinuousStressTesting(config, *dataDir, *outputFile, *useCGO, *continuousInterval)
	} else {
		runSingleStressTest(config, *dataDir, *outputFile, *useCGO)
	}
}

func runSingleStressTest(config *benchmark.StressTestConfig, dataDir, outputFile string, useCGO bool) {
	fmt.Printf("Starting single stress test run...\n")

	// Initialize storage and store
	store, cleanup, err := initializeStore(dataDir, useCGO)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer cleanup()

	// Create benchmark suite
	suite := benchmark.NewProductionBenchmarkSuite(store, config)

	// Run benchmarks
	ctx := context.Background()
	score, err := suite.RunProductionBenchmarks(ctx)
	if err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}

	// Print results
	printComprehensiveResults(score)

	// Save results
	if err := suite.SaveBenchmarkScore(score, outputFile); err != nil {
		log.Printf("Failed to save results: %v", err)
	}

	fmt.Printf("\nStress test completed successfully!\n")
	fmt.Printf("Results saved to: %s\n", outputFile)
}

func runContinuousStressTesting(config *benchmark.StressTestConfig, dataDir, outputFile string, useCGO bool, interval time.Duration) {
	fmt.Printf("Starting continuous stress testing (interval: %v)...\n", interval)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	runCount := 0

	for {
		runCount++
		fmt.Printf("=== Continuous Run #%d ===\n", runCount)

		// Initialize fresh storage for each run
		store, cleanup, err := initializeStore(dataDir, useCGO)
		if err != nil {
			log.Printf("Failed to initialize store for run %d: %v", runCount, err)
			time.Sleep(interval)
			continue
		}

		// Create benchmark suite
		suite := benchmark.NewProductionBenchmarkSuite(store, config)

		// Run benchmarks
		ctx := context.Background()
		score, err := suite.RunProductionBenchmarks(ctx)
		if err != nil {
			log.Printf("Benchmark run %d failed: %v", runCount, err)
			cleanup()
			time.Sleep(interval)
			continue
		}

		// Print summary results
		printContinuousRunSummary(runCount, score)

		// Save results with run number
		runOutputFile := fmt.Sprintf("continuous_run_%d_%s", runCount, outputFile)
		if err := suite.SaveBenchmarkScore(score, runOutputFile); err != nil {
			log.Printf("Failed to save results for run %d: %v", runCount, err)
		}

		cleanup()

		// Wait for next run
		fmt.Printf("Waiting %v until next run...\n\n", interval)
		time.Sleep(interval)
	}
}

func initializeStore(dataDir string, useCGO bool) (*store.MantisStore, func(), error) {
	// Parse sizes
	cacheSize := int64(256 * 1024 * 1024) // 256MB
	bufferSize := int64(64 * 1024 * 1024) // 64MB

	// Initialize storage engine
	storageConfig := storage.StorageConfig{
		DataDir:    dataDir,
		BufferSize: bufferSize,
		CacheSize:  cacheSize,
		UseCGO:     useCGO,
		SyncWrites: false, // Async for better performance in stress tests
	}

	var storageEngine storage.StorageEngine
	if useCGO {
		storageEngine = storage.NewCGOStorageEngine(storageConfig)
	} else {
		storageEngine = storage.NewPureGoStorageEngine(storageConfig)
	}

	// Initialize storage engine
	if err := storageEngine.Init(dataDir); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize storage engine: %v", err)
	}

	// Initialize cache manager
	cacheConfig := cache.CacheConfig{
		MaxSize:         cacheSize,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute * 5,
		EvictionPolicy:  "lru",
	}
	cacheManager := cache.NewCacheManager(cacheConfig)

	// Create unified store
	mantisStore := store.NewMantisStore(storageEngine, cacheManager)

	cleanup := func() {
		if storageEngine != nil {
			storageEngine.Close()
		}
	}

	return mantisStore, cleanup, nil
}

func printComprehensiveResults(score *benchmark.BenchmarkScore) {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("MANTISDB PRODUCTION STRESS BENCHMARK RESULTS\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n")

	// Overall Score
	fmt.Printf("Overall Performance Score: %.2f/100 (%s)\n", score.OverallScore, score.Grade)

	// System Information
	fmt.Printf("\nSystem Information:\n")
	fmt.Printf("  OS/Architecture: %s/%s\n", score.SystemInfo.OS, score.SystemInfo.Architecture)
	fmt.Printf("  CPUs: %d\n", score.SystemInfo.CPUs)
	fmt.Printf("  Memory: %d MB\n", score.SystemInfo.Memory)
	fmt.Printf("  Go Version: %s\n", score.SystemInfo.GoVersion)

	// Test Environment
	fmt.Printf("\nTest Environment:\n")
	fmt.Printf("  Stress Level: %s\n", score.TestEnvironment.StressLevel)
	fmt.Printf("  Duration: %v\n", score.TestEnvironment.Duration)
	fmt.Printf("  Workers: %d\n", score.TestEnvironment.Workers)
	fmt.Printf("  Total Operations: %d\n", score.TestEnvironment.TotalOperations)
	fmt.Printf("  Data Processed: %.2f MB\n", score.TestEnvironment.DataProcessedMB)

	// Category Scores
	fmt.Printf("\nCategory Performance Scores:\n")
	for category, categoryScore := range score.CategoryScores {
		grade := getGradeFromScore(categoryScore)
		fmt.Printf("  %-25s: %6.2f/100 (%s)\n", category, categoryScore, grade)
	}

	// Individual Test Results
	fmt.Printf("\nDetailed Test Results:\n")
	fmt.Printf("%-30s %10s %12s %12s %12s %8s %6s\n",
		"Test Name", "Ops", "Ops/Sec", "Avg Lat", "P99 Lat", "Errors", "Score")
	fmt.Printf(strings.Repeat("-", 90) + "\n")

	for _, result := range score.Results {
		fmt.Printf("%-30s %10d %12.1f %12s %12s %8d %6.1f\n",
			truncateString(result.Name, 30),
			result.Operations,
			result.OpsPerSecond,
			formatDuration(result.AvgLatency),
			formatDuration(result.P99Latency),
			result.ErrorCount,
			result.Score,
		)
	}

	// Recommendations
	if len(score.Recommendations) > 0 {
		fmt.Printf("\nPerformance Recommendations:\n")
		for i, rec := range score.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, rec)
		}
	}

	// Performance Analysis
	fmt.Printf("\nPerformance Analysis:\n")
	analyzePerformance(score)

	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
}

func printContinuousRunSummary(runNumber int, score *benchmark.BenchmarkScore) {
	fmt.Printf("Run #%d Summary:\n", runNumber)
	fmt.Printf("  Overall Score: %.2f/100 (%s)\n", score.OverallScore, score.Grade)
	fmt.Printf("  Total Operations: %d\n", score.TestEnvironment.TotalOperations)
	fmt.Printf("  Data Processed: %.2f MB\n", score.TestEnvironment.DataProcessedMB)

	// Show top performing and worst performing tests
	if len(score.Results) > 0 {
		bestResult := score.Results[0]
		worstResult := score.Results[0]

		for _, result := range score.Results {
			if result.Score > bestResult.Score {
				bestResult = result
			}
			if result.Score < worstResult.Score {
				worstResult = result
			}
		}

		fmt.Printf("  Best Test: %s (%.1f/100)\n", bestResult.Name, bestResult.Score)
		fmt.Printf("  Worst Test: %s (%.1f/100)\n", worstResult.Name, worstResult.Score)
	}

	fmt.Printf("  Timestamp: %s\n", score.Timestamp.Format("2006-01-02 15:04:05"))
}

func analyzePerformance(score *benchmark.BenchmarkScore) {
	// Throughput analysis
	totalOps := score.TestEnvironment.TotalOperations
	duration := score.TestEnvironment.Duration
	avgThroughput := float64(totalOps) / duration.Seconds()

	fmt.Printf("  Average Throughput: %.2f ops/sec\n", avgThroughput)

	// Memory efficiency
	dataProcessed := score.TestEnvironment.DataProcessedMB
	if len(score.Results) > 0 {
		avgMemoryUsage := 0.0
		for _, result := range score.Results {
			avgMemoryUsage += result.MemoryUsageMB
		}
		avgMemoryUsage /= float64(len(score.Results))

		memoryEfficiency := dataProcessed / avgMemoryUsage
		fmt.Printf("  Memory Efficiency: %.2f (data/memory ratio)\n", memoryEfficiency)
	}

	// Error analysis
	totalErrors := int64(0)
	for _, result := range score.Results {
		totalErrors += result.ErrorCount
	}
	errorRate := float64(totalErrors) / float64(totalOps) * 100
	fmt.Printf("  Overall Error Rate: %.4f%%\n", errorRate)

	// Performance classification
	if score.OverallScore >= 85 {
		fmt.Printf("  Classification: EXCELLENT - Production ready with high performance\n")
	} else if score.OverallScore >= 70 {
		fmt.Printf("  Classification: GOOD - Production ready with acceptable performance\n")
	} else if score.OverallScore >= 55 {
		fmt.Printf("  Classification: FAIR - May need tuning for production use\n")
	} else {
		fmt.Printf("  Classification: POOR - Requires significant optimization\n")
	}
}

func getGradeFromScore(score float64) string {
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

func showUsage() {
	fmt.Printf("MantisDB Production Stress Benchmark Tool\n")
	fmt.Printf("=========================================\n\n")
	fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
	fmt.Printf("Options:\n")
	flag.PrintDefaults()
	fmt.Printf("\nStress Levels:\n")
	fmt.Printf("  light    - 30s duration, CPU cores workers, 500 ops/sec, 512B data\n")
	fmt.Printf("  medium   - 60s duration, 2x CPU cores workers, 1000 ops/sec, 1KB data\n")
	fmt.Printf("  heavy    - 120s duration, 4x CPU cores workers, 2000 ops/sec, 2KB data\n")
	fmt.Printf("  extreme  - 300s duration, 8x CPU cores workers, 5000 ops/sec, 4KB data\n")
	fmt.Printf("\nExamples:\n")
	fmt.Printf("  %s -stress=heavy -data-dir=/tmp/stress_test\n", os.Args[0])
	fmt.Printf("  %s -stress=extreme -use-cgo -output=extreme_results.json\n", os.Args[0])
	fmt.Printf("  %s -continuous -interval=10m -stress=medium\n", os.Args[0])
	fmt.Printf("  %s -stress=light -duration=30s -workers=8 -ops=2000\n", os.Args[0])
	fmt.Printf("\n")
}
