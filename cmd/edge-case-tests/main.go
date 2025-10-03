package main

import (
	"flag"
	"fmt"
	"os"

	"mantisDB/testing"
)

func main() {
	var (
		configFile   = flag.String("config", "", "Configuration file path")
		outputFile   = flag.String("output", "", "Output file path")
		outputFormat = flag.String("format", "text", "Output format (text, json, html)")
		testType     = flag.String("test", "", "Specific test type to run (large-documents, high-ttl, concurrent-writes, memory-pressure)")
		verbose      = flag.Bool("verbose", false, "Verbose output")
		saveConfig   = flag.String("save-config", "", "Save default configuration to file")
	)
	flag.Parse()

	// Create test runner
	runner := testing.NewEdgeCaseTestRunner()

	// Save default config if requested
	if *saveConfig != "" {
		err := runner.SaveConfig(*saveConfig)
		if err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Default configuration saved to: %s\n", *saveConfig)
		return
	}

	// Load config if provided
	if *configFile != "" {
		err := runner.LoadConfig(*configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Configuration loaded from: %s\n", *configFile)
	}

	// Set output options
	if *outputFile != "" {
		runner.SetOutputFile(*outputFile)
	}
	if *outputFormat != "" {
		runner.SetOutputFormat(*outputFormat)
	}
	if *verbose {
		runner.SetVerbose(true)
	}

	// Run tests
	var err error
	if *testType != "" {
		fmt.Printf("Running specific test: %s\n", *testType)
		err = runner.RunSpecificTest(*testType)
	} else {
		fmt.Println("Running all edge case tests...")
		err = runner.RunAllTests()
	}

	if err != nil {
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Edge case tests completed successfully!")
}
