package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildConfigLoader provides utilities for loading build configuration
type BuildConfigLoader struct {
	defaultConfigPaths []string
	envPrefix          string
}

// NewBuildConfigLoader creates a new build configuration loader
func NewBuildConfigLoader() *BuildConfigLoader {
	return &BuildConfigLoader{
		defaultConfigPaths: []string{
			"build.config.yaml",
			"build.config.yml",
			".mantisdb/build.config.yaml",
			".mantisdb/build.config.yml",
			"configs/build.yaml",
			"configs/build.yml",
		},
		envPrefix: "BUILD_",
	}
}

// LoadConfig loads build configuration with the following precedence:
// 1. Explicit config file path (if provided)
// 2. Default config file locations
// 3. Environment variables
func (l *BuildConfigLoader) LoadConfig(configPath string) (*BuildConfig, error) {
	var actualConfigPath string

	// If explicit path provided, use it
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("specified config file not found: %s", configPath)
		}
		actualConfigPath = configPath
	} else {
		// Try default locations
		actualConfigPath = l.findDefaultConfig()
	}

	// Load configuration
	config, err := LoadBuildConfig(actualConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load build config: %w", err)
	}

	return config, nil
}

// findDefaultConfig searches for a config file in default locations
func (l *BuildConfigLoader) findDefaultConfig() string {
	for _, path := range l.defaultConfigPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "" // No config file found, will use defaults
}

// ValidateEnvironment validates that required tools and environment are available
func (l *BuildConfigLoader) ValidateEnvironment(config *BuildConfig) error {
	var missingTools []string

	// Check required tools
	for _, tool := range config.Environment.RequiredTools {
		if !l.isToolAvailable(tool) {
			missingTools = append(missingTools, tool)
		}
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("missing required tools: %v", missingTools)
	}

	// Validate paths
	if err := l.validatePaths(config); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Validate resources
	if err := l.validateResources(config); err != nil {
		return fmt.Errorf("resource validation failed: %w", err)
	}

	return nil
}

// isToolAvailable checks if a tool is available in PATH
func (l *BuildConfigLoader) isToolAvailable(tool string) bool {
	_, err := os.Stat(fmt.Sprintf("/usr/bin/%s", tool))
	if err == nil {
		return true
	}

	_, err = os.Stat(fmt.Sprintf("/usr/local/bin/%s", tool))
	if err == nil {
		return true
	}

	// Check if tool is in PATH
	path := os.Getenv("PATH")
	if path == "" {
		return false
	}

	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		toolPath := filepath.Join(dir, tool)
		if _, err := os.Stat(toolPath); err == nil {
			return true
		}
		// Also check with .exe extension on Windows
		if _, err := os.Stat(toolPath + ".exe"); err == nil {
			return true
		}
	}

	return false
}

// validatePaths validates that required paths exist or can be created
func (l *BuildConfigLoader) validatePaths(config *BuildConfig) error {
	paths := []string{
		config.Environment.Paths.WorkDir,
		config.Build.Output.Directory,
	}

	for _, path := range paths {
		if path == "" {
			continue
		}

		// Check if path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try to create it
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("cannot create directory %s: %w", path, err)
			}
		}
	}

	return nil
}

// validateResources validates resource requirements
func (l *BuildConfigLoader) validateResources(config *BuildConfig) error {
	// Validate memory requirement
	if config.Environment.Resources.MaxMemory != "" {
		if _, err := ParseSize(config.Environment.Resources.MaxMemory); err != nil {
			return fmt.Errorf("invalid max memory format: %s", config.Environment.Resources.MaxMemory)
		}
	}

	// Validate disk space requirement
	if config.Environment.Resources.DiskSpace != "" {
		if _, err := ParseSize(config.Environment.Resources.DiskSpace); err != nil {
			return fmt.Errorf("invalid disk space format: %s", config.Environment.Resources.DiskSpace)
		}
	}

	// Validate CPU requirement
	if config.Environment.Resources.MaxCPU < 0 {
		return fmt.Errorf("max CPU cannot be negative")
	}

	return nil
}

