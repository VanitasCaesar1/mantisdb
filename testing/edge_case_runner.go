package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mantisDB/cache"
	"mantisDB/storage"
	"mantisDB/store"
)

// EdgeCaseTestRunner provides a command-line interface for running edge case tests
type EdgeCaseTestRunner struct {
	config EdgeCaseTestConfig
}

// EdgeCaseTestConfig holds configuration for edge case tests
type EdgeCaseTestConfig struct {
	StorageConfig storage.StorageConfig `json:"storage_config"`
	CacheConfig   cache.CacheConfig     `json:"cache_config"`
	TestConfig    TestConfig            `json:"test_config"`
	OutputFormat  string                `json:"output_format"` // "json", "text", "html"
	OutputFile    string                `json:"output_file"`
	Verbose       bool                  `json:"verbose"`
}

// TestConfig holds specific test configuration
type TestConfig struct {
	LargeDocumentSizes   []int64       `json:"large_document_sizes"`
	HighTTLValues        []int64       `json:"high_ttl_values"` // in seconds
	ConcurrencyLevels    []int         `json:"concurrency_levels"`
	MemoryPressureLevels []float64     `json:"memory_pressure_levels"`
	TimeoutDuration      time.Duration `json:"timeout_duration"`
}

// NewEdgeCaseTestRunner creates a new test runner with default configuration
func NewEdgeCaseTestRunner() *EdgeCaseTestRunner {
	return &EdgeCaseTestRunner{
		config: EdgeCaseTestConfig{
			StorageConfig: storage.StorageConfig{
				DataDir:    "./test_data_edge_cases",
				BufferSize: 1024 * 1024,
				CacheSize:  50 * 1024 * 1024, // 50MB
				UseCGO:     false,
				SyncWrites: true,
			},
			CacheConfig: cache.CacheConfig{
				MaxSize:         50 * 1024 * 1024, // 50MB
				DefaultTTL:      time.Hour,
				CleanupInterval: time.Minute * 5,
				EvictionPolicy:  "lru",
			},
			TestConfig: TestConfig{
				LargeDocumentSizes:   []int64{1024 * 1024, 5 * 1024 * 1024, 10 * 1024 * 1024},
				HighTTLValues:        []int64{25 * 3600, 7 * 24 * 3600, 30 * 24 * 3600},
				ConcurrencyLevels:    []int{10, 50, 100},
				MemoryPressureLevels: []float64{0.5, 0.7, 0.9},
				TimeoutDuration:      time.Minute * 30,
			},
			OutputFormat: "text",
			Verbose:      false,
		},
	}
}

