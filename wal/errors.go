package wal

import "errors"

// WAL-specific errors
var (
	ErrInvalidWALEntry   = errors.New("invalid WAL entry format")
	ErrChecksumMismatch  = errors.New("WAL entry checksum mismatch")
	ErrWALFileFull       = errors.New("WAL file is full")
	ErrWALFileCorrupted  = errors.New("WAL file is corrupted")
	ErrWALFileNotFound   = errors.New("WAL file not found")
	ErrWALWriteFailed    = errors.New("failed to write to WAL")
	ErrWALReadFailed     = errors.New("failed to read from WAL")
	ErrWALRotationFailed = errors.New("WAL file rotation failed")
	ErrWALCleanupFailed  = errors.New("WAL cleanup failed")
	ErrInvalidLSN        = errors.New("invalid log sequence number")
)
