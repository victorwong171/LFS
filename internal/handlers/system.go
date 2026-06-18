package handlers

import (
	"net/http"

	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

// SystemHandlers 负责宿主机系统控制相关的 HTTP 路由分发
type SystemHandlers struct {
	sysService interfaces.SystemControlService
}

// NewSystemHandlers 创建并返回系统路由控制器实例
func NewSystemHandlers(sysService interfaces.SystemControlService) *SystemHandlers {
	return &SystemHandlers{
		sysService: sysService,
	}
}

// Register 注册系统控制相关路由
func (h *SystemHandlers) Register(r *gin.Engine) {
	r.POST("/system/shutdown", h.TriggerShutdown)
	r.POST("/system/shutdown/cancel", h.CancelShutdown)
	r.POST("/system/display", h.SetDisplayPower)
	r.GET("/system/status", h.GetSystemStatus)
}

type shutdownRequest struct {
	Mode  string `json:"mode" binding:"required"`
	Delay int    `json:"delay"`
}

// TriggerShutdown 处理关机请求
func (h *SystemHandlers) TriggerShutdown(c *gin.Context) {
	var req shutdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := h.sysService.TriggerShutdown(ctx, req.Mode, req.Delay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shutdown triggered successfully"})
}

// CancelShutdown 处理取消关机请求
func (h *SystemHandlers) CancelShutdown(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.sysService.CancelShutdown(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Shutdown cancelled successfully"})
}

type displayRequest struct {
	State string `json:"state" binding:"required"`
}

// SetDisplayPower 处理开关屏幕请求
func (h *SystemHandlers) SetDisplayPower(c *gin.Context) {
	var req displayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	var powerOn bool
	if req.State == "on" {
		powerOn = true
	} else if req.State == "off" {
		powerOn = false
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid display state, must be 'on' or 'off'"})
		return
	}

	ctx := c.Request.Context()
	if err := h.sysService.SetDisplayPower(ctx, powerOn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Display power state set successfully"})
}

// GetSystemStatus 处理系统状态获取请求
func (h *SystemHandlers) GetSystemStatus(c *gin.Context) {
	status := h.sysService.GetSystemStatus()
	c.JSON(http.StatusOK, status)
}
