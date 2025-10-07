package concurrency

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// GoroutineManager manages the lifecycle of goroutines to prevent leaks
type GoroutineManager struct {
	// Goroutine pools
	workerPool  *WorkerPool
	cleanupPool *WorkerPool

	// Tracking
	activeGoroutines sync.Map // goroutineID -> *GoroutineInfo
	goroutineCounter int64    // atomic

	// Monitoring
	monitor      *GoroutineMonitor
	leakDetector *LeakDetector

	// Configuration
	maxGoroutines   int
	cleanupInterval time.Duration

	// Lifecycle
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  int32 // atomic
	stopChan chan struct{}
}

// GoroutineInfo tracks information about a managed goroutine
type GoroutineInfo struct {
	ID        int64
	Name      string
	StartTime time.Time
	LastSeen  time.Time
	Function  string
	Status    GoroutineStatus
	Context   context.Context
	Cancel    context.CancelFunc
	mutex     sync.RWMutex
}

// GoroutineStatus represents the status of a goroutine
type GoroutineStatus int

const (
	StatusRunning GoroutineStatus = iota
	StatusIdle
	StatusBlocked
	StatusStopping
	StatusStopped
)

func (s GoroutineStatus) String() string {
	switch s {
	case StatusRunning:
		return "RUNNING"
	case StatusIdle:
		return "IDLE"
	case StatusBlocked:
		return "BLOCKED"
	case StatusStopping:
		return "STOPPING"
	case StatusStopped:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	name        string
	workers     []*Worker
	workChan    chan WorkItem
	maxWorkers  int
	minWorkers  int
	idleTimeout time.Duration

	// Metrics
	activeWorkers int32 // atomic
	totalJobs     int64 // atomic
	completedJobs int64 // atomic

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running int32 // atomic
	mutex   sync.RWMutex
}

// Worker represents a single worker goroutine
type Worker struct {
	id          int
	pool        *WorkerPool
	lastActive  time.Time
	jobsHandled int64 // atomic
	ctx         context.Context
	cancel      context.CancelFunc
	mutex       sync.RWMutex
}

// WorkItem represents a unit of work to be processed
type WorkItem struct {
	ID       string
	Function func(context.Context) error
	Priority int
	Timeout  time.Duration
	Callback func(error)
}

// GoroutineMonitor monitors goroutine health and performance
type GoroutineMonitor struct {
	manager       *GoroutineManager
	checkInterval time.Duration
	maxIdleTime   time.Duration

	// Metrics
	totalGoroutines   int64 // atomic
	leakedGoroutines  int64 // atomic
	cleanedGoroutines int64 // atomic

	// Lifecycle
	stopChan chan struct{}
	wg       sync.WaitGroup
	running  int32 // atomic
}

// LeakDetector detects and handles goroutine leaks
type LeakDetector struct {
	manager           *GoroutineManager
	detectionInterval time.Duration
	leakThreshold     time.Duration
	maxLeaks          int

	// Detected leaks
	suspectedLeaks sync.Map // goroutineID -> *LeakInfo
	confirmedLeaks sync.Map // goroutineID -> *LeakInfo

	// Metrics
	leaksDetected int64 // atomic
	leaksFixed    int64 // atomic

	// Lifecycle
	stopChan chan struct{}
	wg       sync.WaitGroup
	running  int32 // atomic
}

// LeakInfo contains information about a suspected or confirmed leak
type LeakInfo struct {
	GoroutineID  int64
	DetectedAt   time.Time
	LastActivity time.Time
	StackTrace   string
	LeakScore    float64
	Attempts     int
}

// NewGoroutineManager creates a new goroutine manager
func NewGoroutineManager(config *GoroutineManagerConfig) *GoroutineManager {
	if config == nil {
		config = DefaultGoroutineManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	gm := &GoroutineManager{
		maxGoroutines:   config.MaxGoroutines,
		cleanupInterval: config.CleanupInterval,
		ctx:             ctx,
		cancel:          cancel,
		stopChan:        make(chan struct{}),
	}

	// Initialize worker pools
	gm.workerPool = NewWorkerPool("main", config.WorkerPoolConfig)
	gm.cleanupPool = NewWorkerPool("cleanup", &WorkerPoolConfig{
		MinWorkers:  1,
		MaxWorkers:  2,
		IdleTimeout: 30 * time.Second,
	})

	// Initialize monitor and leak detector
	gm.monitor = NewGoroutineMonitor(gm, config.MonitorConfig)
	gm.leakDetector = NewLeakDetector(gm, config.LeakDetectorConfig)

	return gm
}

// GoroutineManagerConfig holds configuration for the goroutine manager
type GoroutineManagerConfig struct {
	MaxGoroutines      int
	CleanupInterval    time.Duration
	WorkerPoolConfig   *WorkerPoolConfig
	MonitorConfig      *GoroutineMonitorConfig
	LeakDetectorConfig *LeakDetectorConfig
}

// WorkerPoolConfig holds configuration for worker pools
type WorkerPoolConfig struct {
	MinWorkers  int
	MaxWorkers  int
	IdleTimeout time.Duration
}

// GoroutineMonitorConfig holds configuration for goroutine monitoring
type GoroutineMonitorConfig struct {
	CheckInterval time.Duration
	MaxIdleTime   time.Duration
}

// LeakDetectorConfig holds configuration for leak detection
type LeakDetectorConfig struct {
	DetectionInterval time.Duration
	LeakThreshold     time.Duration
	MaxLeaks          int
}

// DefaultGoroutineManagerConfig returns default configuration
func DefaultGoroutineManagerConfig() *GoroutineManagerConfig {
	return &GoroutineManagerConfig{
		MaxGoroutines:   1000,
		CleanupInterval: 30 * time.Second,
		WorkerPoolConfig: &WorkerPoolConfig{
			MinWorkers:  2,
			MaxWorkers:  10,
			IdleTimeout: 60 * time.Second,
		},
		MonitorConfig: &GoroutineMonitorConfig{
			CheckInterval: 10 * time.Second,
			MaxIdleTime:   5 * time.Minute,
		},
		LeakDetectorConfig: &LeakDetectorConfig{
			DetectionInterval: 30 * time.Second,
			LeakThreshold:     10 * time.Minute,
			MaxLeaks:          50,
		},
	}
}

// Start starts the goroutine manager
func (gm *GoroutineManager) Start() error {
	if !atomic.CompareAndSwapInt32(&gm.running, 0, 1) {
		return fmt.Errorf("goroutine manager is already running")
	}

	// Start worker pools
	if err := gm.workerPool.Start(gm.ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	if err := gm.cleanupPool.Start(gm.ctx); err != nil {
		return fmt.Errorf("failed to start cleanup pool: %w", err)
	}

	// Start monitor and leak detector
	if err := gm.monitor.Start(gm.ctx); err != nil {
		return fmt.Errorf("failed to start monitor: %w", err)
	}

	if err := gm.leakDetector.Start(gm.ctx); err != nil {
		return fmt.Errorf("failed to start leak detector: %w", err)
	}

	// Start cleanup routine
	gm.wg.Add(1)
	go gm.cleanupLoop()

	return nil
}

// Stop stops the goroutine manager and all managed goroutines
func (gm *GoroutineManager) Stop() error {
	if !atomic.CompareAndSwapInt32(&gm.running, 1, 0) {
		return nil
	}

	// Cancel all managed goroutines
	gm.cancel()

	// Stop components
	gm.monitor.Stop()
	gm.leakDetector.Stop()
	gm.workerPool.Stop()
	gm.cleanupPool.Stop()

	// Stop cleanup loop
	close(gm.stopChan)
	gm.wg.Wait()

	// Force cleanup any remaining goroutines
	gm.forceCleanup()

	return nil
}

// SpawnGoroutine spawns a new managed goroutine
func (gm *GoroutineManager) SpawnGoroutine(name string, fn func(context.Context)) (*GoroutineInfo, error) {
	if atomic.LoadInt32(&gm.running) == 0 {
		return nil, fmt.Errorf("goroutine manager is not running")
	}

	// Check goroutine limit
	current := atomic.LoadInt64(&gm.goroutineCounter)
	if int(current) >= gm.maxGoroutines {
		return nil, fmt.Errorf("maximum goroutines limit reached: %d", gm.maxGoroutines)
	}

	// Create goroutine info
	id := atomic.AddInt64(&gm.goroutineCounter, 1)
	ctx, cancel := context.WithCancel(gm.ctx)

	info := &GoroutineInfo{
		ID:        id,
		Name:      name,
		StartTime: time.Now(),
		LastSeen:  time.Now(),
		Function:  runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(),
		Status:    StatusRunning,
		Context:   ctx,
		Cancel:    cancel,
	}

	// Register goroutine
	gm.activeGoroutines.Store(id, info)

	// Start goroutine
	go func() {
		defer func() {
			// Handle panics
			if r := recover(); r != nil {
				// Log panic in production
				fmt.Printf("Goroutine %d (%s) panicked: %v\n", id, name, r)
			}

			// Update status and cleanup
			info.mutex.Lock()
			info.Status = StatusStopped
			info.mutex.Unlock()

			gm.activeGoroutines.Delete(id)
			cancel()
		}()

		// Update last seen periodically
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-ticker.C:
					info.mutex.Lock()
					info.LastSeen = time.Now()
					info.mutex.Unlock()
				case <-ctx.Done():
					return
				}
			}
		}()

		// Execute function
		fn(ctx)
	}()

	return info, nil
}

