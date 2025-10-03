package checkpoint

import (
	"time"
)

// CheckpointFilter provides filtering capabilities for checkpoint queries
type CheckpointFilter struct {
	// Status filters
	Status []CheckpointStatus `json:"status"`

	// Type filters
	Type []CheckpointType `json:"type"`

	// LSN range filters
	MinLSN *uint64 `json:"min_lsn"`
	MaxLSN *uint64 `json:"max_lsn"`

	// Time range filters
	CreatedAfter  *time.Time `json:"created_after"`
	CreatedBefore *time.Time `json:"created_before"`

	// Size filters
	MinSize *int64 `json:"min_size"`
	MaxSize *int64 `json:"max_size"`

	// Validation filters
	ValidatedOnly   bool `json:"validated_only"`
	UnvalidatedOnly bool `json:"unvalidated_only"`

	// Creator filter
	CreatedBy []string `json:"created_by"`

	// Tag filters
	Tags map[string]string `json:"tags"`

	// Limit and offset for pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewCheckpointFilter creates a new checkpoint filter
func NewCheckpointFilter() *CheckpointFilter {
	return &CheckpointFilter{
		Status:    make([]CheckpointStatus, 0),
		Type:      make([]CheckpointType, 0),
		CreatedBy: make([]string, 0),
		Tags:      make(map[string]string),
	}
}

// WithStatus adds status filter
func (f *CheckpointFilter) WithStatus(status ...CheckpointStatus) *CheckpointFilter {
	f.Status = append(f.Status, status...)
	return f
}

// WithType adds type filter
func (f *CheckpointFilter) WithType(checkpointType ...CheckpointType) *CheckpointFilter {
	f.Type = append(f.Type, checkpointType...)
	return f
}

// WithLSNRange adds LSN range filter
func (f *CheckpointFilter) WithLSNRange(minLSN, maxLSN uint64) *CheckpointFilter {
	f.MinLSN = &minLSN
	f.MaxLSN = &maxLSN
	return f
}

// WithMinLSN adds minimum LSN filter
func (f *CheckpointFilter) WithMinLSN(minLSN uint64) *CheckpointFilter {
	f.MinLSN = &minLSN
	return f
}

// WithMaxLSN adds maximum LSN filter
func (f *CheckpointFilter) WithMaxLSN(maxLSN uint64) *CheckpointFilter {
	f.MaxLSN = &maxLSN
	return f
}

// WithTimeRange adds time range filter
func (f *CheckpointFilter) WithTimeRange(after, before time.Time) *CheckpointFilter {
	f.CreatedAfter = &after
	f.CreatedBefore = &before
	return f
}

// WithCreatedAfter adds created after filter
func (f *CheckpointFilter) WithCreatedAfter(after time.Time) *CheckpointFilter {
	f.CreatedAfter = &after
	return f
}

// WithCreatedBefore adds created before filter
func (f *CheckpointFilter) WithCreatedBefore(before time.Time) *CheckpointFilter {
	f.CreatedBefore = &before
	return f
}

// WithSizeRange adds size range filter
func (f *CheckpointFilter) WithSizeRange(minSize, maxSize int64) *CheckpointFilter {
	f.MinSize = &minSize
	f.MaxSize = &maxSize
	return f
}

// WithMinSize adds minimum size filter
func (f *CheckpointFilter) WithMinSize(minSize int64) *CheckpointFilter {
	f.MinSize = &minSize
	return f
}

// WithMaxSize adds maximum size filter
func (f *CheckpointFilter) WithMaxSize(maxSize int64) *CheckpointFilter {
	f.MaxSize = &maxSize
	return f
}

// WithValidatedOnly filters only validated checkpoints
func (f *CheckpointFilter) WithValidatedOnly() *CheckpointFilter {
	f.ValidatedOnly = true
	f.UnvalidatedOnly = false
	return f
}

// WithUnvalidatedOnly filters only unvalidated checkpoints
func (f *CheckpointFilter) WithUnvalidatedOnly() *CheckpointFilter {
	f.UnvalidatedOnly = true
	f.ValidatedOnly = false
	return f
}

// WithCreatedBy adds creator filter
func (f *CheckpointFilter) WithCreatedBy(creators ...string) *CheckpointFilter {
	f.CreatedBy = append(f.CreatedBy, creators...)
	return f
}

// WithTag adds tag filter
func (f *CheckpointFilter) WithTag(key, value string) *CheckpointFilter {
	if f.Tags == nil {
		f.Tags = make(map[string]string)
	}
	f.Tags[key] = value
	return f
}

// WithTags adds multiple tag filters
func (f *CheckpointFilter) WithTags(tags map[string]string) *CheckpointFilter {
	if f.Tags == nil {
		f.Tags = make(map[string]string)
	}
	for k, v := range tags {
		f.Tags[k] = v
	}
	return f
}

// WithLimit adds limit for pagination
func (f *CheckpointFilter) WithLimit(limit int) *CheckpointFilter {
	f.Limit = limit
	return f
}

// WithOffset adds offset for pagination
func (f *CheckpointFilter) WithOffset(offset int) *CheckpointFilter {
	f.Offset = offset
	return f
}

// WithPagination adds both limit and offset
func (f *CheckpointFilter) WithPagination(limit, offset int) *CheckpointFilter {
	f.Limit = limit
	f.Offset = offset
	return f
}

// Matches checks if a checkpoint matches the filter criteria
func (f *CheckpointFilter) Matches(checkpoint *Checkpoint) bool {
	// Status filter
	if len(f.Status) > 0 {
		found := false
		for _, status := range f.Status {
			if checkpoint.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Type filter
	if len(f.Type) > 0 {
		found := false
		for _, checkpointType := range f.Type {
			if checkpoint.Type == checkpointType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// LSN range filters
	if f.MinLSN != nil && checkpoint.LSN < *f.MinLSN {
		return false
	}
	if f.MaxLSN != nil && checkpoint.LSN > *f.MaxLSN {
		return false
	}

	// Time range filters
	if f.CreatedAfter != nil && checkpoint.Timestamp.Before(*f.CreatedAfter) {
		return false
	}
	if f.CreatedBefore != nil && checkpoint.Timestamp.After(*f.CreatedBefore) {
		return false
	}

	// Size filters
	if f.MinSize != nil && checkpoint.Size < *f.MinSize {
		return false
	}
	if f.MaxSize != nil && checkpoint.Size > *f.MaxSize {
		return false
	}

	// Validation filters
	if f.ValidatedOnly && checkpoint.ValidatedAt == nil {
		return false
	}
	if f.UnvalidatedOnly && checkpoint.ValidatedAt != nil {
		return false
	}

	// Creator filter
	if len(f.CreatedBy) > 0 {
		found := false
		for _, creator := range f.CreatedBy {
			if checkpoint.CreatedBy == creator {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Tag filters
	if len(f.Tags) > 0 {
		for key, value := range f.Tags {
			if checkpointValue, exists := checkpoint.Metadata.Tags[key]; !exists || checkpointValue != value {
				return false
			}
		}
	}

	return true
}

// CompletedCheckpoints returns a filter for completed checkpoints only
func CompletedCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithStatus(CheckpointStatusCompleted)
}

// FullCheckpoints returns a filter for full checkpoints only
func FullCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithType(CheckpointTypeFull)
}

// IncrementalCheckpoints returns a filter for incremental checkpoints only
func IncrementalCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithType(CheckpointTypeIncremental)
}

// ValidatedCheckpoints returns a filter for validated checkpoints only
func ValidatedCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithValidatedOnly()
}

// RecentCheckpoints returns a filter for checkpoints created within the specified duration
func RecentCheckpoints(duration time.Duration) *CheckpointFilter {
	since := time.Now().Add(-duration)
	return NewCheckpointFilter().WithCreatedAfter(since)
}

// CheckpointsInLSNRange returns a filter for checkpoints within the specified LSN range
func CheckpointsInLSNRange(minLSN, maxLSN uint64) *CheckpointFilter {
	return NewCheckpointFilter().WithLSNRange(minLSN, maxLSN)
}

// LargeCheckpoints returns a filter for checkpoints larger than the specified size
func LargeCheckpoints(minSize int64) *CheckpointFilter {
	return NewCheckpointFilter().WithMinSize(minSize)
}

// SmallCheckpoints returns a filter for checkpoints smaller than the specified size
func SmallCheckpoints(maxSize int64) *CheckpointFilter {
	return NewCheckpointFilter().WithMaxSize(maxSize)
}

// CheckpointsByCreator returns a filter for checkpoints created by specific creators
func CheckpointsByCreator(creators ...string) *CheckpointFilter {
	return NewCheckpointFilter().WithCreatedBy(creators...)
}

// CheckpointsWithTag returns a filter for checkpoints with a specific tag
func CheckpointsWithTag(key, value string) *CheckpointFilter {
	return NewCheckpointFilter().WithTag(key, value)
}

// FailedCheckpoints returns a filter for failed checkpoints only
func FailedCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithStatus(CheckpointStatusFailed)
}

// CorruptedCheckpoints returns a filter for corrupted checkpoints only
func CorruptedCheckpoints() *CheckpointFilter {
	return NewCheckpointFilter().WithStatus(CheckpointStatusCorrupted)
}

// CheckpointsCreatedToday returns a filter for checkpoints created today
func CheckpointsCreatedToday() *CheckpointFilter {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)
	return NewCheckpointFilter().WithTimeRange(startOfDay, endOfDay)
}

// CheckpointsCreatedThisWeek returns a filter for checkpoints created this week
func CheckpointsCreatedThisWeek() *CheckpointFilter {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	startOfWeek := now.AddDate(0, 0, -weekday+1)
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())
	endOfWeek := startOfWeek.Add(7 * 24 * time.Hour)
	return NewCheckpointFilter().WithTimeRange(startOfWeek, endOfWeek)
}

// OldCheckpoints returns a filter for checkpoints older than the specified duration
func OldCheckpoints(age time.Duration) *CheckpointFilter {
	cutoff := time.Now().Add(-age)
	return NewCheckpointFilter().WithCreatedBefore(cutoff)
}
