package compression

import (
	"fmt"
	"sync"
	"time"
)

// CompressionManager integrates compression engine with cold data detection
type CompressionManager struct {
	engine           *CompressionEngine
	coldDetector     *ColdDataDetector
	transparent      *TransparentCompression
	backgroundWorker *BackgroundCompressionWorker
	config           *CompressionManagerConfig
	mutex            sync.RWMutex
}

// CompressionManagerConfig configures the compression manager
type CompressionManagerConfig struct {
	Enabled                bool          `json:"enabled"`
	BackgroundCompression  bool          `json:"background_compression"`
	CompressionInterval    time.Duration `json:"compression_interval"`
	MaxCandidatesPerCycle  int           `json:"max_candidates_per_cycle"`
	CompressionThreshold   int64         `json:"compression_threshold"`
	DecompressionCacheSize int           `json:"decompression_cache_size"`
}

// BackgroundCompressionWorker handles background compression of cold data
type BackgroundCompressionWorker struct {
	manager    *CompressionManager
	stopChan   chan struct{}
	workQueue  chan CompressionJob
	workerPool []chan CompressionJob
	wg         sync.WaitGroup
	mutex      sync.RWMutex
}

// CompressionJob represents a compression task
type CompressionJob struct {
	Key      string
	Data     []byte
	Metadata *DataMetadata
	Callback func(key string, compressed []byte, algorithm string, err error)
}

// CompressionResult represents the result of a compression operation
type CompressionResult struct {
	Key              string        `json:"key"`
	OriginalSize     int64         `json:"original_size"`
	CompressedSize   int64         `json:"compressed_size"`
	Algorithm        string        `json:"algorithm"`
	CompressionRatio float64       `json:"compression_ratio"`
	CompressionTime  time.Duration `json:"compression_time"`
	Success          bool          `json:"success"`
	Error            string        `json:"error,omitempty"`
}

// NewCompressionManager creates a new compression manager
func NewCompressionManager(config *CompressionManagerConfig) *CompressionManager {
	if config == nil {
		config = &CompressionManagerConfig{
			Enabled:                true,
			BackgroundCompression:  true,
			CompressionInterval:    5 * time.Minute,
			MaxCandidatesPerCycle:  100,
			CompressionThreshold:   1024,
			DecompressionCacheSize: 1000,
		}
	}

	transparentConfig := &TransparentConfig{
		Enabled:            config.Enabled,
		MinSize:            config.CompressionThreshold,
		ColdThreshold:      24 * time.Hour,
		DefaultAlgorithm:   "lz4",
		CompressionLevel:   1,
		BackgroundCompress: config.BackgroundCompression,
	}

	coldConfig := &ColdDataConfig{
		Enabled:              config.Enabled,
		ColdThreshold:        24 * time.Hour,
		BloomFilterSize:      1000000,
		BloomFilterHashCount: 3,
		AccessTrackingWindow: 7 * 24 * time.Hour,
		MinAccessCount:       5,
		SizeThreshold:        config.CompressionThreshold,
		CleanupInterval:      time.Hour,
	}

	manager := &CompressionManager{
		engine:       NewCompressionEngine(),
		coldDetector: NewColdDataDetector(coldConfig),
		transparent:  NewTransparentCompression(transparentConfig),
		config:       config,
	}

	if config.BackgroundCompression {
		manager.backgroundWorker = NewBackgroundCompressionWorker(manager)
		manager.backgroundWorker.Start()
	}

	return manager
}

// NewBackgroundCompressionWorker creates a new background compression worker
func NewBackgroundCompressionWorker(manager *CompressionManager) *BackgroundCompressionWorker {
	worker := &BackgroundCompressionWorker{
		manager:    manager,
		stopChan:   make(chan struct{}),
		workQueue:  make(chan CompressionJob, 1000),
		workerPool: make([]chan CompressionJob, 4), // 4 worker goroutines
	}

	// Initialize worker pool
	for i := range worker.workerPool {
		worker.workerPool[i] = make(chan CompressionJob, 10)
	}

	return worker
}

// Start starts the background compression worker
func (bcw *BackgroundCompressionWorker) Start() {
	// Start worker goroutines
	for i, workerChan := range bcw.workerPool {
		bcw.wg.Add(1)
		go bcw.worker(i, workerChan)
	}

	// Start job dispatcher
	bcw.wg.Add(1)
	go bcw.dispatcher()

	// Start periodic cold data compression
	bcw.wg.Add(1)
	go bcw.periodicCompression()
}

// Stop stops the background compression worker
func (bcw *BackgroundCompressionWorker) Stop() {
	close(bcw.stopChan)
	bcw.wg.Wait()
}

// worker processes compression jobs
func (bcw *BackgroundCompressionWorker) worker(id int, jobs <-chan CompressionJob) {
	defer bcw.wg.Done()

	for {
		select {
		case job := <-jobs:
			bcw.processJob(job)
		case <-bcw.stopChan:
			return
		}
	}
}

// dispatcher distributes jobs to workers
func (bcw *BackgroundCompressionWorker) dispatcher() {
	defer bcw.wg.Done()

	workerIndex := 0
	for {
		select {
		case job := <-bcw.workQueue:
			// Round-robin job distribution
			bcw.workerPool[workerIndex] <- job
			workerIndex = (workerIndex + 1) % len(bcw.workerPool)
		case <-bcw.stopChan:
			return
		}
	}
}