// SubmitWork submits work to the worker pool
func (gm *GoroutineManager) SubmitWork(item WorkItem) error {
	if atomic.LoadInt32(&gm.running) == 0 {
		return fmt.Errorf("goroutine manager is not running")
	}

	return gm.workerPool.SubmitWork(item)
}

// GetGoroutineInfo returns information about a specific goroutine
func (gm *GoroutineManager) GetGoroutineInfo(id int64) (*GoroutineInfo, bool) {
	value, exists := gm.activeGoroutines.Load(id)
	if !exists {
		return nil, false
	}

	info := value.(*GoroutineInfo)
	info.mutex.RLock()
	defer info.mutex.RUnlock()

	// Return a copy to avoid race conditions
	return &GoroutineInfo{
		ID:        info.ID,
		Name:      info.Name,
		StartTime: info.StartTime,
		LastSeen:  info.LastSeen,
		Function:  info.Function,
		Status:    info.Status,
	}, true
}

// GetActiveGoroutines returns information about all active goroutines
func (gm *GoroutineManager) GetActiveGoroutines() []*GoroutineInfo {
	var goroutines []*GoroutineInfo

	gm.activeGoroutines.Range(func(key, value interface{}) bool {
		info := value.(*GoroutineInfo)
		info.mutex.RLock()

		goroutines = append(goroutines, &GoroutineInfo{
			ID:        info.ID,
			Name:      info.Name,
			StartTime: info.StartTime,
			LastSeen:  info.LastSeen,
			Function:  info.Function,
			Status:    info.Status,
		})

		info.mutex.RUnlock()
		return true
	})

	return goroutines
}

