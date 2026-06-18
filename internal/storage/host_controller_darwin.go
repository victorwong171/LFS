//go:build darwin

package storage

import (
	"context"
	"fmt"
	"os/exec"
)

// Shutdown 执行 macOS 系统关机操作
func (c *LocalHostController) Shutdown(ctx context.Context, delaySeconds int) error {
	// 如果是即时关机且处于桌面 GUI 会话，使用 osascript 免密关机
	if delaySeconds == 0 {
		appleScript := `tell application "System Events" to shut down`
		guiCmd := c.execCommand(ctx, "osascript", "-e", appleScript)
		if err := guiCmd.Run(); err == nil {
			return nil
		}
	}

	// 降级使用 sudo 命令行关机
	var cmd *exec.Cmd
	if delaySeconds > 0 {
		minutes := (delaySeconds + 59) / 60
		cmd = c.execCommand(ctx, "sudo", "shutdown", "-h", fmt.Sprintf("+%d", minutes))
	} else {
		cmd = c.execCommand(ctx, "sudo", "shutdown", "-h", "now")
	}
	return cmd.Run()
}

// CancelShutdown 取消 macOS 已计划的定时关机
func (c *LocalHostController) CancelShutdown(ctx context.Context) error {
	cmd := c.execCommand(ctx, "sudo", "shutdown", "-c")
	return cmd.Run()
}

// SetDisplayPower 控制 macOS 显示器开启/关闭
func (c *LocalHostController) SetDisplayPower(ctx context.Context, powerOn bool) error {
	if powerOn {
		// macOS 唤醒屏幕：发送 1 秒的用户活跃断言
		cmd := c.execCommand(ctx, "caffeinate", "-u", "-t", "1")
		return cmd.Run()
	} else {
		// macOS 息屏：使用 pmset 命令睡眠屏幕，不影响进程运行
		cmd := c.execCommand(ctx, "pmset", "displaysleepnow")
		return cmd.Run()
	}
}
