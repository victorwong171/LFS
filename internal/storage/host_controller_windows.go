//go:build windows

package storage

import (
	"context"
	"fmt"
	"strconv"
)

// Shutdown 执行 Windows 系统关机操作
func (c *LocalHostController) Shutdown(ctx context.Context, delaySeconds int) error {
	// Windows: 普通用户即可调用 shutdown.exe 关机
	cmd := c.execCommand(ctx, "shutdown.exe", "/s", "/t", strconv.Itoa(delaySeconds))
	if err := cmd.Run(); err != nil {
		// 如果因为环境限制需要提权，则通过 PowerShell UAC 尝试
		psCmd := fmt.Sprintf("Start-Process shutdown.exe -ArgumentList '/s /t %d' -Verb RunAs", delaySeconds)
		uacCmd := c.execCommand(ctx, "powershell.exe", "-Command", psCmd)
		return uacCmd.Run()
	}
	return nil
}

// CancelShutdown 取消 Windows 已计划的定时关机
func (c *LocalHostController) CancelShutdown(ctx context.Context) error {
	cmd := c.execCommand(ctx, "shutdown.exe", "/a")
	if err := cmd.Run(); err != nil {
		// 尝试提权取消
		psCmd := "Start-Process shutdown.exe -ArgumentList '/a' -Verb RunAs"
		uacCmd := c.execCommand(ctx, "powershell.exe", "-Command", psCmd)
		return uacCmd.Run()
	}
	return nil
}

// SetDisplayPower 控制 Windows 显示器开启/关闭
func (c *LocalHostController) SetDisplayPower(ctx context.Context, powerOn bool) error {
	if powerOn {
		// Windows 唤醒屏幕：模拟鼠标微小位移以唤醒屏幕
		psCmd := `[void][System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(([System.Windows.Forms.Cursor]::Position.X + 1), [System.Windows.Forms.Cursor]::Position.Y); [void][System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(([System.Windows.Forms.Cursor]::Position.X - 1), [System.Windows.Forms.Cursor]::Position.Y)`
		cmd := c.execCommand(ctx, "powershell.exe", "-Command", psCmd)
		return cmd.Run()
	} else {
		// Windows 息屏：使用 SendMessage 发送 SC_MONITORPOWER=2 消息
		psDef := `[DllImport("user32.dll")] public static extern int SendMessage(int hWnd, int hMsg, int wParam, int lParam);`
		psCmd := fmt.Sprintf("Add-Type -MemberDefinition '%s' -Name 'Win32SendMessage' -Namespace 'Win32' -PassThru; [Win32.Win32SendMessage]::SendMessage(0xffff, 0x0112, 0xf170, 2)", psDef)
		cmd := c.execCommand(ctx, "powershell.exe", "-Command", psCmd)
		return cmd.Run()
	}
}