// GetStats returns goroutine manager statistics
func (gm *GoroutineManager) GetStats() *GoroutineStats {
	activeCount := int64(0)
	gm.activeGoroutines.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	return &GoroutineStats{
		ActiveGoroutines: activeCount,
		TotalSpawned:     atomic.LoadInt64(&gm.goroutineCounter),
		LeaksDetected:    atomic.LoadInt64(&gm.leakDetector.leaksDetected),
		LeaksFixed:       atomic.LoadInt64(&gm.leakDetector.leaksFixed),
		WorkerPoolStats:  gm.workerPool.GetStats(),
		CleanupPoolStats: gm.cleanupPool.GetStats(),
	}
}

// GoroutineStats contains statistics about goroutine management
type GoroutineStats struct {
	ActiveGoroutines int64
	TotalSpawned     int64
	LeaksDetected    int64
	LeaksFixed       int64
	WorkerPoolStats  *WorkerPoolStats
	CleanupPoolStats *WorkerPoolStats
}

// cleanupLoop runs periodic cleanup of inactive goroutines
func (gm *GoroutineManager) cleanupLoop() {
	defer gm.wg.Done()

	ticker := time.NewTicker(gm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gm.performCleanup()
		case <-gm.stopChan:
			return
		case <-gm.ctx.Done():
			return
		}
	}
}

