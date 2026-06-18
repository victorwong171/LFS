package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockSystemControlService struct {
	shutdownMode  string
	shutdownDelay int
	shutdownErr   error

	cancelCalls int
	cancelErr   error

	displayPower bool
	displayErr   error

	status map[string]interface{}
}

func (m *mockSystemControlService) TriggerShutdown(ctx context.Context, mode string, delaySeconds int) error {
	m.shutdownMode = mode
	m.shutdownDelay = delaySeconds
	return m.shutdownErr
}

func (m *mockSystemControlService) CancelShutdown(ctx context.Context) error {
	m.cancelCalls++
	return m.cancelErr
}

func (m *mockSystemControlService) SetDisplayPower(ctx context.Context, powerOn bool) error {
	m.displayPower = powerOn
	return m.displayErr
}

func (m *mockSystemControlService) GetSystemStatus() map[string]interface{} {
	return m.status
}

func setupTestRouter(service *mockSystemControlService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handlers := NewSystemHandlers(service)
	handlers.Register(r)
	return r
}

func TestTriggerShutdownHandler(t *testing.T) {
	mockService := &mockSystemControlService{}
	router := setupTestRouter(mockService)

	// 1. 正常请求
	reqBody, _ := json.Marshal(map[string]interface{}{
		"mode":  "scheduled",
		"delay": 300,
	})
	req := httptest.NewRequest("POST", "/system/shutdown", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if mockService.shutdownMode != "scheduled" || mockService.shutdownDelay != 300 {
		t.Errorf("Mock service did not receive correct parameters")
	}

	// 2. 参数缺失错误 (JSON 绑定错误)
	reqBodyErr, _ := json.Marshal(map[string]interface{}{
		"delay": 300, // 缺少必填参数 mode
	})
	req = httptest.NewRequest("POST", "/system/shutdown", bytes.NewBuffer(reqBodyErr))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for bad request, got %d", w.Code)
	}

	// 3. 服务层执行失败错误
	mockService.shutdownErr = errors.New("platform error")
	reqBody, _ = json.Marshal(map[string]interface{}{
		"mode":  "immediate",
		"delay": 0,
	})
	req = httptest.NewRequest("POST", "/system/shutdown", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 on service error, got %d", w.Code)
	}
}

func TestCancelShutdownHandler(t *testing.T) {
	mockService := &mockSystemControlService{}
	router := setupTestRouter(mockService)

	// 1. 成功请求
	req := httptest.NewRequest("POST", "/system/shutdown/cancel", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if mockService.cancelCalls != 1 {
		t.Errorf("Expected 1 cancel call to service")
	}

	// 2. 失败请求
	mockService.cancelErr = errors.New("cancel failed")
	req = httptest.NewRequest("POST", "/system/shutdown/cancel", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestSetDisplayPowerHandler(t *testing.T) {
	mockService := &mockSystemControlService{}
	router := setupTestRouter(mockService)

	// 1. 开启屏幕
	reqBody, _ := json.Marshal(map[string]string{"state": "on"})
	req := httptest.NewRequest("POST", "/system/display", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if !mockService.displayPower {
		t.Errorf("Expected displayPower true")
	}

	// 2. 关闭屏幕
	reqBody, _ = json.Marshal(map[string]string{"state": "off"})
	req = httptest.NewRequest("POST", "/system/display", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if mockService.displayPower {
		t.Errorf("Expected displayPower false")
	}

	// 3. 非法参数类型
	reqBody, _ = json.Marshal(map[string]string{"state": "invalid"})
	req = httptest.NewRequest("POST", "/system/display", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid state, got %d", w.Code)
	}

	// 4. 传输错误处理 (Service Error)
	mockService.displayErr = errors.New("display hardware error")
	reqBody, _ = json.Marshal(map[string]string{"state": "off"})
	req = httptest.NewRequest("POST", "/system/display", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGetSystemStatusHandler(t *testing.T) {
	mockService := &mockSystemControlService{
		status: map[string]interface{}{
			"active_transfers": int64(3),
			"display_on":       true,
			"shutdown_mode":    "scheduled",
		},
	}
	router := setupTestRouter(mockService)

	req := httptest.NewRequest("GET", "/system/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["active_transfers"].(float64) != 3 || resp["display_on"].(bool) != true || resp["shutdown_mode"].(string) != "scheduled" {
		t.Errorf("Returned status map does not match expected values")
	}
}
