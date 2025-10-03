package wal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"time"
)

// OperationType represents the type of operation in a WAL entry
type OperationType uint32

const (
	OpInsert OperationType = iota + 1
	OpUpdate
	OpDelete
	OpCommit
	OpAbort
)

// String returns the string representation of the operation type
func (ot OperationType) String() string {
	switch ot {
	case OpInsert:
		return "OpInsert"
	case OpUpdate:
		return "OpUpdate"
	case OpDelete:
		return "OpDelete"
	case OpCommit:
		return "OpCommit"
	case OpAbort:
		return "OpAbort"
	default:
		return fmt.Sprintf("Unknown(%d)", ot)
	}
}

// Operation represents a database operation
type Operation struct {
	Type     OperationType
	Key      string
	Value    []byte
	OldValue []byte // For rollback support
}

// WALEntry represents a single entry in the Write-Ahead Log
type WALEntry struct {
	LSN       uint64    // Log Sequence Number
	TxnID     uint64    // Transaction ID
	Operation Operation // The operation being logged
	Timestamp time.Time
	Checksum  uint32 // Entry integrity checksum
}

// WALEntryHeader represents the fixed-size header of a WAL entry
type WALEntryHeader struct {
	LSN        uint64 // 8 bytes
	TxnID      uint64 // 8 bytes
	OpType     uint32 // 4 bytes
	Timestamp  int64  // 8 bytes (Unix timestamp)
	PayloadLen uint32 // 4 bytes
	Checksum   uint32 // 4 bytes
}

const (
	WALEntryHeaderSize = 36 // Size of WALEntryHeader in bytes
)

// Serialize converts a WAL entry to binary format
func (entry *WALEntry) Serialize() ([]byte, error) {
	var buf bytes.Buffer

	// Serialize operation payload first to calculate length
	payload, err := entry.serializePayload()
	if err != nil {
		return nil, err
	}

	// Create header
	header := WALEntryHeader{
		LSN:        entry.LSN,
		TxnID:      entry.TxnID,
		OpType:     uint32(entry.Operation.Type),
		Timestamp:  entry.Timestamp.Unix(),
		PayloadLen: uint32(len(payload)),
		Checksum:   0, // Will be calculated after serialization
	}

	// Write header
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return nil, err
	}

	// Write payload
	buf.Write(payload)

	// Calculate and update checksum
	data := buf.Bytes()
	// Calculate checksum on all data except the checksum field itself (bytes 32-35 in header)
	checksumData := make([]byte, 0, len(data))
	checksumData = append(checksumData, data[:32]...) // Header before checksum
	checksumData = append(checksumData, data[36:]...) // Header after checksum + payload
	checksum := crc32.ChecksumIEEE(checksumData)
	binary.LittleEndian.PutUint32(data[32:36], checksum) // Update checksum in header

	return data, nil
}

// serializePayload serializes the operation payload
func (entry *WALEntry) serializePayload() ([]byte, error) {
	var buf bytes.Buffer

	// Write key length and key
	keyBytes := []byte(entry.Operation.Key)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(keyBytes))); err != nil {
		return nil, err
	}
	buf.Write(keyBytes)

	// Write value length and value
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(entry.Operation.Value))); err != nil {
		return nil, err
	}
	buf.Write(entry.Operation.Value)

	// Write old value length and old value (for rollback)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(entry.Operation.OldValue))); err != nil {
		return nil, err
	}
	buf.Write(entry.Operation.OldValue)

	return buf.Bytes(), nil
}

// Deserialize converts binary data back to a WAL entry
func DeserializeWALEntry(data []byte) (*WALEntry, error) {
	if len(data) < WALEntryHeaderSize {
		return nil, ErrInvalidWALEntry
	}

	// Read header
	var header WALEntryHeader
	buf := bytes.NewReader(data[:WALEntryHeaderSize])
	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	// Verify payload length
	expectedTotalLen := WALEntryHeaderSize + int(header.PayloadLen)
	if len(data) < expectedTotalLen {
		return nil, ErrInvalidWALEntry
	}

	// Verify checksum only on the expected data length
	expectedChecksum := header.Checksum
	// Calculate checksum on all data except the checksum field itself (bytes 32-35 in header)
	checksumData := make([]byte, 0, expectedTotalLen)
	checksumData = append(checksumData, data[:32]...)                 // Header before checksum
	checksumData = append(checksumData, data[36:expectedTotalLen]...) // Header after checksum + payload
	actualChecksum := crc32.ChecksumIEEE(checksumData)
	if expectedChecksum != actualChecksum {
		return nil, ErrChecksumMismatch
	}

	// Deserialize payload
	payload := data[WALEntryHeaderSize : WALEntryHeaderSize+header.PayloadLen]
	operation, err := deserializePayload(payload, OperationType(header.OpType))
	if err != nil {
		return nil, err
	}

	entry := &WALEntry{
		LSN:       header.LSN,
		TxnID:     header.TxnID,
		Operation: *operation,
		Timestamp: time.Unix(header.Timestamp, 0),
		Checksum:  header.Checksum,
	}

	return entry, nil
}

// deserializePayload deserializes the operation payload
func deserializePayload(data []byte, opType OperationType) (*Operation, error) {
	buf := bytes.NewReader(data)

	// Read key
	var keyLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
		return nil, err
	}
	keyBytes := make([]byte, keyLen)
	if keyLen > 0 {
		if _, err := buf.Read(keyBytes); err != nil {
			return nil, err
		}
	}

	// Read value
	var valueLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &valueLen); err != nil {
		return nil, err
	}
	value := make([]byte, valueLen)
	if valueLen > 0 {
		if _, err := buf.Read(value); err != nil {
			return nil, err
		}
	}

	// Read old value
	var oldValueLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &oldValueLen); err != nil {
		return nil, err
	}
	oldValue := make([]byte, oldValueLen)
	if oldValueLen > 0 {
		if _, err := buf.Read(oldValue); err != nil {
			return nil, err
		}
	}

	operation := &Operation{
		Type:     opType,
		Key:      string(keyBytes),
		Value:    value,
		OldValue: oldValue,
	}

	return operation, nil
}

// CalculateChecksum calculates CRC32 checksum for data integrity
func CalculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// VerifyChecksum verifies the integrity of data using CRC32 checksum
func VerifyChecksum(data []byte, expectedChecksum uint32) bool {
	actualChecksum := crc32.ChecksumIEEE(data)
	return actualChecksum == expectedChecksum
}