// performCleanup performs cleanup of inactive goroutines
func (gm *GoroutineManager) performCleanup() {
	now := time.Now()
	toCleanup := make([]int64, 0)

	gm.activeGoroutines.Range(func(key, value interface{}) bool {
		id := key.(int64)
		info := value.(*GoroutineInfo)

		info.mutex.RLock()
		lastSeen := info.LastSeen
		status := info.Status
		info.mutex.RUnlock()

		// Check if goroutine should be cleaned up
		if status == StatusStopped || now.Sub(lastSeen) > 5*time.Minute {
			toCleanup = append(toCleanup, id)
		}

		return true
	})

	// Cleanup identified goroutines
	for _, id := range toCleanup {
		if value, exists := gm.activeGoroutines.LoadAndDelete(id); exists {
			info := value.(*GoroutineInfo)
			info.Cancel()
		}
	}
}

// forceCleanup forcefully cleans up all remaining goroutines
func (gm *GoroutineManager) forceCleanup() {
	gm.activeGoroutines.Range(func(key, value interface{}) bool {
		info := value.(*GoroutineInfo)
		info.Cancel()
		gm.activeGoroutines.Delete(key)
		return true
	})
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(name string, config *WorkerPoolConfig) *WorkerPool {
	if config == nil {
		config = &WorkerPoolConfig{
			MinWorkers:  2,
			MaxWorkers:  10,
			IdleTimeout: 60 * time.Second,
		}
	}

	return &WorkerPool{
		name:        name,
		workChan:    make(chan WorkItem, config.MaxWorkers*2),
		maxWorkers:  config.MaxWorkers,
		minWorkers:  config.MinWorkers,
		idleTimeout: config.IdleTimeout,
		workers:     make([]*Worker, 0, config.MaxWorkers),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&wp.running, 0, 1) {
		return fmt.Errorf("worker pool %s is already running", wp.name)
	}

	wp.ctx, wp.cancel = context.WithCancel(ctx)

	// Start minimum number of workers
	for i := 0; i < wp.minWorkers; i++ {
		wp.addWorker()
	}

	return nil
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&wp.running, 1, 0) {
		return nil
	}

	wp.cancel()
	close(wp.workChan)
	wp.wg.Wait()

	return nil
}

