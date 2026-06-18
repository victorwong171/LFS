package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"lfs/internal/interfaces"
)

// SystemControlServiceImpl 实现了 interfaces.SystemControlService 和 interfaces.TransferActivityListener
type SystemControlServiceImpl struct {
	controller interfaces.HostController
	keepAwake  *platformKeepAwake

	activeTransfers    int64 // 原子计数，当前活跃的传输数量
	shutdownOnComplete int32 // 原子布尔值，是否在传输完成后关机 (0-否, 1-是)

	mutex        sync.Mutex
	delayTimer   *time.Timer
	shutdownMode string    // "immediate" | "scheduled" | "on_complete" | ""
	shutdownTime time.Time // 计划中的关机绝对时间
	displayOn    bool      // 显示器状态
}

// NewSystemControlService 创建并返回系统控制服务实例
func NewSystemControlService(controller interfaces.HostController) *SystemControlServiceImpl {
	return &SystemControlServiceImpl{
		controller: controller,
		keepAwake:  newPlatformKeepAwake(),
		displayOn:  true, // 默认显示器处于开启状态
	}
}

// TriggerShutdown 触发关机任务
func (s *SystemControlServiceImpl) TriggerShutdown(ctx context.Context, mode string, delaySeconds int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 触发前先清理原有的关机任务
	s.cancelTimerLocked()

	switch mode {
	case "immediate":
		s.shutdownMode = "immediate"
		s.shutdownTime = time.Now()
		return s.controller.Shutdown(ctx, 0)

	case "scheduled":
		if delaySeconds <= 0 {
			return errors.New("delaySeconds must be greater than 0 for scheduled mode")
		}
		s.shutdownMode = "scheduled"
		s.shutdownTime = time.Now().Add(time.Duration(delaySeconds) * time.Second)

		// 启动阻止系统休眠，确保倒计时期间系统处于激活状态
		s.keepAwake.start()

		s.delayTimer = time.AfterFunc(time.Duration(delaySeconds)*time.Second, func() {
			s.mutex.Lock()
			// 在定时器触发时清除状态并执行关机
			s.shutdownMode = ""
			s.shutdownTime = time.Time{}
			s.mutex.Unlock()

			// 执行物理关机
			_ = s.controller.Shutdown(context.Background(), 0)
		})
		return nil

	case "on_complete":
		s.shutdownMode = "on_complete"
		s.shutdownTime = time.Time{}
		atomic.StoreInt32(&s.shutdownOnComplete, 1)

		// 启动阻止休眠锁，直到传输完成并关机
		s.keepAwake.start()

		// 如果当前恰好没有传输任务，立即触发关机
		if atomic.LoadInt64(&s.activeTransfers) <= 0 {
			// 在另一个协程中触发，避免持锁时间过长
			go func() {
				_ = s.controller.Shutdown(context.Background(), 0)
			}()
		}
		return nil

	default:
		return errors.New("invalid shutdown mode")
	}
}

// CancelShutdown 取消关机计划
func (s *SystemControlServiceImpl) CancelShutdown(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cancelTimerLocked()
	s.shutdownMode = ""
	s.shutdownTime = time.Time{}
	atomic.StoreInt32(&s.shutdownOnComplete, 0)

	// 如果没有活跃传输，释放防止休眠锁
	if atomic.LoadInt64(&s.activeTransfers) <= 0 {
		s.keepAwake.stop()
	}

	// 同时也通知底层控制器尝试撤销可能存在的 OS 关机计划
	return s.controller.CancelShutdown(ctx)
}

// SetDisplayPower 控制显示器开启/关闭
func (s *SystemControlServiceImpl) SetDisplayPower(ctx context.Context, powerOn bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.controller.SetDisplayPower(ctx, powerOn); err != nil {
		return err
	}
	s.displayOn = powerOn
	return nil
}

// GetSystemStatus 获取系统运行状态
func (s *SystemControlServiceImpl) GetSystemStatus() map[string]interface{} {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var shutdownTimeStr string
	if !s.shutdownTime.IsZero() {
		shutdownTimeStr = s.shutdownTime.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"active_transfers":     atomic.LoadInt64(&s.activeTransfers),
		"shutdown_on_complete": atomic.LoadInt32(&s.shutdownOnComplete) == 1,
		"shutdown_mode":        s.shutdownMode,
		"shutdown_time":        shutdownTimeStr,
		"display_on":           s.displayOn,
	}
}

// OnTransferStart 传输活动监听器：传输开始时触发
func (s *SystemControlServiceImpl) OnTransferStart() {
	newVal := atomic.AddInt64(&s.activeTransfers, 1)
	if newVal == 1 {
		// 从 0 变为 1，激活防休眠锁，保证屏幕关闭时程序不挂起
		s.keepAwake.start()
	}
}

// OnTransferEnd 传输活动监听器：传输结束时触发
func (s *SystemControlServiceImpl) OnTransferEnd() {
	newVal := atomic.AddInt64(&s.activeTransfers, -1)
	if newVal <= 0 {
		// 任务归零，检查是否配置了“传输完成后关机”
		if atomic.LoadInt32(&s.shutdownOnComplete) == 1 {
			_ = s.controller.Shutdown(context.Background(), 0)
			return
		}

		s.mutex.Lock()
		isScheduled := s.shutdownMode == "scheduled"
		s.mutex.Unlock()

		// 如果没有定时关机在排期，释放防休眠锁，允许系统进入正常省电状态
		if !isScheduled {
			s.keepAwake.stop()
		}
	}
}

// cancelTimerLocked 清除定时器 (调用时必须持有 s.mutex 锁)
func (s *SystemControlServiceImpl) cancelTimerLocked() {
	if s.delayTimer != nil {
		s.delayTimer.Stop()
		s.delayTimer = nil
	}
}
