package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type mockHostController struct {
	mu            sync.Mutex
	shutdownCalls int
	lastDelay     int
	cancelCalls   int
	displayCalls  int
	displayState  bool
}

func (m *mockHostController) Shutdown(ctx context.Context, delaySeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownCalls++
	m.lastDelay = delaySeconds
	return nil
}

func (m *mockHostController) CancelShutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelCalls++
	return nil
}

func (m *mockHostController) SetDisplayPower(ctx context.Context, powerOn bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.displayCalls++
	m.displayState = powerOn
	return nil
}

func TestSystemControlService(t *testing.T) {
	mockCtrl := &mockHostController{}
	service := NewSystemControlService(mockCtrl)

	// 1. 测试初始状态
	status := service.GetSystemStatus()
	if status["active_transfers"].(int64) != 0 {
		t.Errorf("Expected 0 active transfers, got %v", status["active_transfers"])
	}
	if status["shutdown_mode"].(string) != "" {
		t.Errorf("Expected empty shutdown mode, got %v", status["shutdown_mode"])
	}
	if status["display_on"].(bool) != true {
		t.Errorf("Expected display_on to be true")
	}

	// 2. 测试即时关机
	ctx := context.Background()
	err := service.TriggerShutdown(ctx, "immediate", 0)
	if err != nil {
		t.Errorf("TriggerShutdown failed: %v", err)
	}
	mockCtrl.mu.Lock()
	if mockCtrl.shutdownCalls != 1 || mockCtrl.lastDelay != 0 {
		t.Errorf("Expected 1 shutdown call with delay 0, got %v calls with delay %d", mockCtrl.shutdownCalls, mockCtrl.lastDelay)
	}
	mockCtrl.mu.Unlock()

	// 3. 测试定时关机
	err = service.TriggerShutdown(ctx, "scheduled", 1) // 1秒延迟
	if err != nil {
		t.Errorf("TriggerShutdown failed: %v", err)
	}
	status = service.GetSystemStatus()
	if status["shutdown_mode"].(string) != "scheduled" {
		t.Errorf("Expected shutdown mode 'scheduled', got %v", status["shutdown_mode"])
	}
	if status["shutdown_time"].(string) == "" {
		t.Errorf("Expected non-empty shutdown_time")
	}

	// 等待定时器触发
	time.Sleep(1200 * time.Millisecond)
	mockCtrl.mu.Lock()
	if mockCtrl.shutdownCalls != 2 {
		t.Errorf("Expected 2 shutdown calls after scheduled timer fired, got %v", mockCtrl.shutdownCalls)
	}
	mockCtrl.mu.Unlock()

	// 4. 测试取消关机
	err = service.TriggerShutdown(ctx, "scheduled", 10) // 10秒延迟
	if err != nil {
		t.Errorf("TriggerShutdown failed: %v", err)
	}
	err = service.CancelShutdown(ctx)
	if err != nil {
		t.Errorf("CancelShutdown failed: %v", err)
	}
	status = service.GetSystemStatus()
	if status["shutdown_mode"].(string) != "" {
		t.Errorf("Expected empty shutdown mode after cancel, got %v", status["shutdown_mode"])
	}
	mockCtrl.mu.Lock()
	if mockCtrl.cancelCalls != 1 {
		t.Errorf("Expected 1 cancel call, got %v", mockCtrl.cancelCalls)
	}
	mockCtrl.mu.Unlock()

	// 5. 测试显示器电源控制
	err = service.SetDisplayPower(ctx, false)
	if err != nil {
		t.Errorf("SetDisplayPower failed: %v", err)
	}
	status = service.GetSystemStatus()
	if status["display_on"].(bool) != false {
		t.Errorf("Expected display_on to be false")
	}
	mockCtrl.mu.Lock()
	if mockCtrl.displayCalls != 1 || mockCtrl.displayState != false {
		t.Errorf("Expected 1 display call with state false")
	}
	mockCtrl.mu.Unlock()
}

func TestTransferActivityAndOnCompleteShutdown(t *testing.T) {
	mockCtrl := &mockHostController{}
	service := NewSystemControlService(mockCtrl)

	// 1. 模拟传输任务并发启动与结束
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.OnTransferStart()
		}()
	}
	wg.Wait()

	status := service.GetSystemStatus()
	if status["active_transfers"].(int64) != 5 {
		t.Errorf("Expected 5 active transfers, got %v", status["active_transfers"])
	}

	// 2. 触发“传输完成后关机”模式
	ctx := context.Background()
	err := service.TriggerShutdown(ctx, "on_complete", 0)
	if err != nil {
		t.Errorf("TriggerShutdown on_complete failed: %v", err)
	}

	mockCtrl.mu.Lock()
	if mockCtrl.shutdownCalls != 0 {
		t.Errorf("Should not shut down while transfers are active")
	}
	mockCtrl.mu.Unlock()

	// 3. 逐渐结束传输任务，直到归零触发关机
	for i := 0; i < 4; i++ {
		service.OnTransferEnd()
	}
	mockCtrl.mu.Lock()
	if mockCtrl.shutdownCalls != 0 {
		t.Errorf("Should not shut down when there is still 1 active transfer")
	}
	mockCtrl.mu.Unlock()

	// 结束最后一个任务
	service.OnTransferEnd()
	
	// 给一点协程切换的时间
	time.Sleep(50 * time.Millisecond)

	mockCtrl.mu.Lock()
	if mockCtrl.shutdownCalls != 1 {
		t.Errorf("Expected 1 shutdown call when active transfers reached 0, got %d", mockCtrl.shutdownCalls)
	}
	mockCtrl.mu.Unlock()
}

func TestTriggerShutdownInvalidInputs(t *testing.T) {
	mockCtrl := &mockHostController{}
	service := NewSystemControlService(mockCtrl)
	ctx := context.Background()

	// 测试非法延迟时间
	err := service.TriggerShutdown(ctx, "scheduled", 0)
	if err == nil {
		t.Errorf("Expected error for scheduled mode with 0 delay")
	}

	// 测试非法模式
	err = service.TriggerShutdown(ctx, "invalid_mode", 10)
	if err == nil {
		t.Errorf("Expected error for invalid mode")
	}
}

type errorHostController struct {
	mockHostController
}

func (e *errorHostController) SetDisplayPower(ctx context.Context, powerOn bool) error {
	return errors.New("display hardware error")
}

func TestSetDisplayPowerError(t *testing.T) {
	errCtrl := &errorHostController{}
	service := NewSystemControlService(errCtrl)
	ctx := context.Background()

	err := service.SetDisplayPower(ctx, false)
	if err == nil {
		t.Errorf("Expected error when display controller fails")
	}
}

func TestTransferEndWithScheduledShutdown(t *testing.T) {
	mockCtrl := &mockHostController{}
	service := NewSystemControlService(mockCtrl)
	ctx := context.Background()

	// 1. 设置定时关机
	err := service.TriggerShutdown(ctx, "scheduled", 60)
	if err != nil {
		t.Fatalf("TriggerShutdown failed: %v", err)
	}

	// 2. 开始并结束一个传输任务
	service.OnTransferStart()
	service.OnTransferEnd()

	// 验证定时关机仍然是 scheduled 模式，并没有被取消或出错
	status := service.GetSystemStatus()
	if status["shutdown_mode"].(string) != "scheduled" {
		t.Errorf("Expected shutdown mode to remain 'scheduled' after transfer end, got %v", status["shutdown_mode"])
	}
}

