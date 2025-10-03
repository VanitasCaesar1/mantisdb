package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoggingConfig holds the complete logging configuration
type LoggingConfig struct {
	Level      string            `yaml:"level" json:"level"`
	Format     string            `yaml:"format" json:"format"`
	Output     string            `yaml:"output" json:"output"`
	File       FileConfig        `yaml:"file" json:"file"`
	Rotation   RotationConfig    `yaml:"rotation" json:"rotation"`
	Retention  RetentionConfig   `yaml:"retention" json:"retention"`
	Components map[string]string `yaml:"components" json:"components"`
}

// FileConfig holds file output configuration
type FileConfig struct {
	Path string `yaml:"path" json:"path"`
	Dir  string `yaml:"dir" json:"dir"`
}

// RotationConfig holds log rotation configuration
type RotationConfig struct {
	MaxSize    string `yaml:"max_size" json:"max_size"` // e.g., "100MB"
	MaxAge     string `yaml:"max_age" json:"max_age"`   // e.g., "30d"
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
}

// RetentionConfig holds log retention configuration
type RetentionConfig struct {
	Days    int  `yaml:"days" json:"days"`
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns a default logging configuration
func DefaultConfig() LoggingConfig {
	return LoggingConfig{
		Level:  "INFO",
		Format: "json",
		Output: "stdout",
		File: FileConfig{
			Path: "logs/mantisdb.log",
			Dir:  "logs",
		},
		Rotation: RotationConfig{
			MaxSize:    "100MB",
			MaxAge:     "30d",
			MaxBackups: 10,
		},
		Retention: RetentionConfig{
			Days:    30,
			Enabled: true,
		},
		Components: map[string]string{
			"query_executor": "INFO",
			"storage":        "WARN",
			"wal":            "INFO",
			"backup":         "INFO",
			"admin":          "DEBUG",
		},
	}
}

// ParseLevel parses a log level string
func ParseLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN", "WARNING":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return INFO, fmt.Errorf("invalid log level: %s", level)
	}
}

// ParseSize parses a size string (e.g., "100MB", "1GB")
func ParseSize(size string) (int64, error) {
	size = strings.ToUpper(strings.TrimSpace(size))

	if size == "" {
		return 0, fmt.Errorf("empty size string")
	}

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(size, "KB") {
		multiplier = 1024
		numStr = size[:len(size)-2]
	} else if strings.HasSuffix(size, "MB") {
		multiplier = 1024 * 1024
		numStr = size[:len(size)-2]
	} else if strings.HasSuffix(size, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = size[:len(size)-2]
	} else if strings.HasSuffix(size, "B") {
		multiplier = 1
		numStr = size[:len(size)-1]
	} else {
		numStr = size
	}

	var num int64
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid size format: %s", size)
	}

	return num * multiplier, nil
}

// ParseDuration parses a duration string (e.g., "30d", "7h", "60m")
func ParseDuration(duration string) (time.Duration, error) {
	duration = strings.ToLower(strings.TrimSpace(duration))

	if duration == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	if strings.HasSuffix(duration, "d") {
		days := duration[:len(duration)-1]
		var num int
		if _, err := fmt.Sscanf(days, "%d", &num); err != nil {
			return 0, fmt.Errorf("invalid duration format: %s", duration)
		}
		return time.Duration(num) * 24 * time.Hour, nil
	}

	// Use standard time.ParseDuration for other formats
	return time.ParseDuration(duration)
}

// SetupLogging sets up logging based on configuration
func SetupLogging(config LoggingConfig) (*StructuredLogger, *LogManager, error) {
	// Parse log level
	level, err := ParseLevel(config.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create outputs based on configuration
	var outputs []LogOutput

	switch strings.ToLower(config.Output) {
	case "stdout":
		outputs = append(outputs, NewJSONOutput(os.Stdout))
	case "stderr":
		outputs = append(outputs, NewJSONOutput(os.Stderr))
	case "file":
		// Create log directory
		logDir := config.File.Dir
		if logDir == "" {
			logDir = filepath.Dir(config.File.Path)
		}

		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Parse rotation settings
		maxSize, err := ParseSize(config.Rotation.MaxSize)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max size: %w", err)
		}

		maxAge, err := ParseDuration(config.Rotation.MaxAge)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max age: %w", err)
		}

		fileOutput, err := NewFileOutput(FileOutputConfig{
			Filename:   config.File.Path,
			MaxSize:    maxSize,
			MaxAge:     int(maxAge.Hours() / 24), // Convert to days
			MaxBackups: config.Rotation.MaxBackups,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file output: %w", err)
		}

		outputs = append(outputs, fileOutput)
	case "both":
		// Both stdout and file
		outputs = append(outputs, NewJSONOutput(os.Stdout))

		logDir := config.File.Dir
		if logDir == "" {
			logDir = filepath.Dir(config.File.Path)
		}

		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		maxSize, err := ParseSize(config.Rotation.MaxSize)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max size: %w", err)
		}

		maxAge, err := ParseDuration(config.Rotation.MaxAge)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max age: %w", err)
		}

		fileOutput, err := NewFileOutput(FileOutputConfig{
			Filename:   config.File.Path,
			MaxSize:    maxSize,
			MaxAge:     int(maxAge.Hours() / 24),
			MaxBackups: config.Rotation.MaxBackups,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file output: %w", err)
		}

		outputs = append(outputs, fileOutput)
	default:
		return nil, nil, fmt.Errorf("invalid output type: %s", config.Output)
	}

	// Create structured logger
	logger := NewStructuredLogger(Config{
		Level:   level,
		Outputs: outputs,
		Context: make(map[string]interface{}),
	})

	// Create log manager
	logDir := config.File.Dir
	if logDir == "" {
		logDir = filepath.Dir(config.File.Path)
	}

	manager := NewLogManager(LogManagerConfig{
		LogDir: logDir,
		Logger: logger,
	})

	return logger, manager, nil
}

// GetComponentLogger returns a logger configured for a specific component
func GetComponentLogger(baseLogger *StructuredLogger, config LoggingConfig, component string) *StructuredLogger {
	componentLogger := baseLogger.WithComponent(component)

	// Check if component has specific log level
	if levelStr, exists := config.Components[component]; exists {
		if level, err := ParseLevel(levelStr); err == nil {
			componentLogger.SetLevel(level)
		}
	}

	return componentLogger
}