// periodicCompression runs periodic compression of cold data
func (bcw *BackgroundCompressionWorker) periodicCompression() {
	defer bcw.wg.Done()

	ticker := time.NewTicker(bcw.manager.config.CompressionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bcw.compressColdData()
		case <-bcw.stopChan:
			return
		}
	}
}

// compressColdData identifies and compresses cold data
func (bcw *BackgroundCompressionWorker) compressColdData() {
	candidates := bcw.manager.coldDetector.GetColdDataCandidates(bcw.manager.config.MaxCandidatesPerCycle)

	for _, candidate := range candidates {
		// Create compression job for cold data
		job := CompressionJob{
			Key: candidate.Key,
			Metadata: &DataMetadata{
				Size:         candidate.Size,
				LastAccessed: candidate.LastAccessed,
				AccessCount:  candidate.AccessCount,
			},
			Callback: func(key string, compressed []byte, algorithm string, err error) {
				if err != nil {
					// Log compression error
					return
				}
				// Handle successful compression (e.g., update storage)
			},
		}

		select {
		case bcw.workQueue <- job:
			// Job queued successfully
		default:
			// Queue is full, skip remaining candidates
			return
		}
	}
}

// processJob processes a single compression job
func (bcw *BackgroundCompressionWorker) processJob(job CompressionJob) {
	start := time.Now()

	compressed, algorithm, err := bcw.manager.engine.Compress(job.Data, job.Metadata)

	duration := time.Since(start)

	if job.Callback != nil {
		job.Callback(job.Key, compressed, algorithm, err)
	}

	// Record metrics
	if err == nil && algorithm != "none" {
		bcw.manager.engine.monitor.RecordCompressionTime(algorithm, duration)
	}
}

// Write handles writing data with compression
func (cm *CompressionManager) Write(key string, data []byte) ([]byte, error) {
	if !cm.config.Enabled {
		return data, nil
	}

	// Record access for cold data detection
	cm.coldDetector.RecordAccess(key, int64(len(data)))

	// Use transparent compression
	return cm.transparent.Write(key, data)
}

// Read handles reading data with decompression
func (cm *CompressionManager) Read(key string, data []byte) ([]byte, error) {
	if !cm.config.Enabled {
		return data, nil
	}

	// Record access for cold data detection
	cm.coldDetector.RecordAccess(key, int64(len(data)))

	// Use transparent compression for decompression
	return cm.transparent.Read(key, data)
}

// CompressAsync queues data for background compression
func (cm *CompressionManager) CompressAsync(key string, data []byte, metadata *DataMetadata, callback func(key string, compressed []byte, algorithm string, err error)) error {
	if !cm.config.Enabled || cm.backgroundWorker == nil {
		return fmt.Errorf("background compression not enabled")
	}

	job := CompressionJob{
		Key:      key,
		Data:     data,
		Metadata: metadata,
		Callback: callback,
	}

	select {
	case cm.backgroundWorker.workQueue <- job:
		return nil
	default:
		return fmt.Errorf("compression queue is full")
	}
}

// GetCompressionStats returns comprehensive compression statistics
func (cm *CompressionManager) GetCompressionStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Engine stats
	engineStats := cm.engine.GetStats()
	stats["engine"] = map[string]interface{}{
		"total_compressed":   engineStats.TotalCompressed,
		"total_decompressed": engineStats.TotalDecompressed,
		"compression_ratio":  engineStats.CompressionRatio,
		"compression_time":   engineStats.CompressionTime,
		"decompression_time": engineStats.DecompressionTime,
	}

	// Monitor metrics
	monitorMetrics := cm.engine.monitor.GetMetrics()
	stats["metrics"] = monitorMetrics

	// Cold data detector stats
	coldConfig := cm.coldDetector.GetConfig()
	stats["cold_detection"] = map[string]interface{}{
		"enabled":        coldConfig.Enabled,
		"cold_threshold": coldConfig.ColdThreshold,
		"size_threshold": coldConfig.SizeThreshold,
	}

	// Background worker stats
	if cm.backgroundWorker != nil {
		stats["background_worker"] = map[string]interface{}{
			"queue_size":   len(cm.backgroundWorker.workQueue),
			"worker_count": len(cm.backgroundWorker.workerPool),
		}
	}

	return stats
}

// GetColdDataCandidates returns current cold data candidates
func (cm *CompressionManager) GetColdDataCandidates(limit int) []ColdDataCandidate {
	return cm.coldDetector.GetColdDataCandidates(limit)
}

// UpdateConfig updates the compression manager configuration
func (cm *CompressionManager) UpdateConfig(config *CompressionManagerConfig) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config = config

	// Update transparent compression config
	transparentConfig := &TransparentConfig{
		Enabled:            config.Enabled,
		MinSize:            config.CompressionThreshold,
		ColdThreshold:      24 * time.Hour,
		DefaultAlgorithm:   "lz4",
		CompressionLevel:   1,
		BackgroundCompress: config.BackgroundCompression,
	}
	cm.transparent.UpdateConfig(transparentConfig)

	// Update cold detector config
	coldConfig := cm.coldDetector.GetConfig()
	coldConfig.Enabled = config.Enabled
	coldConfig.SizeThreshold = config.CompressionThreshold
	cm.coldDetector.UpdateConfig(coldConfig)

	return nil
}

// Shutdown gracefully shuts down the compression manager
func (cm *CompressionManager) Shutdown() {
	if cm.backgroundWorker != nil {
		cm.backgroundWorker.Stop()
	}
}
