//go:build !windows && !darwin

package storage

import (
	"context"
	"fmt"
)

// Shutdown 对不支持操作系统的占位实现
func (c *LocalHostController) Shutdown(ctx context.Context, delaySeconds int) error {
	return fmt.Errorf("unsupported operating system for shutdown")
}

// CancelShutdown 对不支持操作系统的占位实现
func (c *LocalHostController) CancelShutdown(ctx context.Context) error {
	return nil
}

// SetDisplayPower 对不支持操作系统的占位实现
func (c *LocalHostController) SetDisplayPower(ctx context.Context, powerOn bool) error {
	return fmt.Errorf("unsupported operating system for display control")
}
