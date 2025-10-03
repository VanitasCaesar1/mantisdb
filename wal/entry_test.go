package wal

import (
	"testing"
	"time"
)

func TestWALEntrySerialization(t *testing.T) {
	// Create a test WAL entry
	entry := &WALEntry{
		LSN:   12345,
		TxnID: 67890,
		Operation: Operation{
			Type:     OpInsert,
			Key:      "test_key",
			Value:    []byte("test_value"),
			OldValue: []byte("old_value"),
		},
		Timestamp: time.Unix(1609459200, 0), // 2021-01-01 00:00:00 UTC
	}

	// Serialize the entry
	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize WAL entry: %v", err)
	}

	// Deserialize the entry
	deserializedEntry, err := DeserializeWALEntry(data)
	if err != nil {
		t.Fatalf("Failed to deserialize WAL entry: %v", err)
	}

	// Verify all fields match
	if deserializedEntry.LSN != entry.LSN {
		t.Errorf("LSN mismatch: expected %d, got %d", entry.LSN, deserializedEntry.LSN)
	}

	if deserializedEntry.TxnID != entry.TxnID {
		t.Errorf("TxnID mismatch: expected %d, got %d", entry.TxnID, deserializedEntry.TxnID)
	}

	if deserializedEntry.Operation.Type != entry.Operation.Type {
		t.Errorf("Operation type mismatch: expected %d, got %d", entry.Operation.Type, deserializedEntry.Operation.Type)
	}

	if deserializedEntry.Operation.Key != entry.Operation.Key {
		t.Errorf("Key mismatch: expected %s, got %s", entry.Operation.Key, deserializedEntry.Operation.Key)
	}

	if string(deserializedEntry.Operation.Value) != string(entry.Operation.Value) {
		t.Errorf("Value mismatch: expected %s, got %s", string(entry.Operation.Value), string(deserializedEntry.Operation.Value))
	}

	if string(deserializedEntry.Operation.OldValue) != string(entry.Operation.OldValue) {
		t.Errorf("OldValue mismatch: expected %s, got %s", string(entry.Operation.OldValue), string(deserializedEntry.Operation.OldValue))
	}

	if deserializedEntry.Timestamp.Unix() != entry.Timestamp.Unix() {
		t.Errorf("Timestamp mismatch: expected %d, got %d", entry.Timestamp.Unix(), deserializedEntry.Timestamp.Unix())
	}
}

func TestWALEntryChecksumVerification(t *testing.T) {
	entry := &WALEntry{
		LSN:   1,
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "key",
			Value: []byte("value"),
		},
		Timestamp: time.Now(),
	}

	// Serialize the entry
	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize WAL entry: %v", err)
	}

	// Corrupt the data by changing a byte
	corruptedData := make([]byte, len(data))
	copy(corruptedData, data)
	corruptedData[50] ^= 0xFF // Flip bits in payload

	// Try to deserialize corrupted data
	_, err = DeserializeWALEntry(corruptedData)
	if err != ErrChecksumMismatch {
		t.Errorf("Expected checksum mismatch error, got: %v", err)
	}
}

func TestOperationTypes(t *testing.T) {
	operations := []OperationType{OpInsert, OpUpdate, OpDelete, OpCommit, OpAbort}

	for _, opType := range operations {
		entry := &WALEntry{
			LSN:   1,
			TxnID: 1,
			Operation: Operation{
				Type:  opType,
				Key:   "test",
				Value: []byte("test"),
			},
			Timestamp: time.Now(),
		}

		data, err := entry.Serialize()
		if err != nil {
			t.Fatalf("Failed to serialize entry with operation type %d: %v", opType, err)
		}

		deserializedEntry, err := DeserializeWALEntry(data)
		if err != nil {
			t.Fatalf("Failed to deserialize entry with operation type %d: %v", opType, err)
		}

		if deserializedEntry.Operation.Type != opType {
			t.Errorf("Operation type mismatch: expected %d, got %d", opType, deserializedEntry.Operation.Type)
		}
	}
}

