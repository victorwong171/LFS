package interfaces

import "context"

// HostController 负责与宿主机操作系统的物理交互接口（适配器层）
type HostController interface {
	// Shutdown 触发宿主机物理关机。
	// delaySeconds: 延迟关机时间（秒）。0 表示立即关机。
	Shutdown(ctx context.Context, delaySeconds int) error

	// CancelShutdown 取消已经计划的延迟关机（主要是 Windows 平台）。
	CancelShutdown(ctx context.Context) error

	// SetDisplayPower 控制显示器输出睡眠或唤醒。
	// powerOn: true 代表唤醒显示器，false 代表让显示器黑屏休眠。
	SetDisplayPower(ctx context.Context, powerOn bool) error
}

// SystemControlService 业务逻辑接口，负责维护关机定时器、传输任务计数及防止睡眠逻辑（应用服务层）
type SystemControlService interface {
	// TriggerShutdown 启动关机任务。
	// mode: 关机模式，可选值为 "immediate"（即时关机）、"scheduled"（定时关机）、"on_complete"（传输完成后关机）。
	// delaySeconds: 定时关机的倒计时时间（秒）。
	TriggerShutdown(ctx context.Context, mode string, delaySeconds int) error

	// CancelShutdown 取消已排期的定时关机或“传输完成后关机”计划。
	CancelShutdown(ctx context.Context) error

	// SetDisplayPower 开启或关闭显示器。
	SetDisplayPower(ctx context.Context, powerOn bool) error

	// GetSystemStatus 返回系统当前状态（包含定时关机挂起状态、当前活跃传输数等）。
	GetSystemStatus() map[string]interface{}
}

// TransferActivityListener 同级功能组合解耦接口，传输活动监听器
type TransferActivityListener interface {
	// OnTransferStart 传输开始时的通知
	OnTransferStart()

	// OnTransferEnd 传输结束时的通知
	OnTransferEnd()
}
