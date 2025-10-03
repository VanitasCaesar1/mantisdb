package compression

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
	"time"
)

// ColdDataDetector manages cold data detection using bloom filters and access patterns
type ColdDataDetector struct {
	bloomFilter   *BloomFilter
	accessTracker *AccessTracker
	policies      []ColdDataPolicy
	config        *ColdDataConfig
	mutex         sync.RWMutex
}

// ColdDataConfig configures cold data detection behavior
type ColdDataConfig struct {
	Enabled              bool          `json:"enabled"`
	ColdThreshold        time.Duration `json:"cold_threshold"`
	BloomFilterSize      uint          `json:"bloom_filter_size"`
	BloomFilterHashCount uint          `json:"bloom_filter_hash_count"`
	AccessTrackingWindow time.Duration `json:"access_tracking_window"`
	MinAccessCount       int64         `json:"min_access_count"`
	SizeThreshold        int64         `json:"size_threshold"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
}

// ColdDataPolicy defines rules for identifying cold data
type ColdDataPolicy interface {
	IsCold(key string, metadata *DataMetadata, tracker *AccessTracker) bool
	Name() string
	Priority() int
}

// BloomFilter implements a space-efficient probabilistic data structure
type BloomFilter struct {
	bitArray  []bool
	size      uint
	hashCount uint
	mutex     sync.RWMutex
}

// ColdDataCandidate represents a candidate for compression
type ColdDataCandidate struct {
	Key          string    `json:"key"`
	LastAccessed time.Time `json:"last_accessed"`
	AccessCount  int64     `json:"access_count"`
	Size         int64     `json:"size"`
	ColdScore    float64   `json:"cold_score"`
	Policies     []string  `json:"policies"`
}

// NewColdDataDetector creates a new cold data detector
func NewColdDataDetector(config *ColdDataConfig) *ColdDataDetector {
	if config == nil {
		config = &ColdDataConfig{
			Enabled:              true,
			ColdThreshold:        24 * time.Hour,
			BloomFilterSize:      1000000, // 1M bits
			BloomFilterHashCount: 3,
			AccessTrackingWindow: 7 * 24 * time.Hour, // 1 week
			MinAccessCount:       5,
			SizeThreshold:        1024, // 1KB
			CleanupInterval:      time.Hour,
		}
	}

	detector := &ColdDataDetector{
		bloomFilter:   NewBloomFilter(config.BloomFilterSize, config.BloomFilterHashCount),
		accessTracker: NewAccessTracker(),
		policies:      make([]ColdDataPolicy, 0),
		config:        config,
	}

	// Add default policies
	detector.AddPolicy(&TimeBasedColdPolicy{Threshold: config.ColdThreshold})
	detector.AddPolicy(&AccessCountColdPolicy{MinCount: config.MinAccessCount})
	detector.AddPolicy(&SizeBasedColdPolicy{MinSize: config.SizeThreshold})

	// Start cleanup routine
	go detector.startCleanupRoutine()

	return detector
}

// NewBloomFilter creates a new bloom filter
func NewBloomFilter(size, hashCount uint) *BloomFilter {
	return &BloomFilter{
		bitArray:  make([]bool, size),
		size:      size,
		hashCount: hashCount,
	}
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(item string) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	hashes := bf.hash(item)
	for _, hash := range hashes {
		bf.bitArray[hash%bf.size] = true
	}
}

// Contains checks if an item might be in the bloom filter
func (bf *BloomFilter) Contains(item string) bool {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	hashes := bf.hash(item)
	for _, hash := range hashes {
		if !bf.bitArray[hash%bf.size] {
			return false
		}
	}
	return true
}

// hash generates multiple hash values for an item
func (bf *BloomFilter) hash(item string) []uint {
	hashes := make([]uint, bf.hashCount)

	// Use SHA-256 as base hash
	h := sha256.Sum256([]byte(item))

	// Generate multiple hashes using double hashing
	hash1 := binary.BigEndian.Uint32(h[:4])
	hash2 := binary.BigEndian.Uint32(h[4:8])

	for i := uint(0); i < bf.hashCount; i++ {
		hashes[i] = uint(hash1 + uint32(i)*hash2)
	}

	return hashes
}

// Clear resets the bloom filter
func (bf *BloomFilter) Clear() {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	for i := range bf.bitArray {
		bf.bitArray[i] = false
	}
}

// EstimateFalsePositiveRate estimates the false positive rate
func (bf *BloomFilter) EstimateFalsePositiveRate(itemCount uint) float64 {
	if itemCount == 0 {
		return 0
	}

	// Formula: (1 - e^(-k*n/m))^k
	// where k = hash count, n = item count, m = bit array size
	k := float64(bf.hashCount)
	n := float64(itemCount)
	m := float64(bf.size)

	return math.Pow(1-math.Exp(-k*n/m), k)
}

// AddPolicy adds a cold data policy
func (cdd *ColdDataDetector) AddPolicy(policy ColdDataPolicy) {
	cdd.mutex.Lock()
	defer cdd.mutex.Unlock()
	cdd.policies = append(cdd.policies, policy)
}

// RecordAccess records access to data
func (cdd *ColdDataDetector) RecordAccess(key string, size int64) {
	if !cdd.config.Enabled {
		return
	}

	// Record in access tracker
	cdd.accessTracker.RecordAccess(key)

	// Add to bloom filter for recent access tracking
	cdd.bloomFilter.Add(key)
}

// IsCold determines if data is cold based on configured policies
func (cdd *ColdDataDetector) IsCold(key string, metadata *DataMetadata) bool {
	if !cdd.config.Enabled {
		return false
	}

	cdd.mutex.RLock()
	defer cdd.mutex.RUnlock()

	// Check if recently accessed (bloom filter check)
	if cdd.bloomFilter.Contains(key) {
		// Might be recently accessed, check access tracker for confirmation
		lastAccess := cdd.accessTracker.GetLastAccess(key)
		if !lastAccess.IsZero() && time.Since(lastAccess) < cdd.config.ColdThreshold/2 {
			return false // Definitely not cold
		}
	}

	// Apply policies to determine if data is cold
	coldCount := 0
	for _, policy := range cdd.policies {
		if policy.IsCold(key, metadata, cdd.accessTracker) {
			coldCount++
		}
	}

	// Data is cold if majority of policies agree
	return coldCount > len(cdd.policies)/2
}

// GetColdDataCandidates returns candidates for compression
func (cdd *ColdDataDetector) GetColdDataCandidates(limit int) []ColdDataCandidate {
	if !cdd.config.Enabled {
		return nil
	}

	candidates := make([]ColdDataCandidate, 0)

	// Get cold keys from access tracker
	coldKeys := cdd.accessTracker.GetColdKeys(cdd.config.ColdThreshold)

	for _, key := range coldKeys {
		if len(candidates) >= limit {
			break
		}

		lastAccess := cdd.accessTracker.GetLastAccess(key)
		accessCount := cdd.accessTracker.GetAccessCount(key)

		metadata := &DataMetadata{
			LastAccessed: lastAccess,
			AccessCount:  accessCount,
		}

		if cdd.IsCold(key, metadata) {
			// Calculate cold score (higher = colder)
			coldScore := cdd.calculateColdScore(key, metadata)

			// Determine which policies flagged this as cold
			applicablePolicies := make([]string, 0)
			cdd.mutex.RLock()
			for _, policy := range cdd.policies {
				if policy.IsCold(key, metadata, cdd.accessTracker) {
					applicablePolicies = append(applicablePolicies, policy.Name())
				}
			}
			cdd.mutex.RUnlock()

			candidate := ColdDataCandidate{
				Key:          key,
				LastAccessed: lastAccess,
				AccessCount:  accessCount,
				Size:         metadata.Size,
				ColdScore:    coldScore,
				Policies:     applicablePolicies,
			}

			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

// calculateColdScore calculates a score indicating how "cold" the data is
func (cdd *ColdDataDetector) calculateColdScore(key string, metadata *DataMetadata) float64 {
	score := 0.0

	// Time since last access (normalized to 0-1)
	if !metadata.LastAccessed.IsZero() {
		timeSinceAccess := time.Since(metadata.LastAccessed)
		timeScore := math.Min(1.0, timeSinceAccess.Hours()/cdd.config.ColdThreshold.Hours())
		score += timeScore * 0.5 // 50% weight
	} else {
		score += 0.5 // Never accessed
	}

	// Access frequency (inverse relationship)
	if metadata.AccessCount > 0 {
		// Lower access count = higher cold score
		accessScore := 1.0 / (1.0 + float64(metadata.AccessCount)/float64(cdd.config.MinAccessCount))
		score += accessScore * 0.3 // 30% weight
	} else {
		score += 0.3 // Never accessed
	}

	// Size factor (larger files get slight preference for compression)
	if metadata.Size > 0 {
		sizeScore := math.Min(0.2, float64(metadata.Size)/float64(cdd.config.SizeThreshold*10))
		score += sizeScore // Up to 20% weight
	}

	return math.Min(1.0, score)
}

// startCleanupRoutine starts the background cleanup routine
func (cdd *ColdDataDetector) startCleanupRoutine() {
	ticker := time.NewTicker(cdd.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		cdd.cleanup()
	}
}

// cleanup removes old access records and resets bloom filter periodically
func (cdd *ColdDataDetector) cleanup() {
	// Clean up old access records
	cdd.accessTracker.Cleanup(cdd.config.AccessTrackingWindow)

	// Reset bloom filter periodically to prevent it from becoming too full
	// This is done every cleanup cycle to maintain accuracy
	cdd.bloomFilter.Clear()
}

// GetConfig returns the current configuration
func (cdd *ColdDataDetector) GetConfig() *ColdDataConfig {
	cdd.mutex.RLock()
	defer cdd.mutex.RUnlock()

	// Return a copy
	config := *cdd.config
	return &config
}

// UpdateConfig updates the detector configuration
func (cdd *ColdDataDetector) UpdateConfig(config *ColdDataConfig) {
	cdd.mutex.Lock()
	defer cdd.mutex.Unlock()
	cdd.config = config
}

// TimeBasedColdPolicy identifies cold data based on last access time
type TimeBasedColdPolicy struct {
	Threshold time.Duration
}

func (p *TimeBasedColdPolicy) Name() string  { return "time_based" }
func (p *TimeBasedColdPolicy) Priority() int { return 1 }

func (p *TimeBasedColdPolicy) IsCold(key string, metadata *DataMetadata, tracker *AccessTracker) bool {
	lastAccess := tracker.GetLastAccess(key)
	if lastAccess.IsZero() {
		return true // Never accessed
	}
	return time.Since(lastAccess) > p.Threshold
}

// AccessCountColdPolicy identifies cold data based on access frequency
type AccessCountColdPolicy struct {
	MinCount int64
}

func (p *AccessCountColdPolicy) Name() string  { return "access_count" }
func (p *AccessCountColdPolicy) Priority() int { return 2 }

func (p *AccessCountColdPolicy) IsCold(key string, metadata *DataMetadata, tracker *AccessTracker) bool {
	accessCount := tracker.GetAccessCount(key)
	return accessCount < p.MinCount
}

// SizeBasedColdPolicy identifies cold data based on size (larger files are better candidates)
type SizeBasedColdPolicy struct {
	MinSize int64
}

func (p *SizeBasedColdPolicy) Name() string  { return "size_based" }
func (p *SizeBasedColdPolicy) Priority() int { return 3 }

func (p *SizeBasedColdPolicy) IsCold(key string, metadata *DataMetadata, tracker *AccessTracker) bool {
	if metadata == nil {
		return false
	}
	return metadata.Size >= p.MinSize
}
