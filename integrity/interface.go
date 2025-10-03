package integrity

// ChecksumEngineInterface defines the contract for checksum operations
type ChecksumEngineInterface interface {
	// Basic checksum operations
	Calculate(data []byte) uint32
	CalculateString(data []byte) string
	Verify(data []byte, expectedChecksum uint32) error
	VerifyString(data []byte, expectedChecksum string) error

	// Batch operations
	CalculateBatch(dataBlocks [][]byte) []uint32
	VerifyBatch(dataBlocks [][]byte, checksums []uint32) []error

	// File-level operations
	CalculateFileChecksum(filePath string) (uint32, error)
	VerifyFileChecksum(filePath string, expectedChecksum uint32) error

	// Configuration
	SetAlgorithm(algorithm ChecksumAlgorithm)
	GetAlgorithm() ChecksumAlgorithm
	GetAlgorithmName() string
}

// CorruptionDetectorInterface defines the contract for corruption detection
type CorruptionDetectorInterface interface {
	// Real-time detection
	DetectCorruption(data []byte, expectedChecksum uint32) *CorruptionEvent
	ValidateData(data []byte, location string) *CorruptionEvent

	// Background scanning
	StartBackgroundScan(directory string) error
	StopBackgroundScan() error
	ScanDirectory(directory string) ([]CorruptionEvent, error)

	// Monitoring
	GetCorruptionStats() *CorruptionStats
	GetHealthStatus() *IntegrityHealthStatus
}

// IntegrityMonitorInterface defines the contract for integrity monitoring
type IntegrityMonitorInterface interface {
	// Metrics collection
	RecordChecksumOperation(operation string, duration int64, success bool)
	RecordCorruptionEvent(event *CorruptionEvent)
	RecordIntegrityCheck(component string, success bool, details map[string]interface{})

	// Health checks
	PerformHealthCheck() *IntegrityHealthStatus
	GetIntegrityMetrics() *IntegrityMetrics

	// Alerting
	RegisterAlertHandler(handler AlertHandler)
	TriggerAlert(level AlertLevel, message string, details map[string]interface{})
}