// LoadConfig loads configuration from a JSON file
func (runner *EdgeCaseTestRunner) LoadConfig(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	err = json.Unmarshal(data, &runner.config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	return nil
}

// SaveConfig saves the current configuration to a JSON file
func (runner *EdgeCaseTestRunner) SaveConfig(configFile string) error {
	data, err := json.MarshalIndent(runner.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// RunAllTests runs all edge case tests
func (runner *EdgeCaseTestRunner) RunAllTests() error {
	fmt.Println("Starting Edge Case Test Suite")
	fmt.Println("============================")

	// Initialize storage and cache
	storageEngine := storage.NewPureGoStorageEngine(runner.config.StorageConfig)
	cacheManager := cache.NewCacheManager(runner.config.CacheConfig)
	mantisStore := store.NewMantisStore(storageEngine, cacheManager)

	// Create test suite
	testSuite := NewEdgeCaseTestSuite(mantisStore)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), runner.config.TestConfig.TimeoutDuration)
	defer cancel()

	// Run tests
	startTime := time.Now()
	results, err := testSuite.RunAllTests(ctx)
	if err != nil {
		return fmt.Errorf("test suite failed: %v", err)
	}

	// Output results
	err = runner.outputResults(results)
	if err != nil {
		return fmt.Errorf("failed to output results: %v", err)
	}

	// Print summary
	runner.printSummary(results, time.Since(startTime))

	return nil
}

// RunSpecificTest runs a specific test category
func (runner *EdgeCaseTestRunner) RunSpecificTest(testType string) error {
	// Initialize storage and cache
	storageEngine := storage.NewPureGoStorageEngine(runner.config.StorageConfig)
	cacheManager := cache.NewCacheManager(runner.config.CacheConfig)
	mantisStore := store.NewMantisStore(storageEngine, cacheManager)

	ctx, cancel := context.WithTimeout(context.Background(), runner.config.TestConfig.TimeoutDuration)
	defer cancel()

	var result *TestResult
	var err error

	switch testType {
	case "large-documents":
		tester := NewLargeDocumentTester(mantisStore)
		result, err = tester.RunTests(ctx)
	case "high-ttl":
		tester := NewHighTTLTester(mantisStore)
		result, err = tester.RunTests(ctx)
	case "concurrent-writes":
		tester := NewConcurrentWriteTester(mantisStore)
		result, err = tester.RunTests(ctx)
	case "memory-pressure":
		tester := NewMemoryPressureTester(mantisStore)
		result, err = tester.RunTests(ctx)
	default:
		return fmt.Errorf("unknown test type: %s", testType)
	}

	if err != nil {
		return fmt.Errorf("test failed: %v", err)
	}

	// Create results wrapper
	results := &TestResults{
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  result.Duration,
		Tests:     map[string]*TestResult{testType: result},
	}

	// Output results
	err = runner.outputResults(results)
	if err != nil {
		return fmt.Errorf("failed to output results: %v", err)
	}

	return nil
}

// outputResults outputs test results in the specified format
func (runner *EdgeCaseTestRunner) outputResults(results *TestResults) error {
	switch runner.config.OutputFormat {
	case "json":
		return runner.outputJSON(results)
	case "html":
		return runner.outputHTML(results)
	default:
		return runner.outputText(results)
	}
}

// outputJSON outputs results in JSON format
func (runner *EdgeCaseTestRunner) outputJSON(results *TestResults) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %v", err)
	}

	if runner.config.OutputFile != "" {
		err = os.WriteFile(runner.config.OutputFile, data, 0644)
		if err != nil {
			return fmt.Errorf("failed to write results file: %v", err)
		}
		fmt.Printf("Results written to: %s\n", runner.config.OutputFile)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

// outputText outputs results in human-readable text format
func (runner *EdgeCaseTestRunner) outputText(results *TestResults) error {
	output := runner.formatTextResults(results)

	if runner.config.OutputFile != "" {
		err := os.WriteFile(runner.config.OutputFile, []byte(output), 0644)
		if err != nil {
			return fmt.Errorf("failed to write results file: %v", err)
		}
		fmt.Printf("Results written to: %s\n", runner.config.OutputFile)
	} else {
		fmt.Print(output)
	}

	return nil
}

// outputHTML outputs results in HTML format
func (runner *EdgeCaseTestRunner) outputHTML(results *TestResults) error {
	html := runner.formatHTMLResults(results)

	if runner.config.OutputFile != "" {
		err := os.WriteFile(runner.config.OutputFile, []byte(html), 0644)
		if err != nil {
			return fmt.Errorf("failed to write results file: %v", err)
		}
		fmt.Printf("Results written to: %s\n", runner.config.OutputFile)
	} else {
		fmt.Print(html)
	}

	return nil
}

// formatTextResults formats results as human-readable text
func (runner *EdgeCaseTestRunner) formatTextResults(results *TestResults) string {
	output := "\nEdge Case Test Results\n"
	output += "======================\n"
	output += fmt.Sprintf("Start Time: %s\n", results.StartTime.Format(time.RFC3339))
	output += fmt.Sprintf("End Time: %s\n", results.EndTime.Format(time.RFC3339))
	output += fmt.Sprintf("Total Duration: %v\n\n", results.Duration)

	for testName, testResult := range results.Tests {
		output += runner.formatTestResult(testName, testResult, 0)
	}

	return output
}

// formatTestResult formats a single test result
func (runner *EdgeCaseTestRunner) formatTestResult(name string, result *TestResult, indent int) string {
	indentStr := ""
	for i := 0; i < indent; i++ {
		indentStr += "  "
	}

	status := "✅ PASS"
	if !result.Success {
		status = "❌ FAIL"
	}

	output := fmt.Sprintf("%s%s %s (Duration: %v)\n", indentStr, status, name, result.Duration)

	if result.Error != nil {
		output += fmt.Sprintf("%s  Error: %v\n", indentStr, result.Error)
	}

	if runner.config.Verbose && len(result.Metrics) > 0 {
		output += fmt.Sprintf("%s  Metrics:\n", indentStr)
		for key, value := range result.Metrics {
			output += fmt.Sprintf("%s    %s: %v\n", indentStr, key, value)
		}
	}

	// Format sub-tests
	for subName, subResult := range result.SubTests {
		output += runner.formatTestResult(subName, subResult, indent+1)
	}

	return output
}

// formatHTMLResults formats results as HTML
func (runner *EdgeCaseTestRunner) formatHTMLResults(results *TestResults) string {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Edge Case Test Results</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f0f0; padding: 10px; border-radius: 5px; }
        .test-result { margin: 10px 0; padding: 10px; border-left: 3px solid #ccc; }
        .pass { border-left-color: #4CAF50; }
        .fail { border-left-color: #f44336; }
        .metrics { background-color: #f9f9f9; padding: 5px; margin: 5px 0; }
        .sub-test { margin-left: 20px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Edge Case Test Results</h1>
        <p><strong>Start Time:</strong> ` + results.StartTime.Format(time.RFC3339) + `</p>
        <p><strong>End Time:</strong> ` + results.EndTime.Format(time.RFC3339) + `</p>
        <p><strong>Total Duration:</strong> ` + results.Duration.String() + `</p>
    </div>
`

	for testName, testResult := range results.Tests {
		html += runner.formatHTMLTestResult(testName, testResult, 0)
	}

	html += `
</body>
</html>`

	return html
}

// formatHTMLTestResult formats a single test result as HTML
func (runner *EdgeCaseTestRunner) formatHTMLTestResult(name string, result *TestResult, level int) string {
	status := "pass"
	statusText := "✅ PASS"
	if !result.Success {
		status = "fail"
		statusText = "❌ FAIL"
	}

	class := fmt.Sprintf("test-result %s", status)
	if level > 0 {
		class += " sub-test"
	}

	html := fmt.Sprintf(`<div class="%s">`, class)
	html += fmt.Sprintf(`<h%d>%s %s</h%d>`, level+2, statusText, name, level+2)
	html += fmt.Sprintf(`<p><strong>Duration:</strong> %v</p>`, result.Duration)

	if result.Error != nil {
		html += fmt.Sprintf(`<p><strong>Error:</strong> %v</p>`, result.Error)
	}

	if runner.config.Verbose && len(result.Metrics) > 0 {
		html += `<div class="metrics"><strong>Metrics:</strong><ul>`
		for key, value := range result.Metrics {
			html += fmt.Sprintf(`<li>%s: %v</li>`, key, value)
		}
		html += `</ul></div>`
	}

	// Format sub-tests
	for subName, subResult := range result.SubTests {
		html += runner.formatHTMLTestResult(subName, subResult, level+1)
	}

	html += `</div>`
	return html
}

// printSummary prints a summary of test results
func (runner *EdgeCaseTestRunner) printSummary(results *TestResults, totalDuration time.Duration) {
	fmt.Println("\nTest Summary")
	fmt.Println("============")

	totalTests := 0
	passedTests := 0
	failedTests := 0

	for _, testResult := range results.Tests {
		totalTests++
		if testResult.Success {
			passedTests++
		} else {
			failedTests++
		}

		// Count sub-tests
		for _, subResult := range testResult.SubTests {
			totalTests++
			if subResult.Success {
				passedTests++
			} else {
				failedTests++
			}
		}
	}

	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d\n", passedTests)
	fmt.Printf("Failed: %d\n", failedTests)
	fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	fmt.Printf("Total Duration: %v\n", totalDuration)

	if failedTests > 0 {
		fmt.Printf("\n⚠️  %d tests failed. Check the detailed results above.\n", failedTests)
	} else {
		fmt.Printf("\n🎉 All tests passed!\n")
	}
}

// SetOutputFile sets the output file path
func (runner *EdgeCaseTestRunner) SetOutputFile(outputFile string) {
	runner.config.OutputFile = outputFile
}

// SetOutputFormat sets the output format
func (runner *EdgeCaseTestRunner) SetOutputFormat(format string) {
	runner.config.OutputFormat = format
}

// SetVerbose sets verbose mode
func (runner *EdgeCaseTestRunner) SetVerbose(verbose bool) {
	runner.config.Verbose = verbose
}
