package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BuildConfig represents the complete build configuration
type BuildConfig struct {
	Version      string               `yaml:"version" env:"BUILD_VERSION"`
	Build        BuildSettings        `yaml:"build"`
	Packaging    PackagingSettings    `yaml:"packaging"`
	Distribution DistributionSettings `yaml:"distribution"`
	Environment  EnvironmentSettings  `yaml:"environment"`
}

// BuildSettings contains build-specific configuration
type BuildSettings struct {
	Platforms    []string           `yaml:"platforms" env:"BUILD_PLATFORMS"`
	Optimization OptimizationConfig `yaml:"optimization"`
	Frontend     FrontendConfig     `yaml:"frontend"`
	Output       OutputConfig       `yaml:"output"`
	Parallel     bool               `yaml:"parallel" env:"BUILD_PARALLEL"`
	Timeout      time.Duration      `yaml:"timeout" env:"BUILD_TIMEOUT"`
}

// OptimizationConfig contains build optimization settings
type OptimizationConfig struct {
	Level      string   `yaml:"level" env:"BUILD_OPTIMIZATION_LEVEL"`
	CGOEnabled bool     `yaml:"cgo_enabled" env:"BUILD_CGO_ENABLED"`
	BuildFlags []string `yaml:"build_flags" env:"BUILD_FLAGS"`
	LDFlags    []string `yaml:"ldflags" env:"BUILD_LDFLAGS"`
	Tags       []string `yaml:"tags" env:"BUILD_TAGS"`
}

// FrontendConfig contains frontend build settings
type FrontendConfig struct {
	BuildCommand string `yaml:"build_command" env:"FRONTEND_BUILD_COMMAND"`
	OutputDir    string `yaml:"output_dir" env:"FRONTEND_OUTPUT_DIR"`
	EmbedPath    string `yaml:"embed_path" env:"FRONTEND_EMBED_PATH"`
	NodeVersion  string `yaml:"node_version" env:"FRONTEND_NODE_VERSION"`
	SkipBuild    bool   `yaml:"skip_build" env:"FRONTEND_SKIP_BUILD"`
}

// OutputConfig contains output directory and naming settings
type OutputConfig struct {
	Directory     string `yaml:"directory" env:"BUILD_OUTPUT_DIR"`
	BinaryPrefix  string `yaml:"binary_prefix" env:"BUILD_BINARY_PREFIX"`
	VersionSuffix bool   `yaml:"version_suffix" env:"BUILD_VERSION_SUFFIX"`
	CleanBefore   bool   `yaml:"clean_before" env:"BUILD_CLEAN_BEFORE"`
}

// PackagingSettings contains packaging configuration
type PackagingSettings struct {
	Compression CompressionConfig `yaml:"compression"`
	Installers  InstallersConfig  `yaml:"installers"`
	Assets      AssetsConfig      `yaml:"assets"`
	Checksums   ChecksumsConfig   `yaml:"checksums"`
}

// CompressionConfig contains compression settings
type CompressionConfig struct {
	Level  int    `yaml:"level" env:"PACKAGE_COMPRESSION_LEVEL"`
	Format string `yaml:"format" env:"PACKAGE_COMPRESSION_FORMAT"`
}

// InstallersConfig contains installer generation settings
type InstallersConfig struct {
	Unix    UnixInstallersConfig    `yaml:"unix"`
	Windows WindowsInstallersConfig `yaml:"windows"`
}

// UnixInstallersConfig contains Unix-specific installer settings
type UnixInstallersConfig struct {
	Systemd   bool `yaml:"systemd" env:"INSTALLER_SYSTEMD"`
	Homebrew  bool `yaml:"homebrew" env:"INSTALLER_HOMEBREW"`
	UserLocal bool `yaml:"user_local" env:"INSTALLER_USER_LOCAL"`
}

// WindowsInstallersConfig contains Windows-specific installer settings
type WindowsInstallersConfig struct {
	Chocolatey bool `yaml:"chocolatey" env:"INSTALLER_CHOCOLATEY"`
	MSI        bool `yaml:"msi" env:"INSTALLER_MSI"`
	Service    bool `yaml:"service" env:"INSTALLER_SERVICE"`
}

// AssetsConfig contains asset inclusion settings
type AssetsConfig struct {
	IncludeDocs    bool     `yaml:"include_docs" env:"PACKAGE_INCLUDE_DOCS"`
	IncludeLicense bool     `yaml:"include_license" env:"PACKAGE_INCLUDE_LICENSE"`
	ExtraFiles     []string `yaml:"extra_files" env:"PACKAGE_EXTRA_FILES"`
	ExcludeFiles   []string `yaml:"exclude_files" env:"PACKAGE_EXCLUDE_FILES"`
}

