package compression

import (
	"fmt"
	"log"
	"time"
)

// Example demonstrates the compression system usage
func Example() {
	fmt.Println("=== MantisDB Compression System Example ===")

	// 1. Create compression manager with default configuration
	config := &CompressionManagerConfig{
		Enabled:                true,
		BackgroundCompression:  true,
		CompressionInterval:    30 * time.Second,
		MaxCandidatesPerCycle:  50,
		CompressionThreshold:   1024, // 1KB
		DecompressionCacheSize: 100,
	}

	manager := NewCompressionManager(config)
	defer manager.Shutdown()

	fmt.Println("✓ Compression manager initialized")

	// 2. Create sample data
	sampleData := []struct {
		key  string
		data []byte
	}{
		{"user_profile_1", generateSampleData(2048, "user profile data")},
		{"log_entry_1", generateSampleData(4096, "application log entry")},
		{"config_file", generateSampleData(1024, "configuration settings")},
		{"small_data", generateSampleData(512, "small data")}, // Below threshold
	}

	// 3. Write data (with compression)
	fmt.Println("\n--- Writing Data ---")
	for _, item := range sampleData {
		compressed, err := manager.Write(item.key, item.data)
		if err != nil {
			log.Printf("Error compressing %s: %v", item.key, err)
			continue
		}

		originalSize := len(item.data)
		compressedSize := len(compressed)
		ratio := float64(originalSize) / float64(compressedSize)

		fmt.Printf("Key: %s\n", item.key)
		fmt.Printf("  Original: %d bytes\n", originalSize)
		fmt.Printf("  Compressed: %d bytes\n", compressedSize)
		fmt.Printf("  Ratio: %.2f:1\n", ratio)
		fmt.Printf("  Savings: %d bytes (%.1f%%)\n\n",
			originalSize-compressedSize,
			float64(originalSize-compressedSize)/float64(originalSize)*100)
	}

	// 4. Simulate access patterns for cold data detection
	fmt.Println("--- Simulating Access Patterns ---")

	// Access some data frequently
	for i := 0; i < 10; i++ {
		manager.Read("user_profile_1", sampleData[0].data)
		time.Sleep(100 * time.Millisecond)
	}

	// Access other data less frequently
	manager.Read("log_entry_1", sampleData[1].data)
	manager.Read("config_file", sampleData[2].data)

	fmt.Println("✓ Access patterns simulated")

	// 5. Wait for background compression to process cold data
	fmt.Println("\n--- Waiting for Background Processing ---")
	time.Sleep(2 * time.Second)

	// 6. Get cold data candidates
	candidates := manager.GetColdDataCandidates(10)
	fmt.Printf("Found %d cold data candidates:\n", len(candidates))
	for _, candidate := range candidates {
		fmt.Printf("  Key: %s, Cold Score: %.2f, Last Accessed: %v\n",
			candidate.Key, candidate.ColdScore, candidate.LastAccessed.Format("15:04:05"))
	}

	// 7. Generate compression statistics
	fmt.Println("\n--- Compression Statistics ---")
	stats := manager.GetCompressionStats()

	if engineStats, ok := stats["engine"].(map[string]interface{}); ok {
		fmt.Printf("Total Compressed: %v bytes\n", engineStats["total_compressed"])
		fmt.Printf("Total Decompressed: %v bytes\n", engineStats["total_decompressed"])
		fmt.Printf("Compression Ratio: %.2f:1\n", engineStats["compression_ratio"])
	}

	if metricsData, ok := stats["metrics"].(CompressionMetrics); ok {
		fmt.Printf("Overall Ratio: %.2f:1\n", metricsData.OverallRatio)
		fmt.Printf("Compression Rate: %.2f MB/s\n", metricsData.CompressionRate)
		fmt.Printf("Decompression Rate: %.2f MB/s\n", metricsData.DecompressionRate)

		fmt.Println("\nAlgorithm Performance:")
		for algo, algoStats := range metricsData.Algorithms {
			fmt.Printf("  %s: %d compressions, %.2f:1 ratio\n",
				algo, algoStats.CompressionCount, algoStats.AverageRatio)
		}
	}

	// 8. Create reporter and generate dashboard
	fmt.Println("\n--- Compression Reporting ---")
	reporterConfig := &ReporterConfig{
		HTTPPort:            8090,
		ReportInterval:      time.Minute,
		RetentionPeriod:     time.Hour,
		EnableHTTPEndpoints: false, // Disable for example
		EnableFileReports:   false,
	}

	reporter := NewCompressionReporter(manager.engine.monitor, reporterConfig)
	defer reporter.Stop()

	dashboard := reporter.GenerateDashboard()
	fmt.Printf("Dashboard Overview:\n")
	fmt.Printf("  Total Data Processed: %s\n", dashboard.Overview.TotalDataProcessed)
	fmt.Printf("  Storage Savings: %s\n", dashboard.Overview.StorageSavings)
	fmt.Printf("  Average Compression Ratio: %.2f:1\n", dashboard.Overview.AverageCompressionRatio)
	fmt.Printf("  System Health: %s\n", dashboard.Overview.SystemHealth)
	fmt.Printf("  Active Alerts: %d\n", dashboard.Overview.ActiveAlerts)

	if len(dashboard.Recommendations) > 0 {
		fmt.Println("\nRecommendations:")
		for _, rec := range dashboard.Recommendations {
			fmt.Printf("  • %s\n", rec)
		}
	}

	fmt.Println("\n=== Example Complete ===")
}

