package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"mantisDB/config"
)

func main() {
	var (
		configPath   = flag.String("config", "", "Path to build configuration file")
		validate     = flag.Bool("validate", false, "Validate configuration and environment")
		showDefaults = flag.Bool("defaults", false, "Show default configuration")
		createSample = flag.String("create-sample", "", "Create a sample configuration file")
		format       = flag.String("format", "yaml", "Output format (yaml|json)")
		envInfo      = flag.Bool("env-info", false, "Show environment variable information")
	)
	flag.Parse()

	loader := config.NewBuildConfigLoader()

	// Create sample configuration
	if *createSample != "" {
		if err := loader.CreateSampleConfig(*createSample); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating sample config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Sample configuration created at: %s\n", *createSample)
		return
	}

	// Show default configuration
	if *showDefaults {
		defaultConfig := config.DefaultBuildConfig()
		if *format == "json" {
			data, err := json.MarshalIndent(defaultConfig, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling to JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			if err := defaultConfig.SaveToFile("/dev/stdout"); err != nil {
				fmt.Fprintf(os.Stderr, "Error outputting YAML: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Show environment variable information
	if *envInfo {
		showEnvironmentInfo()
		return
	}

	// Load configuration
	buildConfig, err := loader.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration and environment
	if *validate {
		fmt.Println("Validating configuration...")
		if err := loader.ValidateEnvironment(buildConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Configuration and environment are valid")

		// Show configuration info
		fmt.Println("\nConfiguration Summary:")
		loader.PrintConfig(buildConfig)
		return
	}

	// Default: show current configuration
	if *format == "json" {
		data, err := json.MarshalIndent(buildConfig, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		loader.PrintConfig(buildConfig)
	}
}

func showEnvironmentInfo() {
	fmt.Println("Build Configuration Environment Variables:")
	fmt.Println()

	envVars := []struct {
		name        string
		description string
		example     string
	}{
		{"BUILD_VERSION", "Build version", "1.0.0"},
		{"BUILD_PLATFORMS", "Target platforms (comma-separated)", "linux/amd64,darwin/amd64"},
		{"BUILD_PARALLEL", "Enable parallel builds", "true"},
		{"BUILD_TIMEOUT", "Build timeout", "30m"},
		{"BUILD_OPTIMIZATION_LEVEL", "Optimization level", "size|speed|debug"},
		{"BUILD_CGO_ENABLED", "Enable CGO", "false"},
		{"BUILD_FLAGS", "Build flags (comma-separated)", "-trimpath,-race"},
		{"BUILD_LDFLAGS", "Linker flags (comma-separated)", "-s,-w"},
		{"BUILD_TAGS", "Build tags (comma-separated)", "production,release"},
		{"FRONTEND_BUILD_COMMAND", "Frontend build command", "npm run build"},
		{"FRONTEND_OUTPUT_DIR", "Frontend output directory", "admin/frontend/dist"},
		{"FRONTEND_EMBED_PATH", "Frontend embed path", "admin/api/assets"},
		{"FRONTEND_NODE_VERSION", "Node.js version", "18"},
		{"FRONTEND_SKIP_BUILD", "Skip frontend build", "false"},
		{"BUILD_OUTPUT_DIR", "Build output directory", "./dist"},
		{"BUILD_BINARY_PREFIX", "Binary name prefix", "mantisdb"},
		{"BUILD_VERSION_SUFFIX", "Add version suffix to binaries", "true"},
		{"BUILD_CLEAN_BEFORE", "Clean before build", "true"},
		{"PACKAGE_COMPRESSION_LEVEL", "Compression level (0-9)", "9"},
		{"PACKAGE_COMPRESSION_FORMAT", "Compression format", "auto|tar.gz|zip"},
		{"GITHUB_REPOSITORY", "GitHub repository", "owner/repo"},
		{"GITHUB_TOKEN", "GitHub token", "ghp_xxxxxxxxxxxx"},
		{"GITHUB_DRAFT", "Create draft release", "false"},
		{"GITHUB_PRERELEASE", "Mark as prerelease", "false"},
		{"GITHUB_RELEASE_NOTES", "Release notes", "auto|path/to/notes.md"},
		{"GITHUB_TAG_PREFIX", "Git tag prefix", "v"},
		{"BUILD_WORK_DIR", "Working directory", "."},
		{"BUILD_TEMP_DIR", "Temporary directory", "/tmp/mantisdb-build"},
		{"BUILD_CACHE_DIR", "Cache directory", "./.build-cache"},
		{"BUILD_MAX_MEMORY", "Maximum memory usage", "4GB"},
		{"BUILD_MAX_CPU", "Maximum CPU cores", "0"},
		{"BUILD_DISK_SPACE", "Required disk space", "10GB"},
	}

	for _, env := range envVars {
		fmt.Printf("  %-30s %s\n", env.name, env.description)
		if env.example != "" {
			fmt.Printf("  %-30s Example: %s\n", "", env.example)
		}
		fmt.Println()
	}
}