// SubmitWork submits work to the pool
func (wp *WorkerPool) SubmitWork(item WorkItem) error {
	if atomic.LoadInt32(&wp.running) == 0 {
		return fmt.Errorf("worker pool %s is not running", wp.name)
	}

	select {
	case wp.workChan <- item:
		atomic.AddInt64(&wp.totalJobs, 1)
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool %s is shutting down", wp.name)
	default:
		// Pool is full, try to add more workers
		if wp.canAddWorker() {
			wp.addWorker()
		}

		// Try again
		select {
		case wp.workChan <- item:
			atomic.AddInt64(&wp.totalJobs, 1)
			return nil
		case <-time.After(time.Second):
			return fmt.Errorf("worker pool %s is overloaded", wp.name)
		}
	}
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() *WorkerPoolStats {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	return &WorkerPoolStats{
		Name:          wp.name,
		ActiveWorkers: atomic.LoadInt32(&wp.activeWorkers),
		TotalWorkers:  int32(len(wp.workers)),
		TotalJobs:     atomic.LoadInt64(&wp.totalJobs),
		CompletedJobs: atomic.LoadInt64(&wp.completedJobs),
		QueueLength:   int32(len(wp.workChan)),
	}
}

// WorkerPoolStats contains statistics about a worker pool
type WorkerPoolStats struct {
	Name          string
	ActiveWorkers int32
	TotalWorkers  int32
	TotalJobs     int64
	CompletedJobs int64
	QueueLength   int32
}

// canAddWorker checks if more workers can be added
func (wp *WorkerPool) canAddWorker() bool {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	return len(wp.workers) < wp.maxWorkers
}

// addWorker adds a new worker to the pool
func (wp *WorkerPool) addWorker() {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	if len(wp.workers) >= wp.maxWorkers {
		return
	}

	workerID := len(wp.workers)
	ctx, cancel := context.WithCancel(wp.ctx)

	worker := &Worker{
		id:         workerID,
		pool:       wp,
		lastActive: time.Now(),
		ctx:        ctx,
		cancel:     cancel,
	}

	wp.workers = append(wp.workers, worker)
	atomic.AddInt32(&wp.activeWorkers, 1)

	wp.wg.Add(1)
	go worker.run()
}

// run runs the worker loop
func (w *Worker) run() {
	defer func() {
		w.pool.wg.Done()
		atomic.AddInt32(&w.pool.activeWorkers, -1)
	}()

	idleTimer := time.NewTimer(w.pool.idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case work, ok := <-w.pool.workChan:
			if !ok {
				return // Channel closed
			}

			w.handleWork(work)
			idleTimer.Reset(w.pool.idleTimeout)

		case <-idleTimer.C:
			// Worker has been idle too long
			if w.pool.canRemoveWorker() {
				return
			}
			idleTimer.Reset(w.pool.idleTimeout)

		case <-w.ctx.Done():
			return
		}
	}
}

// handleWork processes a work item
func (w *Worker) handleWork(work WorkItem) {
	w.mutex.Lock()
	w.lastActive = time.Now()
	w.mutex.Unlock()

	atomic.AddInt64(&w.jobsHandled, 1)

	// Create context with timeout if specified
	ctx := w.ctx
	if work.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, work.Timeout)
		defer cancel()
	}

	// Execute work
	err := work.Function(ctx)

	// Call callback if provided
	if work.Callback != nil {
		work.Callback(err)
	}

	atomic.AddInt64(&w.pool.completedJobs, 1)
}

// canRemoveWorker checks if this worker can be removed
func (wp *WorkerPool) canRemoveWorker() bool {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	return len(wp.workers) > wp.minWorkers
}