// ExampleTransparentCompression demonstrates transparent compression usage
func ExampleTransparentCompression() {
	fmt.Println("=== Transparent Compression Example ===")

	// Create transparent compression with custom config
	config := &TransparentConfig{
		Enabled:            true,
		MinSize:            512, // Compress data > 512 bytes
		ColdThreshold:      time.Hour,
		DefaultAlgorithm:   "lz4",
		CompressionLevel:   1,
		BackgroundCompress: true,
	}

	tc := NewTransparentCompression(config)

	// Test data
	testData := []byte("This is a sample text that will be compressed transparently. " +
		"It contains repeated patterns that should compress well with LZ4 algorithm. " +
		"The compression system will automatically determine if this data should be compressed " +
		"based on the configured policies and thresholds.")

	fmt.Printf("Original data size: %d bytes\n", len(testData))

	// Write with compression
	compressed, err := tc.Write("test_key", testData)
	if err != nil {
		log.Fatalf("Compression failed: %v", err)
	}

	fmt.Printf("Compressed data size: %d bytes\n", len(compressed))
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(testData))/float64(len(compressed)))

	// Read with decompression
	decompressed, err := tc.Read("test_key", compressed)
	if err != nil {
		log.Fatalf("Decompression failed: %v", err)
	}

	fmt.Printf("Decompressed data size: %d bytes\n", len(decompressed))
	fmt.Printf("Data integrity: %t\n", string(testData) == string(decompressed))

	fmt.Println("✓ Transparent compression example complete")
}

// ExampleColdDataDetection demonstrates cold data detection
func ExampleColdDataDetection() {
	fmt.Println("=== Cold Data Detection Example ===")

	config := &ColdDataConfig{
		Enabled:              true,
		ColdThreshold:        5 * time.Second, // Short threshold for demo
		BloomFilterSize:      10000,
		BloomFilterHashCount: 3,
		AccessTrackingWindow: time.Minute,
		MinAccessCount:       2,
		SizeThreshold:        100,
		CleanupInterval:      30 * time.Second,
	}

	detector := NewColdDataDetector(config)

	// Simulate data access patterns
	keys := []string{"hot_data", "warm_data", "cold_data_1", "cold_data_2"}

	// Access hot data frequently
	for i := 0; i < 5; i++ {
		detector.RecordAccess("hot_data", 1024)
		time.Sleep(100 * time.Millisecond)
	}

	// Access warm data moderately
	for i := 0; i < 2; i++ {
		detector.RecordAccess("warm_data", 2048)
		time.Sleep(200 * time.Millisecond)
	}

	// Access cold data once, then wait
	detector.RecordAccess("cold_data_1", 4096)
	detector.RecordAccess("cold_data_2", 8192)

	// Wait for data to become "cold"
	fmt.Println("Waiting for data to become cold...")
	time.Sleep(6 * time.Second)

	// Check which data is considered cold
	for _, key := range keys {
		metadata := &DataMetadata{
			Size:         1024,
			LastAccessed: time.Now().Add(-10 * time.Second),
			AccessCount:  1,
		}

		isCold := detector.IsCold(key, metadata)
		fmt.Printf("Key: %s, Is Cold: %t\n", key, isCold)
	}

	// Get cold data candidates
	candidates := detector.GetColdDataCandidates(10)
	fmt.Printf("\nCold data candidates (%d found):\n", len(candidates))
	for _, candidate := range candidates {
		fmt.Printf("  Key: %s, Score: %.2f, Access Count: %d\n",
			candidate.Key, candidate.ColdScore, candidate.AccessCount)
	}

	fmt.Println("✓ Cold data detection example complete")
}

// generateSampleData generates sample data for testing
func generateSampleData(size int, pattern string) []byte {
	data := make([]byte, size)
	patternBytes := []byte(pattern)

	for i := 0; i < size; i++ {
		data[i] = patternBytes[i%len(patternBytes)]
	}

	return data
}

// RunAllExamples runs all compression examples
func RunAllExamples() {
	Example()
	fmt.Println()
	ExampleTransparentCompression()
	fmt.Println()
	ExampleColdDataDetection()
}
