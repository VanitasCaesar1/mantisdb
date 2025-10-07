package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultBuildConfig(t *testing.T) {
	config := DefaultBuildConfig()

	if config.Version == "" {
		t.Error("Default config should have a version")
	}

	if len(config.Build.Platforms) == 0 {
		t.Error("Default config should have platforms")
	}

	if config.Build.Output.BinaryPrefix == "" {
		t.Error("Default config should have binary prefix")
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("BUILD_VERSION", "2.0.0")
	os.Setenv("BUILD_PLATFORMS", "linux/amd64,darwin/amd64")
	os.Setenv("BUILD_PARALLEL", "false")
	os.Setenv("BUILD_CGO_ENABLED", "true")
	defer func() {
		os.Unsetenv("BUILD_VERSION")
		os.Unsetenv("BUILD_PLATFORMS")
		os.Unsetenv("BUILD_PARALLEL")
		os.Unsetenv("BUILD_CGO_ENABLED")
	}()

	config := DefaultBuildConfig()
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load from environment: %v", err)
	}

	if config.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", config.Version)
	}

	expectedPlatforms := []string{"linux/amd64", "darwin/amd64"}
	if len(config.Build.Platforms) != len(expectedPlatforms) {
		t.Errorf("Expected %d platforms, got %d", len(expectedPlatforms), len(config.Build.Platforms))
	}

	for i, platform := range expectedPlatforms {
		if config.Build.Platforms[i] != platform {
			t.Errorf("Expected platform %s, got %s", platform, config.Build.Platforms[i])
		}
	}

	if config.Build.Parallel != false {
		t.Errorf("Expected parallel=false, got %t", config.Build.Parallel)
	}

	if config.Build.Optimization.CGOEnabled != true {
		t.Errorf("Expected CGO enabled=true, got %t", config.Build.Optimization.CGOEnabled)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*BuildConfig)
		expectError bool
	}{
		{
			name:        "valid config",
			modifyFunc:  func(c *BuildConfig) {},
			expectError: false,
		},
		{
			name: "empty version",
			modifyFunc: func(c *BuildConfig) {
				c.Version = ""
			},
			expectError: true,
		},
		{
			name: "no platforms",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Platforms = []string{}
			},
			expectError: true,
		},
		{
			name: "invalid platform",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Platforms = []string{"invalid/platform"}
			},
			expectError: true,
		},
		{
			name: "invalid optimization level",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Optimization.Level = "invalid"
			},
			expectError: true,
		},
		{
			name: "invalid compression level",
			modifyFunc: func(c *BuildConfig) {
				c.Packaging.Compression.Level = 10
			},
			expectError: true,
		},
		{
			name: "empty output directory",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Output.Directory = ""
			},
			expectError: true,
		},
		{
			name: "empty binary prefix",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Output.BinaryPrefix = ""
			},
			expectError: true,
		},
		{
			name: "invalid timeout",
			modifyFunc: func(c *BuildConfig) {
				c.Build.Timeout = -1 * time.Second
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultBuildConfig()
			tt.modifyFunc(config)

			err := config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
			}
		})
	}
}

func TestBinaryNaming(t *testing.T) {
	config := DefaultBuildConfig()
	config.Version = "1.2.3"
	config.Build.Output.BinaryPrefix = "testapp"

	tests := []struct {
		platform      string
		expected      string
		versionSuffix bool
	}{
		{"linux/amd64", "testapp-linux-amd64-1.2.3", true},
		{"darwin/arm64", "testapp-darwin-arm64-1.2.3", true},
		{"windows/amd64", "testapp-windows-amd64-1.2.3.exe", true},
		{"linux/amd64", "testapp-linux-amd64", false},
		{"windows/amd64", "testapp-windows-amd64.exe", false},
	}

	for _, tt := range tests {
		config.Build.Output.VersionSuffix = tt.versionSuffix
		result := config.GetBinaryName(tt.platform)
		if result != tt.expected {
			t.Errorf("For platform %s (version suffix: %t), expected %s, got %s",
				tt.platform, tt.versionSuffix, tt.expected, result)
		}
	}
}

func TestPackageNaming(t *testing.T) {
	config := DefaultBuildConfig()
	config.Version = "1.2.3"
	config.Build.Output.BinaryPrefix = "testapp"

	tests := []struct {
		platform string
		format   string
		expected string
	}{
		{"linux/amd64", "auto", "testapp-linux-amd64-1.2.3.tar.gz"},
		{"darwin/arm64", "auto", "testapp-darwin-arm64-1.2.3.tar.gz"},
		{"windows/amd64", "auto", "testapp-windows-amd64-1.2.3.zip"},
		{"linux/amd64", "zip", "testapp-linux-amd64-1.2.3.zip"},
		{"linux/amd64", "tar.gz", "testapp-linux-amd64-1.2.3.tar.gz"},
	}

	for _, tt := range tests {
		config.Packaging.Compression.Format = tt.format
		result := config.GetPackageName(tt.platform)
		if result != tt.expected {
			t.Errorf("For platform %s with format %s, expected %s, got %s",
				tt.platform, tt.format, tt.expected, result)
		}
	}
}

func TestPlatformDetection(t *testing.T) {
	config := DefaultBuildConfig()

	unixPlatforms := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}
	windowsPlatforms := []string{"windows/amd64", "windows/arm64"}

	for _, platform := range unixPlatforms {
		if !config.IsUnixPlatform(platform) {
			t.Errorf("Platform %s should be detected as Unix", platform)
		}
		if config.IsWindowsPlatform(platform) {
			t.Errorf("Platform %s should not be detected as Windows", platform)
		}
	}

	for _, platform := range windowsPlatforms {
		if !config.IsWindowsPlatform(platform) {
			t.Errorf("Platform %s should be detected as Windows", platform)
		}
		if config.IsUnixPlatform(platform) {
			t.Errorf("Platform %s should not be detected as Unix", platform)
		}
	}
}

func TestBuildConfigLoader(t *testing.T) {
	loader := NewBuildConfigLoader()

	// Test loading with no config file (should use defaults)
	config, err := loader.LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	if config.Version == "" {
		t.Error("Loaded config should have a version")
	}

	// Test config info
	info := loader.GetConfigInfo(config)
	if info["version"] != config.Version {
		t.Error("Config info should include version")
	}

	if platforms, ok := info["platforms"].([]string); !ok || len(platforms) == 0 {
		t.Error("Config info should include platforms")
	}
}

func TestConfigMerging(t *testing.T) {
	loader := NewBuildConfigLoader()

	base := DefaultBuildConfig()
	base.Version = "1.0.0"
	base.Build.Platforms = []string{"linux/amd64"}

	override := DefaultBuildConfig()
	override.Version = "2.0.0"
	override.Build.Platforms = []string{"darwin/amd64", "windows/amd64"}
	override.Distribution.GitHub.Repository = "test/repo"

	merged := loader.MergeConfigs(base, override)

	if merged.Version != "2.0.0" {
		t.Errorf("Expected merged version 2.0.0, got %s", merged.Version)
	}

	expectedPlatforms := []string{"darwin/amd64", "windows/amd64"}
	if len(merged.Build.Platforms) != len(expectedPlatforms) {
		t.Errorf("Expected %d platforms, got %d", len(expectedPlatforms), len(merged.Build.Platforms))
	}

	if merged.Distribution.GitHub.Repository != "test/repo" {
		t.Errorf("Expected repository test/repo, got %s", merged.Distribution.GitHub.Repository)
	}
}
