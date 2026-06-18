package services

import (
	"context"
	"mime/multipart"
	"sync"
	"testing"

	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

type mockStorage struct {
	interfaces.Storage
}

func (m *mockStorage) SaveFile(ctx context.Context, file *multipart.FileHeader, rangeHeader string) error {
	return nil
}

func (m *mockStorage) SaveFileChunk(ctx context.Context, chunkInfo interfaces.FileChunkInfo, file *multipart.FileHeader) error {
	return nil
}

func (m *mockStorage) DownloadFile(ctx context.Context, c *gin.Context, filename, rangeHeader string) error {
	return nil
}

func (m *mockStorage) DownloadFileChunk(ctx context.Context, c *gin.Context, filename string, chunkIndex, chunkSize int64) error {
	return nil
}

type mockCalculator struct {
	interfaces.MD5Calculator
}

type mockActivityListener struct {
	mu         sync.Mutex
	startCalls int
	endCalls   int
}

func (m *mockActivityListener) OnTransferStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls++
}

func (m *mockActivityListener) OnTransferEnd() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endCalls++
}

func TestFileServiceActivityNotification(t *testing.T) {
	storage := &mockStorage{}
	calc := &mockCalculator{}
	service := NewFileService(storage, calc, "")

	listener := &mockActivityListener{}
	service.AddTransferActivityListener(listener)

	ctx := context.Background()

	// 1. 测试 UploadFile
	_ = service.UploadFile(ctx, nil, "")

	// 2. 测试 UploadFileChunk
	_ = service.UploadFileChunk(ctx, interfaces.FileChunkInfo{}, nil)

	// 3. 测试 BatchUpload (单个文件)
	files := []*multipart.FileHeader{{}}
	_, _, _ = service.BatchUpload(ctx, files)

	// 4. 测试 DownloadFile
	_ = service.DownloadFile(ctx, nil, "", "")

	// 5. 测试 DownloadFileChunk
	_ = service.DownloadFileChunk(ctx, nil, "", 0, 0)

	listener.mu.Lock()
	defer listener.mu.Unlock()

	// 各自应当各自通知一次
	if listener.startCalls != 5 || listener.endCalls != 5 {
		t.Errorf("Expected 5 start/end calls, got %d start and %d end", listener.startCalls, listener.endCalls)
	}
}

func TestFileServiceBatchUploadMultipleFiles(t *testing.T) {
	storage := &mockStorage{}
	calc := &mockCalculator{}
	service := NewFileService(storage, calc, "")

	listener := &mockActivityListener{}
	service.AddTransferActivityListener(listener)

	ctx := context.Background()

	// 测试 BatchUpload (多个文件以触发并发通道路径)
	files := []*multipart.FileHeader{{}, {}}
	_, _, _ = service.BatchUpload(ctx, files)

	listener.mu.Lock()
	defer listener.mu.Unlock()

	// 批量上传作为一个整体，只应当通知一次 Start 和一次 End
	if listener.startCalls != 1 || listener.endCalls != 1 {
		t.Errorf("Expected 1 start/end call for batch upload, got %d start and %d end", listener.startCalls, listener.endCalls)
	}
}
