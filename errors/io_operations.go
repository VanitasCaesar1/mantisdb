package errors

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// IOOperationManager provides high-level I/O operations with error handling
type IOOperationManager struct {
	handler *IOErrorHandler
}

// NewIOOperationManager creates a new I/O operation manager
func NewIOOperationManager(handler *IOErrorHandler) *IOOperationManager {
	return &IOOperationManager{
		handler: handler,
	}
}

// FileOperation represents a file operation result
type FileOperation struct {
	Path         string        `json:"path"`
	Operation    string        `json:"operation"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	BytesRead    int64         `json:"bytes_read,omitempty"`
	BytesWritten int64         `json:"bytes_written,omitempty"`
	Error        error         `json:"error,omitempty"`
}

// ReadFile reads a file with retry logic
func (m *IOOperationManager) ReadFile(ctx context.Context, path string) ([]byte, *FileOperation) {
	op := &FileOperation{
		Path:      path,
		Operation: "read",
	}

	startTime := time.Now()

	result, retryResult := m.handler.ExecuteWithRetryAndResult(ctx, func() (interface{}, error) {
		data, err := os.ReadFile(path)
		return data, err
	}, fmt.Sprintf("file:%s", path))

	var data []byte
	if result != nil {
		data = result.([]byte)
	}

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	if retryResult.Success {
		op.BytesRead = int64(len(data))
	}

	return data, op
}

// WriteFile writes data to a file with retry logic
func (m *IOOperationManager) WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) *FileOperation {
	op := &FileOperation{
		Path:      path,
		Operation: "write",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		return os.WriteFile(path, data, perm)
	}, fmt.Sprintf("file:%s", path))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	if retryResult.Success {
		op.BytesWritten = int64(len(data))
	}

	return op
}

// AppendToFile appends data to a file with retry logic
func (m *IOOperationManager) AppendToFile(ctx context.Context, path string, data []byte) *FileOperation {
	op := &FileOperation{
		Path:      path,
		Operation: "append",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = file.Write(data)
		return err
	}, fmt.Sprintf("file:%s", path))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	if retryResult.Success {
		op.BytesWritten = int64(len(data))
	}

	return op
}

// SyncFile syncs a file to disk with retry logic
func (m *IOOperationManager) SyncFile(ctx context.Context, path string) *FileOperation {
	op := &FileOperation{
		Path:      path,
		Operation: "sync",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer file.Close()

		return file.Sync()
	}, fmt.Sprintf("file:%s", path))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return op
}

// CopyFile copies a file with retry logic
func (m *IOOperationManager) CopyFile(ctx context.Context, srcPath, dstPath string) *FileOperation {
	op := &FileOperation{
		Path:      fmt.Sprintf("%s -> %s", srcPath, dstPath),
		Operation: "copy",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		bytesWritten, err := io.Copy(dst, src)
		if err != nil {
			return err
		}

		op.BytesWritten = bytesWritten
		return dst.Sync()
	}, fmt.Sprintf("copy:%s:%s", srcPath, dstPath))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return op
}

// DeleteFile deletes a file with retry logic
func (m *IOOperationManager) DeleteFile(ctx context.Context, path string) *FileOperation {
	op := &FileOperation{
		Path:      path,
		Operation: "delete",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		return os.Remove(path)
	}, fmt.Sprintf("file:%s", path))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return op
}

// CreateDirectory creates a directory with retry logic
func (m *IOOperationManager) CreateDirectory(ctx context.Context, path string, perm os.FileMode) *FileOperation {
	op := &FileOperation{
		Path:      path,
		Operation: "mkdir",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		return os.MkdirAll(path, perm)
	}, fmt.Sprintf("dir:%s", path))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return op
}

// StatFile gets file information with retry logic
func (m *IOOperationManager) StatFile(ctx context.Context, path string) (os.FileInfo, *FileOperation) {
	op := &FileOperation{
		Path:      path,
		Operation: "stat",
	}

	startTime := time.Now()

	result, retryResult := m.handler.ExecuteWithRetryAndResult(ctx, func() (interface{}, error) {
		info, err := os.Stat(path)
		return info, err
	}, fmt.Sprintf("file:%s", path))

	var info os.FileInfo
	if result != nil {
		info = result.(os.FileInfo)
	}

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return info, op
}

// RenameFile renames a file with retry logic
func (m *IOOperationManager) RenameFile(ctx context.Context, oldPath, newPath string) *FileOperation {
	op := &FileOperation{
		Path:      fmt.Sprintf("%s -> %s", oldPath, newPath),
		Operation: "rename",
	}

	startTime := time.Now()

	retryResult := m.handler.ExecuteWithRetry(ctx, func() error {
		return os.Rename(oldPath, newPath)
	}, fmt.Sprintf("rename:%s:%s", oldPath, newPath))

	op.Duration = time.Since(startTime)
	op.Success = retryResult.Success
	op.Error = retryResult.LastError

	return op
}

// BatchFileOperations performs multiple file operations with individual retry logic
type BatchFileOperations struct {
	manager    *IOOperationManager
	operations []func(context.Context) *FileOperation
	results    []*FileOperation
}

// NewBatchFileOperations creates a new batch file operations manager
func (m *IOOperationManager) NewBatchFileOperations() *BatchFileOperations {
	return &BatchFileOperations{
		manager:    m,
		operations: make([]func(context.Context) *FileOperation, 0),
		results:    make([]*FileOperation, 0),
	}
}

// AddReadFile adds a read file operation to the batch
func (b *BatchFileOperations) AddReadFile(path string) {
	b.operations = append(b.operations, func(ctx context.Context) *FileOperation {
		_, op := b.manager.ReadFile(ctx, path)
		return op
	})
}

// AddWriteFile adds a write file operation to the batch
func (b *BatchFileOperations) AddWriteFile(path string, data []byte, perm os.FileMode) {
	b.operations = append(b.operations, func(ctx context.Context) *FileOperation {
		return b.manager.WriteFile(ctx, path, data, perm)
	})
}

// AddDeleteFile adds a delete file operation to the batch
func (b *BatchFileOperations) AddDeleteFile(path string) {
	b.operations = append(b.operations, func(ctx context.Context) *FileOperation {
		return b.manager.DeleteFile(ctx, path)
	})
}

// Execute executes all operations in the batch
func (b *BatchFileOperations) Execute(ctx context.Context) []*FileOperation {
	b.results = make([]*FileOperation, 0, len(b.operations))

	for _, operation := range b.operations {
		result := operation(ctx)
		b.results = append(b.results, result)

		// Check context cancellation
		if ctx.Err() != nil {
			break
		}
	}

	return b.results
}

// GetResults returns the results of the batch operations
func (b *BatchFileOperations) GetResults() []*FileOperation {
	return b.results
}

// GetSuccessCount returns the number of successful operations
func (b *BatchFileOperations) GetSuccessCount() int {
	count := 0
	for _, result := range b.results {
		if result.Success {
			count++
		}
	}
	return count
}

// GetFailureCount returns the number of failed operations
func (b *BatchFileOperations) GetFailureCount() int {
	count := 0
	for _, result := range b.results {
		if !result.Success {
			count++
		}
	}
	return count
}

// GetTotalDuration returns the total duration of all operations
func (b *BatchFileOperations) GetTotalDuration() time.Duration {
	var total time.Duration
	for _, result := range b.results {
		total += result.Duration
	}
	return total
}
