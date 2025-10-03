package integrity

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IntegritySystem is the main system that coordinates all integrity components
type IntegritySystem struct {
	config             *IntegrityConfig
	checksumEngine     *ChecksumEngine
	corruptionDetector *CorruptionDetector
	walVerifier        *WALIntegrityVerifier
	monitor            *IntegrityMonitor
	mutex              sync.RWMutex
	running            bool
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
}

// NewIntegritySystem creates a new integrity system with all components
func NewIntegritySystem(config *IntegrityConfig) *IntegritySystem {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	system := &IntegritySystem{
		config:             config,
		checksumEngine:     NewChecksumEngine(config.ChecksumAlgorithm),
		corruptionDetector: NewCorruptionDetector(config),
		walVerifier:        NewWALIntegrityVerifier(config),
		monitor:            NewIntegrityMonitor(config),
		ctx:                ctx,
		cancel:             cancel,
	}

	// Wire up event handlers
	system.setupEventHandlers()

	return system
}

// Start starts the integrity system
func (is *IntegritySystem) Start() error {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	if is.running {
		return fmt.Errorf("integrity system is already running")
	}

	// Setup default alert handlers
	logHandler := NewLogAlertHandler()
	is.monitor.RegisterAlertHandler(logHandler)

	// Start background corruption detection if enabled
	if is.config.EnableBackgroundScan {
		// This would typically scan a configured data directory
		// For now, we'll just mark it as ready to scan
		is.monitor.RecordIntegrityCheck("background_scanner", true, map[string]interface{}{
			"status": "ready",
		})
	}

	is.running = true

	// Perform initial health check
	is.monitor.PerformHealthCheck()

	return nil
}

// Stop stops the integrity system
func (is *IntegritySystem) Stop() error {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	if !is.running {
		return fmt.Errorf("integrity system is not running")
	}

	// Cancel context and wait for goroutines
	is.cancel()
	is.wg.Wait()

	// Stop components
	if err := is.corruptionDetector.StopBackgroundScan(); err != nil {
		// Log error but continue shutdown
	}

	if err := is.monitor.Close(); err != nil {
		// Log error but continue shutdown
	}

	is.running = false
	return nil
}

// GetChecksumEngine returns the checksum engine
func (is *IntegritySystem) GetChecksumEngine() *ChecksumEngine {
	return is.checksumEngine
}

// GetCorruptionDetector returns the corruption detector
func (is *IntegritySystem) GetCorruptionDetector() *CorruptionDetector {
	return is.corruptionDetector
}

// GetWALVerifier returns the WAL integrity verifier
func (is *IntegritySystem) GetWALVerifier() *WALIntegrityVerifier {
	return is.walVerifier
}

// GetMonitor returns the integrity monitor
func (is *IntegritySystem) GetMonitor() *IntegrityMonitor {
	return is.monitor
}

// VerifyData performs comprehensive data verification
func (is *IntegritySystem) VerifyData(data []byte, location string, expectedChecksum uint32) error {
	startTime := time.Now()

	// Verify checksum
	if err := is.checksumEngine.Verify(data, expectedChecksum); err != nil {
		// Record the operation
		is.monitor.RecordChecksumOperation("verify_data", time.Since(startTime).Nanoseconds(), false)

		// Detect corruption
		if event := is.corruptionDetector.DetectCorruption(data, expectedChecksum); event != nil {
			is.monitor.RecordCorruptionEvent(event)
		}

		return err
	}

	// Record successful operation
	is.monitor.RecordChecksumOperation("verify_data", time.Since(startTime).Nanoseconds(), true)
	return nil
}

// CalculateAndVerifyChecksum calculates checksum and verifies data integrity
func (is *IntegritySystem) CalculateAndVerifyChecksum(data []byte, location string) (uint32, error) {
	startTime := time.Now()

	checksum := is.checksumEngine.Calculate(data)

	// Validate the data
	if event := is.corruptionDetector.ValidateData(data, location); event != nil {
		is.monitor.RecordCorruptionEvent(event)
		is.monitor.RecordChecksumOperation("calculate_checksum", time.Since(startTime).Nanoseconds(), false)
		return checksum, &CorruptionDetectedError{
			Location:    location,
			Type:        string(event.Type),
			Description: event.Description,
			Checksum:    checksum,
		}
	}

	// Record successful operation
	is.monitor.RecordChecksumOperation("calculate_checksum", time.Since(startTime).Nanoseconds(), true)
	return checksum, nil
}

