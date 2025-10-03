package integrity

import (
	"time"
)

// CorruptionEvent represents a detected corruption event
type CorruptionEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Location    string                 `json:"location"`
	Type        CorruptionType         `json:"type"`
	Severity    CorruptionSeverity     `json:"severity"`
	Description string                 `json:"description"`
	Expected    uint32                 `json:"expected_checksum,omitempty"`
	Actual      uint32                 `json:"actual_checksum,omitempty"`
	Size        int64                  `json:"size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CorruptionType represents different types of corruption
type CorruptionType string

const (
	CorruptionTypeChecksum CorruptionType = "checksum_mismatch"
	CorruptionTypeFormat   CorruptionType = "format_error"
	CorruptionTypeSize     CorruptionType = "size_mismatch"
	CorruptionTypeHeader   CorruptionType = "header_corruption"
	CorruptionTypeData     CorruptionType = "data_corruption"
	CorruptionTypeUnknown  CorruptionType = "unknown"
)

// CorruptionSeverity represents the severity of corruption
type CorruptionSeverity string

const (
	SeverityCritical CorruptionSeverity = "critical"
	SeverityHigh     CorruptionSeverity = "high"
	SeverityMedium   CorruptionSeverity = "medium"
	SeverityLow      CorruptionSeverity = "low"
)

// CorruptionStats contains statistics about corruption events
type CorruptionStats struct {
	TotalEvents      int64                        `json:"total_events"`
	EventsByType     map[CorruptionType]int64     `json:"events_by_type"`
	EventsBySeverity map[CorruptionSeverity]int64 `json:"events_by_severity"`
	LastEvent        *CorruptionEvent             `json:"last_event,omitempty"`
	LastScanTime     time.Time                    `json:"last_scan_time"`
	CorruptionRate   float64                      `json:"corruption_rate"`
	RecoverySuccess  int64                        `json:"recovery_success"`
	RecoveryFailures int64                        `json:"recovery_failures"`
}

// IntegrityHealthStatus represents the overall health of the integrity system
type IntegrityHealthStatus struct {
	Status          HealthStatus            `json:"status"`
	LastCheckTime   time.Time               `json:"last_check_time"`
	ComponentHealth map[string]HealthStatus `json:"component_health"`
	ActiveScans     int                     `json:"active_scans"`
	RecentEvents    []CorruptionEvent       `json:"recent_events"`
	Metrics         *IntegrityMetrics       `json:"metrics"`
	Recommendations []string                `json:"recommendations,omitempty"`
}

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusWarning  HealthStatus = "warning"
	HealthStatusCritical HealthStatus = "critical"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// IntegrityMetrics contains performance and operational metrics
type IntegrityMetrics struct {
	ChecksumOperations  *OperationMetrics   `json:"checksum_operations"`
	CorruptionDetection *OperationMetrics   `json:"corruption_detection"`
	BackgroundScans     *ScanMetrics        `json:"background_scans"`
	FileOperations      *OperationMetrics   `json:"file_operations"`
	PerformanceMetrics  *PerformanceMetrics `json:"performance_metrics"`
	LastUpdated         time.Time           `json:"last_updated"`
}

// OperationMetrics contains metrics for specific operations
type OperationMetrics struct {
	TotalOperations  int64         `json:"total_operations"`
	SuccessfulOps    int64         `json:"successful_operations"`
	FailedOps        int64         `json:"failed_operations"`
	AverageLatency   time.Duration `json:"average_latency"`
	MaxLatency       time.Duration `json:"max_latency"`
	MinLatency       time.Duration `json:"min_latency"`
	OperationsPerSec float64       `json:"operations_per_second"`
	LastOperation    time.Time     `json:"last_operation"`
}

// ScanMetrics contains metrics for background scanning operations
type ScanMetrics struct {
	TotalScans      int64         `json:"total_scans"`
	CompletedScans  int64         `json:"completed_scans"`
	FailedScans     int64         `json:"failed_scans"`
	FilesScanned    int64         `json:"files_scanned"`
	BytesScanned    int64         `json:"bytes_scanned"`
	AverageScanTime time.Duration `json:"average_scan_time"`
	LastScanTime    time.Time     `json:"last_scan_time"`
	ActiveScans     int           `json:"active_scans"`
}

// PerformanceMetrics contains performance-related metrics
type PerformanceMetrics struct {
	ThroughputBytesPerSec int64         `json:"throughput_bytes_per_sec"`
	MemoryUsage           int64         `json:"memory_usage_bytes"`
	CPUUsage              float64       `json:"cpu_usage_percent"`
	DiskIOOperations      int64         `json:"disk_io_operations"`
	CacheHitRate          float64       `json:"cache_hit_rate"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
}

// AlertLevel represents the severity level of alerts
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertHandler defines the interface for handling alerts
type AlertHandler interface {
	HandleAlert(level AlertLevel, message string, details map[string]interface{}) error
}

// IntegrityConfig contains configuration for the integrity system
type IntegrityConfig struct {
	ChecksumAlgorithm       ChecksumAlgorithm `json:"checksum_algorithm"`
	EnableBackgroundScan    bool              `json:"enable_background_scan"`
	ScanInterval            time.Duration     `json:"scan_interval"`
	MaxConcurrentScans      int               `json:"max_concurrent_scans"`
	EnableRealTimeDetection bool              `json:"enable_realtime_detection"`
	EnableAutoRecovery      bool              `json:"enable_auto_recovery"`
	AlertThresholds         AlertThresholds   `json:"alert_thresholds"`
	RetentionPeriod         time.Duration     `json:"retention_period"`
}

// AlertThresholds defines thresholds for triggering alerts
type AlertThresholds struct {
	CorruptionRate float64       `json:"corruption_rate"`
	FailureRate    float64       `json:"failure_rate"`
	ResponseTime   time.Duration `json:"response_time"`
	MemoryUsage    int64         `json:"memory_usage_bytes"`
	DiskUsage      float64       `json:"disk_usage_percent"`
}

// DefaultIntegrityConfig returns a default configuration
func DefaultIntegrityConfig() *IntegrityConfig {
	return &IntegrityConfig{
		ChecksumAlgorithm:       ChecksumCRC32,
		EnableBackgroundScan:    true,
		ScanInterval:            1 * time.Hour,
		MaxConcurrentScans:      2,
		EnableRealTimeDetection: true,
		EnableAutoRecovery:      false,
		AlertThresholds: AlertThresholds{
			CorruptionRate: 0.01, // 1%
			FailureRate:    0.05, // 5%
			ResponseTime:   5 * time.Second,
			MemoryUsage:    1024 * 1024 * 1024, // 1GB
			DiskUsage:      0.90,               // 90%
		},
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
	}
}
