package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// BackupScheduler handles automated backup scheduling with cron-like syntax
type BackupScheduler struct {
	mu sync.RWMutex

	// Dependencies
	snapshotManager *SnapshotManager
	streamer        *BackupStreamer

	// Configuration
	config *SchedulerConfig

	// Scheduling
	cron      *CronScheduler
	schedules map[string]*BackupSchedule
	running   bool

	// State tracking
	scheduledBackups map[string]*ScheduledBackup
	retentionManager *RetentionManager
}

// SchedulerConfig holds configuration for backup scheduling
type SchedulerConfig struct {
	ScheduleFile    string        // File to persist schedules
	MaxConcurrent   int           // Maximum concurrent scheduled backups
	DefaultTimeout  time.Duration // Default timeout for scheduled backups
	RetryAttempts   int           // Number of retry attempts for failed backups
	RetryDelay      time.Duration // Delay between retry attempts
	NotifyOnFailure bool          // Whether to send notifications on failure
	NotifyOnSuccess bool          // Whether to send notifications on success
}

// BackupSchedule represents a scheduled backup configuration
type BackupSchedule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	CronExpr    string                 `json:"cron_expression"`
	Enabled     bool                   `json:"enabled"`
	Destination BackupDestination      `json:"destination"`
	Options     BackupOptions          `json:"options"`
	Retention   RetentionPolicy        `json:"retention"`
	Tags        map[string]string      `json:"tags"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	NextRun     *time.Time             `json:"next_run,omitempty"`
	RunCount    int64                  `json:"run_count"`
	FailCount   int64                  `json:"fail_count"`
	Metadata    map[string]interface{} `json:"metadata"`

	// Internal cron entry ID
	cronEntryID EntryID `json:"-"`
}

// BackupOptions holds options for scheduled backups
type BackupOptions struct {
	CompressionType string            `json:"compression_type"`
	VerifyChecksum  bool              `json:"verify_checksum"`
	Timeout         time.Duration     `json:"timeout"`
	Tags            map[string]string `json:"tags"`
	Priority        int               `json:"priority"` // Higher number = higher priority
}

// ScheduledBackup represents an instance of a scheduled backup execution
type ScheduledBackup struct {
	ID          string     `json:"id"`
	ScheduleID  string     `json:"schedule_id"`
	SnapshotID  string     `json:"snapshot_id,omitempty"`
	StreamID    string     `json:"stream_id,omitempty"`
	Status      string     `json:"status"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Error       string     `json:"error,omitempty"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
}

// RetentionManager handles backup retention policies and cleanup
type RetentionManager struct {
	mu sync.RWMutex

	config   *RetentionConfig
	policies map[string]*RetentionPolicy
}

// RetentionConfig holds configuration for retention management
type RetentionConfig struct {
	CleanupInterval time.Duration // How often to run cleanup
	DefaultPolicy   RetentionPolicy
}

// RetentionPolicy defines how long to keep backups
type RetentionPolicy struct {
	KeepDaily   int           `json:"keep_daily"`   // Number of daily backups to keep
	KeepWeekly  int           `json:"keep_weekly"`  // Number of weekly backups to keep
	KeepMonthly int           `json:"keep_monthly"` // Number of monthly backups to keep
	KeepYearly  int           `json:"keep_yearly"`  // Number of yearly backups to keep
	MaxAge      time.Duration `json:"max_age"`      // Maximum age of backups
	MaxCount    int           `json:"max_count"`    // Maximum number of backups to keep
	MinFree     int64         `json:"min_free"`     // Minimum free space to maintain (bytes)
}

// BackupInfo represents information about a completed backup
type BackupInfo struct {
	ID          string                 `json:"id"`
	ScheduleID  string                 `json:"schedule_id,omitempty"`
	SnapshotID  string                 `json:"snapshot_id"`
	StreamID    string                 `json:"stream_id,omitempty"`
	Destination BackupDestination      `json:"destination"`
	Size        int64                  `json:"size"`
	Checksum    string                 `json:"checksum"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt time.Time              `json:"completed_at"`
	Tags        map[string]string      `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewBackupScheduler creates a new backup scheduler
func NewBackupScheduler(config *SchedulerConfig, snapshotMgr *SnapshotManager,
	streamer *BackupStreamer) (*BackupScheduler, error) {

	if config == nil {
		config = DefaultSchedulerConfig()
	}

	scheduler := &BackupScheduler{
		snapshotManager:  snapshotMgr,
		streamer:         streamer,
		config:           config,
		cron:             NewCronScheduler(),
		schedules:        make(map[string]*BackupSchedule),
		scheduledBackups: make(map[string]*ScheduledBackup),
		retentionManager: NewRetentionManager(DefaultRetentionConfig()),
	}

	// Load existing schedules
	if err := scheduler.loadSchedules(); err != nil {
		return nil, fmt.Errorf("failed to load schedules: %w", err)
	}

	return scheduler, nil
}

// DefaultSchedulerConfig returns default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		ScheduleFile:    "data/backup_schedules.json",
		MaxConcurrent:   3,
		DefaultTimeout:  2 * time.Hour,
		RetryAttempts:   3,
		RetryDelay:      5 * time.Minute,
		NotifyOnFailure: true,
		NotifyOnSuccess: false,
	}
}

// Start starts the backup scheduler
func (bs *BackupScheduler) Start() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.running {
		return fmt.Errorf("scheduler is already running")
	}

	// Start cron scheduler
	bs.cron.Start()
	bs.running = true

	// Start retention manager
	go bs.retentionManager.Start()

	return nil
}

// Stop stops the backup scheduler
func (bs *BackupScheduler) Stop() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.running {
		return nil
	}

	// Stop cron scheduler
	ctx := bs.cron.Stop()
	<-ctx.Done() // Wait for running jobs to complete

	bs.running = false

	// Stop retention manager
	bs.retentionManager.Stop()

	return nil
}

// CreateSchedule creates a new backup schedule
func (bs *BackupScheduler) CreateSchedule(schedule *BackupSchedule) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Validate cron expression
	if _, err := ParseStandard(schedule.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Set metadata
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	// Add to schedules
	bs.schedules[schedule.ID] = schedule

	// Add to cron if enabled
	if schedule.Enabled {
		if err := bs.addToCron(schedule); err != nil {
			return fmt.Errorf("failed to add schedule to cron: %w", err)
		}
	}

	// Save schedules
	return bs.saveSchedules()
}

// UpdateSchedule updates an existing backup schedule
func (bs *BackupScheduler) UpdateSchedule(schedule *BackupSchedule) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	existing, exists := bs.schedules[schedule.ID]
	if !exists {
		return fmt.Errorf("schedule %s not found", schedule.ID)
	}

	// Remove from cron if it was enabled
	if existing.Enabled && existing.cronEntryID != 0 {
		bs.cron.Remove(existing.cronEntryID)
	}

	// Update metadata
	schedule.CreatedAt = existing.CreatedAt
	schedule.UpdatedAt = time.Now()
	schedule.RunCount = existing.RunCount
	schedule.FailCount = existing.FailCount

	// Update schedule
	bs.schedules[schedule.ID] = schedule

	// Add to cron if enabled
	if schedule.Enabled {
		if err := bs.addToCron(schedule); err != nil {
			return fmt.Errorf("failed to add updated schedule to cron: %w", err)
		}
	}

	// Save schedules
	return bs.saveSchedules()
}

// DeleteSchedule deletes a backup schedule
func (bs *BackupScheduler) DeleteSchedule(id string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	schedule, exists := bs.schedules[id]
	if !exists {
		return fmt.Errorf("schedule %s not found", id)
	}

	// Remove from cron
	if schedule.cronEntryID != 0 {
		bs.cron.Remove(schedule.cronEntryID)
	}

	// Remove from schedules
	delete(bs.schedules, id)

	// Save schedules
	return bs.saveSchedules()
}

// GetSchedule retrieves a schedule by ID
func (bs *BackupScheduler) GetSchedule(id string) (*BackupSchedule, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	schedule, exists := bs.schedules[id]
	if !exists {
		return nil, fmt.Errorf("schedule %s not found", id)
	}

	return schedule, nil
}

// ListSchedules returns all schedules
func (bs *BackupScheduler) ListSchedules() []*BackupSchedule {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	schedules := make([]*BackupSchedule, 0, len(bs.schedules))
	for _, schedule := range bs.schedules {
		schedules = append(schedules, schedule)
	}

	// Sort by creation time
	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].CreatedAt.Before(schedules[j].CreatedAt)
	})

	return schedules
}

// RunScheduleNow executes a schedule immediately
func (bs *BackupScheduler) RunScheduleNow(ctx context.Context, scheduleID string) (*ScheduledBackup, error) {
	schedule, err := bs.GetSchedule(scheduleID)
	if err != nil {
		return nil, err
	}

	return bs.executeSchedule(ctx, schedule)
}

// addToCron adds a schedule to the cron scheduler
func (bs *BackupScheduler) addToCron(schedule *BackupSchedule) error {
	entryID, err := bs.cron.AddFunc(schedule.CronExpr, func() {
		ctx := context.Background()
		if _, err := bs.executeSchedule(ctx, schedule); err != nil {
			// Log error but don't fail
			fmt.Printf("Scheduled backup %s failed: %v\n", schedule.ID, err)
		}
	})

	if err != nil {
		return err
	}

	schedule.cronEntryID = entryID

	// Update next run time
	entries := bs.cron.Entries()
	for _, entry := range entries {
		if entry.ID == entryID {
			schedule.NextRun = &entry.Next
			break
		}
	}

	return nil
}

// executeSchedule executes a backup schedule
func (bs *BackupScheduler) executeSchedule(ctx context.Context, schedule *BackupSchedule) (*ScheduledBackup, error) {
	// Check concurrent backup limit
	bs.mu.RLock()
	activeCount := 0
	for _, backup := range bs.scheduledBackups {
		if backup.Status == "running" {
			activeCount++
		}
	}
	bs.mu.RUnlock()

	if activeCount >= bs.config.MaxConcurrent {
		return nil, fmt.Errorf("maximum concurrent backups (%d) reached", bs.config.MaxConcurrent)
	}

	// Create scheduled backup
	backupID := fmt.Sprintf("backup_%s_%d", schedule.ID, time.Now().Unix())
	scheduledBackup := &ScheduledBackup{
		ID:          backupID,
		ScheduleID:  schedule.ID,
		Status:      "running",
		StartTime:   time.Now(),
		Attempt:     1,
		MaxAttempts: bs.config.RetryAttempts,
	}

	bs.mu.Lock()
	bs.scheduledBackups[backupID] = scheduledBackup
	bs.mu.Unlock()

	// Execute backup with retries
	go bs.executeBackupWithRetries(ctx, schedule, scheduledBackup)

	return scheduledBackup, nil
}

// executeBackupWithRetries executes a backup with retry logic
func (bs *BackupScheduler) executeBackupWithRetries(ctx context.Context,
	schedule *BackupSchedule, scheduledBackup *ScheduledBackup) {

	defer func() {
		bs.mu.Lock()
		delete(bs.scheduledBackups, scheduledBackup.ID)
		bs.mu.Unlock()
	}()

	var lastErr error

	for attempt := 1; attempt <= scheduledBackup.MaxAttempts; attempt++ {
		scheduledBackup.Attempt = attempt

		if err := bs.executeBackupAttempt(ctx, schedule, scheduledBackup); err != nil {
			lastErr = err

			// If not the last attempt, wait before retrying
			if attempt < scheduledBackup.MaxAttempts {
				time.Sleep(bs.config.RetryDelay)
				continue
			}
		} else {
			// Success
			bs.markBackupCompleted(schedule, scheduledBackup)
			return
		}
	}

	// All attempts failed
	bs.markBackupFailed(schedule, scheduledBackup, lastErr)
}

// executeBackupAttempt executes a single backup attempt
func (bs *BackupScheduler) executeBackupAttempt(ctx context.Context,
	schedule *BackupSchedule, scheduledBackup *ScheduledBackup) error {

	// Create snapshot
	tags := make(map[string]string)
	for k, v := range schedule.Tags {
		tags[k] = v
	}
	for k, v := range schedule.Options.Tags {
		tags[k] = v
	}
	tags["schedule_id"] = schedule.ID
	tags["backup_id"] = scheduledBackup.ID

	snapshot, err := bs.snapshotManager.CreateSnapshot(ctx, tags)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	scheduledBackup.SnapshotID = snapshot.ID

	// Wait for snapshot to complete
	if err := bs.waitForSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("snapshot creation failed: %w", err)
	}

	// Stream backup to destination
	stream, err := bs.streamer.StreamBackup(ctx, snapshot.ID, schedule.Destination)
	if err != nil {
		return fmt.Errorf("failed to start backup stream: %w", err)
	}

	scheduledBackup.StreamID = stream.ID

	// Wait for stream to complete
	if err := bs.waitForStream(ctx, stream); err != nil {
		return fmt.Errorf("backup streaming failed: %w", err)
	}

	return nil
}

// waitForSnapshot waits for a snapshot to complete
func (bs *BackupScheduler) waitForSnapshot(ctx context.Context, snapshot *Snapshot) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := bs.snapshotManager.GetSnapshot(snapshot.ID)
			if err != nil {
				return err
			}

			switch current.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("snapshot failed: %s", current.Error)
			}
		}
	}
}

// waitForStream waits for a backup stream to complete
func (bs *BackupScheduler) waitForStream(ctx context.Context, stream *BackupStream) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := bs.streamer.GetStream(stream.ID)
			if err != nil {
				return err
			}

			switch current.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("stream failed: %s", current.Error)
			case "cancelled":
				return fmt.Errorf("stream was cancelled")
			}
		}
	}
}

// markBackupCompleted marks a backup as completed
func (bs *BackupScheduler) markBackupCompleted(schedule *BackupSchedule, backup *ScheduledBackup) {
	now := time.Now()
	backup.Status = "completed"
	backup.EndTime = &now

	// Update schedule statistics
	bs.mu.Lock()
	schedule.LastRun = &now
	schedule.RunCount++
	bs.mu.Unlock()

	// Send notification if configured
	if bs.config.NotifyOnSuccess {
		bs.sendNotification("success", schedule, backup, nil)
	}
}

// markBackupFailed marks a backup as failed
func (bs *BackupScheduler) markBackupFailed(schedule *BackupSchedule, backup *ScheduledBackup, err error) {
	now := time.Now()
	backup.Status = "failed"
	backup.Error = err.Error()
	backup.EndTime = &now

	// Update schedule statistics
	bs.mu.Lock()
	schedule.FailCount++
	bs.mu.Unlock()

	// Send notification if configured
	if bs.config.NotifyOnFailure {
		bs.sendNotification("failure", schedule, backup, err)
	}
}

// sendNotification sends a notification about backup status
func (bs *BackupScheduler) sendNotification(status string, schedule *BackupSchedule,
	backup *ScheduledBackup, err error) {
	// This would integrate with notification systems (email, Slack, etc.)
	// For now, just log
	if status == "success" {
		fmt.Printf("Backup %s completed successfully\n", backup.ID)
	} else {
		fmt.Printf("Backup %s failed: %v\n", backup.ID, err)
	}
}

// loadSchedules loads schedules from file
func (bs *BackupScheduler) loadSchedules() error {
	if _, err := os.Stat(bs.config.ScheduleFile); os.IsNotExist(err) {
		return nil // No schedules file exists yet
	}

	data, err := os.ReadFile(bs.config.ScheduleFile)
	if err != nil {
		return fmt.Errorf("failed to read schedules file: %w", err)
	}

	var schedules map[string]*BackupSchedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		return fmt.Errorf("failed to unmarshal schedules: %w", err)
	}

	bs.schedules = schedules

	// Add enabled schedules to cron
	for _, schedule := range schedules {
		if schedule.Enabled {
			if err := bs.addToCron(schedule); err != nil {
				return fmt.Errorf("failed to add schedule %s to cron: %w", schedule.ID, err)
			}
		}
	}

	return nil
}

// saveSchedules saves schedules to file
func (bs *BackupScheduler) saveSchedules() error {
	// Ensure directory exists
	dir := filepath.Dir(bs.config.ScheduleFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create schedules directory: %w", err)
	}

	data, err := json.MarshalIndent(bs.schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	return os.WriteFile(bs.config.ScheduleFile, data, 0644)
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(config *RetentionConfig) *RetentionManager {
	if config == nil {
		config = DefaultRetentionConfig()
	}

	return &RetentionManager{
		config:   config,
		policies: make(map[string]*RetentionPolicy),
	}
}

// DefaultRetentionConfig returns default retention configuration
func DefaultRetentionConfig() *RetentionConfig {
	return &RetentionConfig{
		CleanupInterval: 24 * time.Hour, // Daily cleanup
		DefaultPolicy: RetentionPolicy{
			KeepDaily:   7,                    // Keep 7 daily backups
			KeepWeekly:  4,                    // Keep 4 weekly backups
			KeepMonthly: 12,                   // Keep 12 monthly backups
			KeepYearly:  5,                    // Keep 5 yearly backups
			MaxAge:      365 * 24 * time.Hour, // 1 year
			MaxCount:    100,                  // Maximum 100 backups
		},
	}
}

// Start starts the retention manager
func (rm *RetentionManager) Start() {
	ticker := time.NewTicker(rm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := rm.RunCleanup(); err != nil {
			fmt.Printf("Retention cleanup failed: %v\n", err)
		}
	}
}

// Stop stops the retention manager
func (rm *RetentionManager) Stop() {
	// Implementation would stop the cleanup goroutine
}

// RunCleanup runs the retention cleanup process
func (rm *RetentionManager) RunCleanup() error {
	// This would implement the actual cleanup logic
	// For now, just return nil
	return nil
}

// SetPolicy sets a retention policy for a schedule
func (rm *RetentionManager) SetPolicy(scheduleID string, policy *RetentionPolicy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.policies[scheduleID] = policy
}

// GetPolicy gets the retention policy for a schedule
func (rm *RetentionManager) GetPolicy(scheduleID string) *RetentionPolicy {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if policy, exists := rm.policies[scheduleID]; exists {
		return policy
	}

	return &rm.config.DefaultPolicy
}

// Simple cron implementation for demonstration
type CronScheduler struct {
	entries map[EntryID]*CronEntry
	nextID  EntryID
	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

type EntryID int

type CronEntry struct {
	ID       EntryID
	Schedule string
	Job      func()
	Next     time.Time
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{
		entries: make(map[EntryID]*CronEntry),
		nextID:  1,
		stopCh:  make(chan struct{}),
	}
}

func (c *CronScheduler) AddFunc(schedule string, job func()) (EntryID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &CronEntry{
		ID:       c.nextID,
		Schedule: schedule,
		Job:      job,
		Next:     time.Now().Add(24 * time.Hour), // Simplified: run daily
	}

	c.entries[c.nextID] = entry
	c.nextID++

	return entry.ID, nil
}

func (c *CronScheduler) Remove(id EntryID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

func (c *CronScheduler) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go c.run()
}

func (c *CronScheduler) Stop() context.Context {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func (c *CronScheduler) Entries() []CronEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]CronEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, *entry)
	}
	return entries
}

func (c *CronScheduler) run() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.checkAndRunJobs()
		}
	}
}

func (c *CronScheduler) checkAndRunJobs() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	for _, entry := range c.entries {
		if now.After(entry.Next) {
			go entry.Job()
			entry.Next = now.Add(24 * time.Hour) // Simplified: run daily
		}
	}
}

// ParseStandard is a placeholder for cron expression parsing
func ParseStandard(expr string) (interface{}, error) {
	// Simplified validation - just check it's not empty
	if expr == "" {
		return nil, fmt.Errorf("empty cron expression")
	}
	return expr, nil
}