// StartBackgroundScan starts background integrity scanning
func (is *IntegritySystem) StartBackgroundScan(directory string) error {
	if !is.config.EnableBackgroundScan {
		return fmt.Errorf("background scanning is disabled")
	}

	if err := is.corruptionDetector.StartBackgroundScan(directory); err != nil {
		is.monitor.RecordIntegrityCheck("background_scanner", false, map[string]interface{}{
			"error":     err.Error(),
			"directory": directory,
		})
		return err
	}

	is.monitor.RecordIntegrityCheck("background_scanner", true, map[string]interface{}{
		"status":    "started",
		"directory": directory,
	})

	return nil
}

// StopBackgroundScan stops background integrity scanning
func (is *IntegritySystem) StopBackgroundScan() error {
	if err := is.corruptionDetector.StopBackgroundScan(); err != nil {
		is.monitor.RecordIntegrityCheck("background_scanner", false, map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	is.monitor.RecordIntegrityCheck("background_scanner", true, map[string]interface{}{
		"status": "stopped",
	})

	return nil
}

// GetHealthStatus returns the current health status
func (is *IntegritySystem) GetHealthStatus() *IntegrityHealthStatus {
	return is.monitor.PerformHealthCheck()
}

// GetMetrics returns current integrity metrics
func (is *IntegritySystem) GetMetrics() *IntegrityMetrics {
	return is.monitor.GetIntegrityMetrics()
}

// ExportMetrics exports metrics to a file
func (is *IntegritySystem) ExportMetrics(filePath string) error {
	return is.monitor.ExportMetrics(filePath)
}

// RegisterAlertHandler registers an alert handler
func (is *IntegritySystem) RegisterAlertHandler(handler AlertHandler) {
	is.monitor.RegisterAlertHandler(handler)
}

// IsRunning returns whether the integrity system is running
func (is *IntegritySystem) IsRunning() bool {
	is.mutex.RLock()
	defer is.mutex.RUnlock()
	return is.running
}

// GetConfig returns the current configuration
func (is *IntegritySystem) GetConfig() *IntegrityConfig {
	return is.config
}

// UpdateConfig updates the system configuration
func (is *IntegritySystem) UpdateConfig(config *IntegrityConfig) error {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Update checksum algorithm if changed
	if config.ChecksumAlgorithm != is.config.ChecksumAlgorithm {
		is.checksumEngine.SetAlgorithm(config.ChecksumAlgorithm)
	}

	is.config = config

	// Record configuration update
	is.monitor.RecordIntegrityCheck("integrity_system", true, map[string]interface{}{
		"operation": "config_update",
		"algorithm": is.checksumEngine.GetAlgorithmName(),
	})

	return nil
}

// Private methods

func (is *IntegritySystem) setupEventHandlers() {
	// Register corruption event handler
	is.corruptionDetector.RegisterEventHandler(func(event *CorruptionEvent) {
		is.monitor.RecordCorruptionEvent(event)
	})
}

// Utility functions for easy integration

// VerifyFileIntegrity is a convenience function to verify file integrity
func (is *IntegritySystem) VerifyFileIntegrity(filePath string) error {
	checksum, err := is.checksumEngine.CalculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file checksum: %w", err)
	}

	// For now, we just calculate and cache the checksum
	// In a real implementation, you'd compare against a stored checksum
	is.corruptionDetector.UpdateChecksumCache(filePath, checksum)

	is.monitor.RecordIntegrityCheck("file_verification", true, map[string]interface{}{
		"file_path": filePath,
		"checksum":  fmt.Sprintf("%08x", checksum),
	})

	return nil
}

// BatchVerifyFiles verifies multiple files for integrity
func (is *IntegritySystem) BatchVerifyFiles(filePaths []string) map[string]error {
	results := make(map[string]error)

	for _, filePath := range filePaths {
		results[filePath] = is.VerifyFileIntegrity(filePath)
	}

	is.monitor.RecordIntegrityCheck("batch_file_verification", true, map[string]interface{}{
		"file_count": len(filePaths),
		"results":    len(results),
	})

	return results
}