// CreateSampleConfig creates a sample configuration file
func (l *BuildConfigLoader) CreateSampleConfig(path string) error {
	config := DefaultBuildConfig()
	return config.SaveToFile(path)
}

// GetConfigInfo returns information about the current configuration
func (l *BuildConfigLoader) GetConfigInfo(config *BuildConfig) map[string]interface{} {
	return map[string]interface{}{
		"version":           config.Version,
		"platforms":         config.Build.Platforms,
		"optimization":      config.Build.Optimization.Level,
		"cgo_enabled":       config.Build.Optimization.CGOEnabled,
		"parallel_builds":   config.Build.Parallel,
		"output_directory":  config.Build.Output.Directory,
		"github_repository": config.Distribution.GitHub.Repository,
		"required_tools":    config.Environment.RequiredTools,
	}
}

// MergeConfigs merges multiple configurations with the later ones taking precedence
func (l *BuildConfigLoader) MergeConfigs(configs ...*BuildConfig) *BuildConfig {
	if len(configs) == 0 {
		return DefaultBuildConfig()
	}

	base := configs[0]
	for i := 1; i < len(configs); i++ {
		base = l.mergeConfig(base, configs[i])
	}

	return base
}

// mergeConfig merges two configurations with the second taking precedence
func (l *BuildConfigLoader) mergeConfig(base, override *BuildConfig) *BuildConfig {
	result := *base // Copy base

	// Override non-empty values
	if override.Version != "" {
		result.Version = override.Version
	}

	if len(override.Build.Platforms) > 0 {
		result.Build.Platforms = override.Build.Platforms
	}

	if override.Build.Optimization.Level != "" {
		result.Build.Optimization.Level = override.Build.Optimization.Level
	}

	if len(override.Build.Optimization.BuildFlags) > 0 {
		result.Build.Optimization.BuildFlags = override.Build.Optimization.BuildFlags
	}

	if len(override.Build.Optimization.LDFlags) > 0 {
		result.Build.Optimization.LDFlags = override.Build.Optimization.LDFlags
	}

	if override.Build.Frontend.BuildCommand != "" {
		result.Build.Frontend.BuildCommand = override.Build.Frontend.BuildCommand
	}

	if override.Build.Output.Directory != "" {
		result.Build.Output.Directory = override.Build.Output.Directory
	}

	if override.Distribution.GitHub.Repository != "" {
		result.Distribution.GitHub.Repository = override.Distribution.GitHub.Repository
	}

	// Merge environment variables
	if len(override.Environment.EnvVars) > 0 {
		if result.Environment.EnvVars == nil {
			result.Environment.EnvVars = make(map[string]string)
		}
		for k, v := range override.Environment.EnvVars {
			result.Environment.EnvVars[k] = v
		}
	}

	return &result
}

// PrintConfig prints the configuration in a human-readable format
func (l *BuildConfigLoader) PrintConfig(config *BuildConfig) {
	fmt.Printf("Build Configuration:\n")
	fmt.Printf("  Version: %s\n", config.Version)
	fmt.Printf("  Platforms: %v\n", config.Build.Platforms)
	fmt.Printf("  Optimization: %s (CGO: %t)\n", config.Build.Optimization.Level, config.Build.Optimization.CGOEnabled)
	fmt.Printf("  Parallel: %t\n", config.Build.Parallel)
	fmt.Printf("  Output: %s\n", config.Build.Output.Directory)
	fmt.Printf("  Frontend: %s -> %s\n", config.Build.Frontend.OutputDir, config.Build.Frontend.EmbedPath)
	fmt.Printf("  GitHub: %s\n", config.Distribution.GitHub.Repository)
	fmt.Printf("  Required Tools: %v\n", config.Environment.RequiredTools)
}