func TestEmptyValues(t *testing.T) {
	entry := &WALEntry{
		LSN:   1,
		TxnID: 1,
		Operation: Operation{
			Type:     OpDelete,
			Key:      "key_to_delete",
			Value:    nil,
			OldValue: []byte("old_value"),
		},
		Timestamp: time.Now(),
	}

	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize entry with empty value: %v", err)
	}

	deserializedEntry, err := DeserializeWALEntry(data)
	if err != nil {
		t.Fatalf("Failed to deserialize entry with empty value: %v", err)
	}

	if len(deserializedEntry.Operation.Value) != 0 {
		t.Errorf("Expected empty value, got: %v", deserializedEntry.Operation.Value)
	}

	if string(deserializedEntry.Operation.OldValue) != "old_value" {
		t.Errorf("OldValue mismatch: expected 'old_value', got %s", string(deserializedEntry.Operation.OldValue))
	}
}

func TestChecksumCalculation(t *testing.T) {
	data := []byte("test data for checksum")
	checksum1 := CalculateChecksum(data)
	checksum2 := CalculateChecksum(data)

	if checksum1 != checksum2 {
		t.Errorf("Checksum calculation is not deterministic: %d != %d", checksum1, checksum2)
	}

	if !VerifyChecksum(data, checksum1) {
		t.Errorf("Checksum verification failed for valid data")
	}

	if VerifyChecksum(data, checksum1+1) {
		t.Errorf("Checksum verification should fail for invalid checksum")
	}
}
func TestLargeWALEntry(t *testing.T) {
	// Test with large data to ensure serialization handles size correctly
	largeValue := make([]byte, 10000)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	entry := &WALEntry{
		LSN:   999999,
		TxnID: 888888,
		Operation: Operation{
			Type:     OpUpdate,
			Key:      "large_key_with_many_characters_to_test_serialization",
			Value:    largeValue,
			OldValue: []byte("previous_value"),
		},
		Timestamp: time.Now(),
	}

	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize large WAL entry: %v", err)
	}

	deserializedEntry, err := DeserializeWALEntry(data)
	if err != nil {
		t.Fatalf("Failed to deserialize large WAL entry: %v", err)
	}

	if len(deserializedEntry.Operation.Value) != len(largeValue) {
		t.Errorf("Large value length mismatch: expected %d, got %d",
			len(largeValue), len(deserializedEntry.Operation.Value))
	}

	// Verify the large value content
	for i, b := range deserializedEntry.Operation.Value {
		if b != largeValue[i] {
			t.Errorf("Large value content mismatch at index %d: expected %d, got %d", i, largeValue[i], b)
			break
		}
	}
}

func TestWALEntryBoundaryConditions(t *testing.T) {
	// Test with maximum values
	entry := &WALEntry{
		LSN:   ^uint64(0), // Maximum uint64
		TxnID: ^uint64(0), // Maximum uint64
		Operation: Operation{
			Type:     OpCommit,
			Key:      "",
			Value:    nil,
			OldValue: nil,
		},
		Timestamp: time.Unix(0, 0), // Minimum time
	}

	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize boundary condition WAL entry: %v", err)
	}

	deserializedEntry, err := DeserializeWALEntry(data)
	if err != nil {
		t.Fatalf("Failed to deserialize boundary condition WAL entry: %v", err)
	}

	if deserializedEntry.LSN != ^uint64(0) {
		t.Errorf("LSN boundary condition failed: expected %d, got %d", ^uint64(0), deserializedEntry.LSN)
	}

	if deserializedEntry.TxnID != ^uint64(0) {
		t.Errorf("TxnID boundary condition failed: expected %d, got %d", ^uint64(0), deserializedEntry.TxnID)
	}
}

func TestInvalidWALEntryDeserialization(t *testing.T) {
	// Test with insufficient data
	shortData := []byte{1, 2, 3, 4, 5}
	_, err := DeserializeWALEntry(shortData)
	if err != ErrInvalidWALEntry {
		t.Errorf("Expected ErrInvalidWALEntry for short data, got: %v", err)
	}

	// Test with truncated payload
	entry := &WALEntry{
		LSN:   1,
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "test",
			Value: []byte("test"),
		},
		Timestamp: time.Now(),
	}

	data, err := entry.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize entry: %v", err)
	}

	// Truncate the data
	truncatedData := data[:len(data)-5]
	_, err = DeserializeWALEntry(truncatedData)
	if err != ErrInvalidWALEntry {
		t.Errorf("Expected ErrInvalidWALEntry for truncated data, got: %v", err)
	}
}
