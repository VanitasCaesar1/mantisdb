// Package utils provides common utility functions
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// StringUtils provides string manipulation utilities
type StringUtils struct{}

// IsEmpty checks if a string is empty or contains only whitespace
func (StringUtils) IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Truncate truncates a string to the specified length
func (StringUtils) Truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// SplitPath splits a path string into components
func (StringUtils) SplitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	parts := []string{}
	current := ""

	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// JoinPath joins path components with forward slashes
func (StringUtils) JoinPath(parts ...string) string {
	return strings.Join(parts, "/")
}

// GenerateID generates a random ID string
func (StringUtils) GenerateID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	id := hex.EncodeToString(bytes)

	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, id)
	}
	return id
}

// TimeUtils provides time-related utilities
type TimeUtils struct{}

// Now returns the current time
func (TimeUtils) Now() time.Time {
	return time.Now()
}

// Unix returns the current Unix timestamp
func (TimeUtils) Unix() int64 {
	return time.Now().Unix()
}

// UnixMilli returns the current Unix timestamp in milliseconds
func (TimeUtils) UnixMilli() int64 {
	return time.Now().UnixMilli()
}

// ParseDuration parses a duration string with support for common units
func (TimeUtils) ParseDuration(s string) (time.Duration, error) {
	// Handle common cases that time.ParseDuration doesn't support
	s = strings.ToLower(strings.TrimSpace(s))

	// Convert common units
	replacements := map[string]string{
		"day":   "24h",
		"days":  "24h",
		"d":     "24h",
		"week":  "168h", // 7 * 24h
		"weeks": "168h",
		"w":     "168h",
	}

	for old, new := range replacements {
		s = strings.ReplaceAll(s, old, new)
	}

	return time.ParseDuration(s)
}

// FormatDuration formats a duration in a human-readable format
func (TimeUtils) FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.2fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.2fh", d.Hours())
	}
	return fmt.Sprintf("%.2fd", d.Hours()/24)
}

// ConversionUtils provides type conversion utilities
type ConversionUtils struct{}

// ToString converts various types to string
func (ConversionUtils) ToString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return strconv.FormatBool(val)
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ToInt converts various types to int
func (ConversionUtils) ToInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int8:
		return int(val), nil
	case int16:
		return int(val), nil
	case int32:
		return int(val), nil
	case int64:
		return int(val), nil
	case uint:
		return int(val), nil
	case uint8:
		return int(val), nil
	case uint16:
		return int(val), nil
	case uint32:
		return int(val), nil
	case uint64:
		return int(val), nil
	case float32:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// ToBool converts various types to bool
func (ConversionUtils) ToBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(val)
	case int, int8, int16, int32, int64:
		return val != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		return val != 0, nil
	case float32, float64:
		return val != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

// ValidationUtils provides validation utilities
type ValidationUtils struct{}

// IsValidKey checks if a key is valid for storage
func (ValidationUtils) IsValidKey(key string) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}

	// Check length limit
	const maxKeyLength = 250
	if len(key) > maxKeyLength {
		return false
	}

	// Check for invalid characters (optional - depends on requirements)
	// For now, allow all printable characters
	return true
}

// IsValidValue checks if a value is valid for storage
func (ValidationUtils) IsValidValue(value interface{}) bool {
	if value == nil {
		return false
	}

	// Check size limit for string values
	if str, ok := value.(string); ok {
		const maxValueSize = 1024 * 1024 // 1MB
		return len(str) <= maxValueSize
	}

	return true
}

// ValidateEmail validates an email address (basic validation)
func (ValidationUtils) ValidateEmail(email string) bool {
	if email == "" {
		return false
	}

	// Basic email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return false
	}

	// Check for at least one dot in domain
	return strings.Contains(domain, ".")
}

// MapUtils provides map manipulation utilities
type MapUtils struct{}

// DeepCopy creates a deep copy of a map
func (MapUtils) DeepCopy(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})

	for key, value := range original {
		copy[key] = deepCopyValue(value)
	}

	return copy
}

// deepCopyValue creates a deep copy of a value
func deepCopyValue(original interface{}) interface{} {
	if original == nil {
		return nil
	}

	originalValue := reflect.ValueOf(original)

	switch originalValue.Kind() {
	case reflect.Map:
		copy := make(map[string]interface{})
		for _, key := range originalValue.MapKeys() {
			copy[key.String()] = deepCopyValue(originalValue.MapIndex(key).Interface())
		}
		return copy
	case reflect.Slice:
		copy := make([]interface{}, originalValue.Len())
		for i := 0; i < originalValue.Len(); i++ {
			copy[i] = deepCopyValue(originalValue.Index(i).Interface())
		}
		return copy
	default:
		return original
	}
}

// Merge merges two maps, with the second map taking precedence
func (MapUtils) Merge(map1, map2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy from first map
	for key, value := range map1 {
		result[key] = value
	}

	// Override with second map
	for key, value := range map2 {
		result[key] = value
	}

	return result
}

// SliceUtils provides slice manipulation utilities
type SliceUtils struct{}

// Contains checks if a slice contains a specific string
func (SliceUtils) Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Unique removes duplicate strings from a slice
func (SliceUtils) Unique(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// Filter filters a slice based on a predicate function
func (SliceUtils) Filter(slice []string, predicate func(string) bool) []string {
	result := []string{}

	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}

	return result
}

// Global utility instances
var (
	Strings     = StringUtils{}
	Time        = TimeUtils{}
	Conversions = ConversionUtils{}
	Validation  = ValidationUtils{}
	Maps        = MapUtils{}
	Slices      = SliceUtils{}
)