// ChecksumsConfig contains checksum generation settings
type ChecksumsConfig struct {
	Enabled    bool     `yaml:"enabled" env:"CHECKSUMS_ENABLED"`
	Algorithms []string `yaml:"algorithms" env:"CHECKSUMS_ALGORITHMS"`
	Format     string   `yaml:"format" env:"CHECKSUMS_FORMAT"`
}

// DistributionSettings contains distribution configuration
type DistributionSettings struct {
	GitHub        GitHubConfig        `yaml:"github"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Security      BuildSecurityConfig `yaml:"security"`
}

// GitHubConfig contains GitHub release settings
type GitHubConfig struct {
	Repository   string `yaml:"repository" env:"GITHUB_REPOSITORY"`
	Token        string `yaml:"token" env:"GITHUB_TOKEN"`
	Draft        bool   `yaml:"draft" env:"GITHUB_DRAFT"`
	Prerelease   bool   `yaml:"prerelease" env:"GITHUB_PRERELEASE"`
	ReleaseNotes string `yaml:"release_notes" env:"GITHUB_RELEASE_NOTES"`
	TagPrefix    string `yaml:"tag_prefix" env:"GITHUB_TAG_PREFIX"`
}

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	WebhookURL   string `yaml:"webhook_url" env:"NOTIFICATION_WEBHOOK_URL"`
	SlackChannel string `yaml:"slack_channel" env:"NOTIFICATION_SLACK_CHANNEL"`
	EmailTo      string `yaml:"email_to" env:"NOTIFICATION_EMAIL_TO"`
}

// BuildSecurityConfig contains security-related settings
type BuildSecurityConfig struct {
	CodeSigning  CodeSigningConfig  `yaml:"code_signing"`
	Verification VerificationConfig `yaml:"verification"`
}

// CodeSigningConfig contains code signing settings
type CodeSigningConfig struct {
	Enabled       bool   `yaml:"enabled" env:"CODE_SIGNING_ENABLED"`
	MacOSIdentity string `yaml:"macos_identity" env:"CODE_SIGNING_MACOS_IDENTITY"`
	WindowsCert   string `yaml:"windows_cert" env:"CODE_SIGNING_WINDOWS_CERT"`
	WindowsPass   string `yaml:"windows_pass" env:"CODE_SIGNING_WINDOWS_PASS"`
}

// VerificationConfig contains verification settings
type VerificationConfig struct {
	DependencyCheck   bool `yaml:"dependency_check" env:"VERIFICATION_DEPENDENCY_CHECK"`
	ReproducibleBuild bool `yaml:"reproducible_build" env:"VERIFICATION_REPRODUCIBLE_BUILD"`
	AuditLog          bool `yaml:"audit_log" env:"VERIFICATION_AUDIT_LOG"`
}

// EnvironmentSettings contains environment-specific settings
type EnvironmentSettings struct {
	RequiredTools []string          `yaml:"required_tools"`
	EnvVars       map[string]string `yaml:"env_vars"`
	Paths         PathsConfig       `yaml:"paths"`
	Resources     ResourcesConfig   `yaml:"resources"`
}

// PathsConfig contains path-related settings
type PathsConfig struct {
	WorkDir   string `yaml:"work_dir" env:"BUILD_WORK_DIR"`
	TempDir   string `yaml:"temp_dir" env:"BUILD_TEMP_DIR"`
	CacheDir  string `yaml:"cache_dir" env:"BUILD_CACHE_DIR"`
	OutputDir string `yaml:"output_dir" env:"BUILD_OUTPUT_DIR"`
}

// ResourcesConfig contains resource limit settings
type ResourcesConfig struct {
	MaxMemory    string        `yaml:"max_memory" env:"BUILD_MAX_MEMORY"`
	MaxCPU       int           `yaml:"max_cpu" env:"BUILD_MAX_CPU"`
	DiskSpace    string        `yaml:"disk_space" env:"BUILD_DISK_SPACE"`
	BuildTimeout time.Duration `yaml:"build_timeout" env:"BUILD_TIMEOUT"`
}

// DefaultBuildConfig returns a build configuration with default values
func DefaultBuildConfig() *BuildConfig {
	return &BuildConfig{
		Version: "1.0.0",
		Build: BuildSettings{
			Platforms: []string{
				"linux/amd64",
				"linux/arm64",
				"darwin/amd64",
				"darwin/arm64",
				"windows/amd64",
			},
			Optimization: OptimizationConfig{
				Level:      "size",
				CGOEnabled: false,
				BuildFlags: []string{"-trimpath"},
				LDFlags:    []string{"-s", "-w"},
				Tags:       []string{},
			},
			Frontend: FrontendConfig{
				BuildCommand: "npm run build",
				OutputDir:    "admin/frontend/dist",
				EmbedPath:    "admin/api/assets",
				NodeVersion:  "18",
				SkipBuild:    false,
			},
			Output: OutputConfig{
				Directory:     "./dist",
				BinaryPrefix:  "mantisdb",
				VersionSuffix: true,
				CleanBefore:   true,
			},
			Parallel: true,
			Timeout:  30 * time.Minute,
		},
		Packaging: PackagingSettings{
			Compression: CompressionConfig{
				Level:  9,
				Format: "auto",
			},
			Installers: InstallersConfig{
				Unix: UnixInstallersConfig{
					Systemd:   true,
					Homebrew:  true,
					UserLocal: true,
				},
				Windows: WindowsInstallersConfig{
					Chocolatey: true,
					MSI:        false,
					Service:    true,
				},
			},
			Assets: AssetsConfig{
				IncludeDocs:    true,
				IncludeLicense: true,
				ExtraFiles:     []string{"README.md", "INSTALL.md"},
				ExcludeFiles:   []string{"*.log", "*.tmp"},
			},
			Checksums: ChecksumsConfig{
				Enabled:    true,
				Algorithms: []string{"sha256"},
				Format:     "standard",
			},
		},
		Distribution: DistributionSettings{
			GitHub: GitHubConfig{
				Repository:   "mantisdb/mantisdb",
				Draft:        false,
				Prerelease:   false,
				ReleaseNotes: "auto",
				TagPrefix:    "v",
			},
			Notifications: NotificationsConfig{
				SlackChannel: "#releases",
			},
			Security: BuildSecurityConfig{
				CodeSigning: CodeSigningConfig{
					Enabled: false,
				},
				Verification: VerificationConfig{
					DependencyCheck:   true,
					ReproducibleBuild: true,
					AuditLog:          true,
				},
			},
		},
		Environment: EnvironmentSettings{
			RequiredTools: []string{"go", "node", "npm", "gh"},
			EnvVars:       make(map[string]string),
			Paths: PathsConfig{
				WorkDir:   ".",
				TempDir:   "/tmp/mantisdb-build",
				CacheDir:  "./.build-cache",
				OutputDir: "./dist",
			},
			Resources: ResourcesConfig{
				MaxMemory:    "4GB",
				MaxCPU:       0, // Use all available
				DiskSpace:    "10GB",
				BuildTimeout: 60 * time.Minute,
			},
		},
	}
}

// LoadBuildConfig loads build configuration from file and environment
func LoadBuildConfig(configPath string) (*BuildConfig, error) {
	config := DefaultBuildConfig()

	// Load from file if it exists
	if configPath != "" {
		if err := config.LoadFromFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	if err := config.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadFromFile loads configuration from a YAML file
func (c *BuildConfig) LoadFromFile(configPath string) error {
	if configPath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

// LoadFromEnv loads configuration from environment variables
func (c *BuildConfig) LoadFromEnv() error {
	// Version
	if version := os.Getenv("BUILD_VERSION"); version != "" {
		c.Version = version
	}

	// Build settings
	if platforms := os.Getenv("BUILD_PLATFORMS"); platforms != "" {
		c.Build.Platforms = strings.Split(platforms, ",")
		for i, platform := range c.Build.Platforms {
			c.Build.Platforms[i] = strings.TrimSpace(platform)
		}
	}

	if parallel := os.Getenv("BUILD_PARALLEL"); parallel != "" {
		c.Build.Parallel = strings.ToLower(parallel) == "true"
	}

	if timeout := os.Getenv("BUILD_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			c.Build.Timeout = t
		}
	}

	// Optimization settings
	if level := os.Getenv("BUILD_OPTIMIZATION_LEVEL"); level != "" {
		c.Build.Optimization.Level = level
	}

	if cgoEnabled := os.Getenv("BUILD_CGO_ENABLED"); cgoEnabled != "" {
		c.Build.Optimization.CGOEnabled = strings.ToLower(cgoEnabled) == "true"
	}

	if buildFlags := os.Getenv("BUILD_FLAGS"); buildFlags != "" {
		c.Build.Optimization.BuildFlags = strings.Split(buildFlags, ",")
		for i, flag := range c.Build.Optimization.BuildFlags {
			c.Build.Optimization.BuildFlags[i] = strings.TrimSpace(flag)
		}
	}

	if ldFlags := os.Getenv("BUILD_LDFLAGS"); ldFlags != "" {
		c.Build.Optimization.LDFlags = strings.Split(ldFlags, ",")
		for i, flag := range c.Build.Optimization.LDFlags {
			c.Build.Optimization.LDFlags[i] = strings.TrimSpace(flag)
		}
	}

	if tags := os.Getenv("BUILD_TAGS"); tags != "" {
		c.Build.Optimization.Tags = strings.Split(tags, ",")
		for i, tag := range c.Build.Optimization.Tags {
			c.Build.Optimization.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Frontend settings
	if buildCommand := os.Getenv("FRONTEND_BUILD_COMMAND"); buildCommand != "" {
		c.Build.Frontend.BuildCommand = buildCommand
	}

	if outputDir := os.Getenv("FRONTEND_OUTPUT_DIR"); outputDir != "" {
		c.Build.Frontend.OutputDir = outputDir
	}

	if embedPath := os.Getenv("FRONTEND_EMBED_PATH"); embedPath != "" {
		c.Build.Frontend.EmbedPath = embedPath
	}

	if nodeVersion := os.Getenv("FRONTEND_NODE_VERSION"); nodeVersion != "" {
		c.Build.Frontend.NodeVersion = nodeVersion
	}

	if skipBuild := os.Getenv("FRONTEND_SKIP_BUILD"); skipBuild != "" {
		c.Build.Frontend.SkipBuild = strings.ToLower(skipBuild) == "true"
	}

	// Output settings
	if outputDir := os.Getenv("BUILD_OUTPUT_DIR"); outputDir != "" {
		c.Build.Output.Directory = outputDir
	}

	if binaryPrefix := os.Getenv("BUILD_BINARY_PREFIX"); binaryPrefix != "" {
		c.Build.Output.BinaryPrefix = binaryPrefix
	}

	if versionSuffix := os.Getenv("BUILD_VERSION_SUFFIX"); versionSuffix != "" {
		c.Build.Output.VersionSuffix = strings.ToLower(versionSuffix) == "true"
	}

	if cleanBefore := os.Getenv("BUILD_CLEAN_BEFORE"); cleanBefore != "" {
		c.Build.Output.CleanBefore = strings.ToLower(cleanBefore) == "true"
	}

	// Packaging settings
	if compressionLevel := os.Getenv("PACKAGE_COMPRESSION_LEVEL"); compressionLevel != "" {
		if level, err := strconv.Atoi(compressionLevel); err == nil {
			c.Packaging.Compression.Level = level
		}
	}

	if compressionFormat := os.Getenv("PACKAGE_COMPRESSION_FORMAT"); compressionFormat != "" {
		c.Packaging.Compression.Format = compressionFormat
	}

	// GitHub settings
	if repository := os.Getenv("GITHUB_REPOSITORY"); repository != "" {
		c.Distribution.GitHub.Repository = repository
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		c.Distribution.GitHub.Token = token
	}

	if draft := os.Getenv("GITHUB_DRAFT"); draft != "" {
		c.Distribution.GitHub.Draft = strings.ToLower(draft) == "true"
	}

	if prerelease := os.Getenv("GITHUB_PRERELEASE"); prerelease != "" {
		c.Distribution.GitHub.Prerelease = strings.ToLower(prerelease) == "true"
	}

	if releaseNotes := os.Getenv("GITHUB_RELEASE_NOTES"); releaseNotes != "" {
		c.Distribution.GitHub.ReleaseNotes = releaseNotes
	}

	if tagPrefix := os.Getenv("GITHUB_TAG_PREFIX"); tagPrefix != "" {
		c.Distribution.GitHub.TagPrefix = tagPrefix
	}

	// Environment settings
	if workDir := os.Getenv("BUILD_WORK_DIR"); workDir != "" {
		c.Environment.Paths.WorkDir = workDir
	}

	if tempDir := os.Getenv("BUILD_TEMP_DIR"); tempDir != "" {
		c.Environment.Paths.TempDir = tempDir
	}

	if cacheDir := os.Getenv("BUILD_CACHE_DIR"); cacheDir != "" {
		c.Environment.Paths.CacheDir = cacheDir
	}

	if maxMemory := os.Getenv("BUILD_MAX_MEMORY"); maxMemory != "" {
		c.Environment.Resources.MaxMemory = maxMemory
	}

	if maxCPU := os.Getenv("BUILD_MAX_CPU"); maxCPU != "" {
		if cpu, err := strconv.Atoi(maxCPU); err == nil {
			c.Environment.Resources.MaxCPU = cpu
		}
	}

	if diskSpace := os.Getenv("BUILD_DISK_SPACE"); diskSpace != "" {
		c.Environment.Resources.DiskSpace = diskSpace
	}

	return nil
}

// Validate validates the build configuration
func (c *BuildConfig) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	if len(c.Build.Platforms) == 0 {
		return fmt.Errorf("at least one platform must be specified")
	}

	// Validate platforms
	validPlatforms := map[string]bool{
		"linux/amd64":   true,
		"linux/arm64":   true,
		"darwin/amd64":  true,
		"darwin/arm64":  true,
		"windows/amd64": true,
		"windows/arm64": true,
	}

	for _, platform := range c.Build.Platforms {
		if !validPlatforms[platform] {
			return fmt.Errorf("invalid platform: %s", platform)
		}
	}

	// Validate optimization level
	validLevels := map[string]bool{
		"size":  true,
		"speed": true,
		"debug": true,
	}

	if !validLevels[c.Build.Optimization.Level] {
		return fmt.Errorf("invalid optimization level: %s", c.Build.Optimization.Level)
	}

	// Validate compression level
	if c.Packaging.Compression.Level < 0 || c.Packaging.Compression.Level > 9 {
		return fmt.Errorf("compression level must be between 0 and 9")
	}

	// Validate compression format
	validFormats := map[string]bool{
		"auto":   true,
		"tar.gz": true,
		"zip":    true,
		"tar.xz": true,
	}

	if !validFormats[c.Packaging.Compression.Format] {
		return fmt.Errorf("invalid compression format: %s", c.Packaging.Compression.Format)
	}

	// Validate GitHub repository format
	if c.Distribution.GitHub.Repository != "" {
		parts := strings.Split(c.Distribution.GitHub.Repository, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid GitHub repository format: %s (expected owner/repo)", c.Distribution.GitHub.Repository)
		}
	}

	// Validate paths
	if c.Build.Output.Directory == "" {
		return fmt.Errorf("output directory cannot be empty")
	}

	if c.Build.Output.BinaryPrefix == "" {
		return fmt.Errorf("binary prefix cannot be empty")
	}

	// Validate timeout
	if c.Build.Timeout <= 0 {
		return fmt.Errorf("build timeout must be positive")
	}

	return nil
}

// GetBinaryName returns the binary name for a given platform
func (c *BuildConfig) GetBinaryName(platform string) string {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return c.Build.Output.BinaryPrefix
	}

	os, arch := parts[0], parts[1]
	name := fmt.Sprintf("%s-%s-%s", c.Build.Output.BinaryPrefix, os, arch)

	if c.Build.Output.VersionSuffix {
		name = fmt.Sprintf("%s-%s", name, c.Version)
	}

	if os == "windows" {
		name += ".exe"
	}

	return name
}

// GetPackageName returns the package name for a given platform
func (c *BuildConfig) GetPackageName(platform string) string {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return c.Build.Output.BinaryPrefix
	}

	os, arch := parts[0], parts[1]
	name := fmt.Sprintf("%s-%s-%s", c.Build.Output.BinaryPrefix, os, arch)

	if c.Build.Output.VersionSuffix {
		name = fmt.Sprintf("%s-%s", name, c.Version)
	}

	// Determine extension based on platform and format
	switch c.Packaging.Compression.Format {
	case "auto":
		if os == "windows" {
			name += ".zip"
		} else {
			name += ".tar.gz"
		}
	case "zip":
		name += ".zip"
	case "tar.gz":
		name += ".tar.gz"
	case "tar.xz":
		name += ".tar.xz"
	}

	return name
}

// GetOutputPath returns the full output path for a binary
func (c *BuildConfig) GetOutputPath(platform string) string {
	return filepath.Join(c.Build.Output.Directory, c.GetBinaryName(platform))
}

// GetPackagePath returns the full package path for a platform
func (c *BuildConfig) GetPackagePath(platform string) string {
	return filepath.Join(c.Build.Output.Directory, c.GetPackageName(platform))
}

// IsUnixPlatform returns true if the platform is Unix-like
func (c *BuildConfig) IsUnixPlatform(platform string) bool {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return false
	}
	os := parts[0]
	return os == "linux" || os == "darwin"
}

// IsWindowsPlatform returns true if the platform is Windows
func (c *BuildConfig) IsWindowsPlatform(platform string) bool {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return false
	}
	return parts[0] == "windows"
}

// SaveToFile saves the configuration to a YAML file
func (c *BuildConfig) SaveToFile(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