// NewGoroutineMonitor creates a new goroutine monitor
func NewGoroutineMonitor(manager *GoroutineManager, config *GoroutineMonitorConfig) *GoroutineMonitor {
	if config == nil {
		config = &GoroutineMonitorConfig{
			CheckInterval: 10 * time.Second,
			MaxIdleTime:   5 * time.Minute,
		}
	}

	return &GoroutineMonitor{
		manager:       manager,
		checkInterval: config.CheckInterval,
		maxIdleTime:   config.MaxIdleTime,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the goroutine monitor
func (gm *GoroutineMonitor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&gm.running, 0, 1) {
		return fmt.Errorf("goroutine monitor is already running")
	}

	gm.wg.Add(1)
	go gm.monitorLoop(ctx)

	return nil
}

// Stop stops the goroutine monitor
func (gm *GoroutineMonitor) Stop() error {
	if !atomic.CompareAndSwapInt32(&gm.running, 1, 0) {
		return nil
	}

	close(gm.stopChan)
	gm.wg.Wait()

	return nil
}

// monitorLoop runs the monitoring loop
func (gm *GoroutineMonitor) monitorLoop(ctx context.Context) {
	defer gm.wg.Done()

	ticker := time.NewTicker(gm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gm.checkGoroutines()
		case <-gm.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkGoroutines checks the health of all goroutines
func (gm *GoroutineMonitor) checkGoroutines() {
	now := time.Now()
	totalCount := int64(0)

	gm.manager.activeGoroutines.Range(func(key, value interface{}) bool {
		totalCount++
		info := value.(*GoroutineInfo)

		info.mutex.RLock()
		lastSeen := info.LastSeen
		info.mutex.RUnlock()

		// Check if goroutine appears to be stuck
		if now.Sub(lastSeen) > gm.maxIdleTime {
			// Potential leak detected
			gm.manager.leakDetector.reportSuspectedLeak(info)
		}

		return true
	})

	atomic.StoreInt64(&gm.totalGoroutines, totalCount)
}

// NewLeakDetector creates a new leak detector
func NewLeakDetector(manager *GoroutineManager, config *LeakDetectorConfig) *LeakDetector {
	if config == nil {
		config = &LeakDetectorConfig{
			DetectionInterval: 30 * time.Second,
			LeakThreshold:     10 * time.Minute,
			MaxLeaks:          50,
		}
	}

	return &LeakDetector{
		manager:           manager,
		detectionInterval: config.DetectionInterval,
		leakThreshold:     config.LeakThreshold,
		maxLeaks:          config.MaxLeaks,
		stopChan:          make(chan struct{}),
	}
}

// Start starts the leak detector
func (ld *LeakDetector) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&ld.running, 0, 1) {
		return fmt.Errorf("leak detector is already running")
	}

	ld.wg.Add(1)
	go ld.detectionLoop(ctx)

	return nil
}

// Stop stops the leak detector
func (ld *LeakDetector) Stop() error {
	if !atomic.CompareAndSwapInt32(&ld.running, 1, 0) {
		return nil
	}

	close(ld.stopChan)
	ld.wg.Wait()

	return nil
}

// detectionLoop runs the leak detection loop
func (ld *LeakDetector) detectionLoop(ctx context.Context) {
	defer ld.wg.Done()

	ticker := time.NewTicker(ld.detectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ld.detectLeaks()
		case <-ld.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// detectLeaks detects and handles goroutine leaks
func (ld *LeakDetector) detectLeaks() {
	now := time.Now()

	// Check suspected leaks
	ld.suspectedLeaks.Range(func(key, value interface{}) bool {
		leakInfo := value.(*LeakInfo)

		if now.Sub(leakInfo.DetectedAt) > ld.leakThreshold {
			// Confirm leak
			ld.confirmedLeaks.Store(key, leakInfo)
			ld.suspectedLeaks.Delete(key)
			atomic.AddInt64(&ld.leaksDetected, 1)

			// Attempt to fix leak
			ld.attemptLeakFix(leakInfo)
		}

		return true
	})
}

// reportSuspectedLeak reports a suspected goroutine leak
func (ld *LeakDetector) reportSuspectedLeak(info *GoroutineInfo) {
	leakInfo := &LeakInfo{
		GoroutineID:  info.ID,
		DetectedAt:   time.Now(),
		LastActivity: info.LastSeen,
		StackTrace:   "", // Would capture stack trace in production
		LeakScore:    1.0,
		Attempts:     0,
	}

	ld.suspectedLeaks.Store(info.ID, leakInfo)
}

// attemptLeakFix attempts to fix a confirmed leak
func (ld *LeakDetector) attemptLeakFix(leakInfo *LeakInfo) {
	leakInfo.Attempts++

	// Try to cancel the goroutine
	if value, exists := ld.manager.activeGoroutines.Load(leakInfo.GoroutineID); exists {
		info := value.(*GoroutineInfo)
		info.Cancel()

		// Remove from active goroutines
		ld.manager.activeGoroutines.Delete(leakInfo.GoroutineID)
		atomic.AddInt64(&ld.leaksFixed, 1)

		// Remove from confirmed leaks
		ld.confirmedLeaks.Delete(leakInfo.GoroutineID)
	}
}
