package main

import (
	"flag"
	"fmt"
	"os"

	"mantisDB/testing"
)

func main() {
	// Command line flags
	var (
		configFile        = flag.String("config", "", "Path to configuration file")
		saveConfig        = flag.String("save-config", "", "Save default configuration to file")
		testType          = flag.String("test", "", "Specific test type to run (crash-recovery, disk-space, memory-limits, concurrent-access)")
		outputFile        = flag.String("output", "", "Output file path")
		outputFormat      = flag.String("format", "text", "Output format (text, json, html)")
		verbose           = flag.Bool("verbose", false, "Enable verbose output")
		enableCrash       = flag.Bool("enable-crash", true, "Enable crash recovery tests")
		enableDisk        = flag.Bool("enable-disk", true, "Enable disk space tests")
		enableMemory      = flag.Bool("enable-memory", true, "Enable memory limit tests")
		enableConcurrency = flag.Bool("enable-concurrency", true, "Enable concurrent access tests")
		help              = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Create test runner
	runner := testing.NewReliabilityTestRunner()

	// Load configuration if specified
	if *configFile != "" {
		err := runner.LoadConfig(*configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded configuration from: %s\n", *configFile)
	}

	// Save default configuration if requested
	if *saveConfig != "" {
		err := runner.SaveConfig(*saveConfig)
		if err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Default configuration saved to: %s\n", *saveConfig)
		return
	}

	// Apply command line options
	if *outputFile != "" {
		runner.SetOutputFile(*outputFile)
	}

	if *outputFormat != "" {
		runner.SetOutputFormat(*outputFormat)
	}

	if *verbose {
		runner.SetVerbose(true)
	}

	// Configure test categories
	runner.EnableTestCategory("crash", *enableCrash)
	runner.EnableTestCategory("disk", *enableDisk)
	runner.EnableTestCategory("memory", *enableMemory)
	runner.EnableTestCategory("concurrency", *enableConcurrency)

	// Run tests
	var err error
	if *testType != "" {
		// Run specific test type
		fmt.Printf("Running specific reliability test: %s\n", *testType)
		err = runner.RunSpecificTest(*testType)
	} else {
		// Run all tests
		fmt.Println("Running all reliability tests...")
		err = runner.RunAllTests()
	}

	if err != nil {
		fmt.Printf("Error running reliability tests: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Reliability tests completed successfully!")
}

func printHelp() {
	fmt.Println("MantisDB Reliability Test Suite")
	fmt.Println("===============================")
	fmt.Println()
	fmt.Println("This tool runs comprehensive reliability tests for MantisDB, including:")
	fmt.Println("- Crash recovery testing")
	fmt.Println("- Disk space exhaustion testing")
	fmt.Println("- Memory limit testing")
	fmt.Println("- Concurrent access pattern testing")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/reliability-tests/main.go [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string")
	fmt.Println("        Path to configuration file")
	fmt.Println("  -save-config string")
	fmt.Println("        Save default configuration to file")
	fmt.Println("  -test string")
	fmt.Println("        Specific test type to run:")
	fmt.Println("        - crash-recovery: Test crash recovery scenarios")
	fmt.Println("        - disk-space: Test disk space exhaustion handling")
	fmt.Println("        - memory-limits: Test memory limit scenarios")
	fmt.Println("        - concurrent-access: Test concurrent access patterns")
	fmt.Println("  -output string")
	fmt.Println("        Output file path (default: stdout)")
	fmt.Println("  -format string")
	fmt.Println("        Output format: text, json, html (default: text)")
	fmt.Println("  -verbose")
	fmt.Println("        Enable verbose output with detailed metrics")
	fmt.Println("  -enable-crash")
	fmt.Println("        Enable crash recovery tests (default: true)")
	fmt.Println("  -enable-disk")
	fmt.Println("        Enable disk space tests (default: true)")
	fmt.Println("  -enable-memory")
	fmt.Println("        Enable memory limit tests (default: true)")
	fmt.Println("  -enable-concurrency")
	fmt.Println("        Enable concurrent access tests (default: true)")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Run all reliability tests")
	fmt.Println("  go run cmd/reliability-tests/main.go")
	fmt.Println()
	fmt.Println("  # Run only crash recovery tests")
	fmt.Println("  go run cmd/reliability-tests/main.go -test crash-recovery")
	fmt.Println()
	fmt.Println("  # Run tests with custom configuration")
	fmt.Println("  go run cmd/reliability-tests/main.go -config my_config.json")
	fmt.Println()
	fmt.Println("  # Save results to JSON file")
	fmt.Println("  go run cmd/reliability-tests/main.go -output results.json -format json")
	fmt.Println()
	fmt.Println("  # Run with verbose output")
	fmt.Println("  go run cmd/reliability-tests/main.go -verbose")
	fmt.Println()
	fmt.Println("  # Generate default configuration file")
	fmt.Println("  go run cmd/reliability-tests/main.go -save-config reliability_config.json")
	fmt.Println()
	fmt.Println("  # Run only memory and concurrency tests")
	fmt.Println("  go run cmd/reliability-tests/main.go -enable-crash=false -enable-disk=false")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  The tool uses JSON configuration files to customize test parameters.")
	fmt.Println("  Use -save-config to generate a default configuration file that you can modify.")
	fmt.Println()
	fmt.Println("Test Categories:")
	fmt.Println("  - Crash Recovery: Tests system recovery after simulated crashes")
	fmt.Println("  - Disk Space: Tests behavior when disk space is exhausted")
	fmt.Println("  - Memory Limits: Tests memory pressure handling and limits")
	fmt.Println("  - Concurrent Access: Tests high-concurrency scenarios and deadlock detection")
	fmt.Println()
	fmt.Println("Note: Some reliability tests may intentionally cause failures to test")
	fmt.Println("error handling. Check the detailed results to understand test outcomes.")
}
