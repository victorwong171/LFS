package storage

import (
	"context"
	"os/exec"
)

// commandExecutor 定义了执行命令的函数签名，方便单元测试进行 Mock
type commandExecutor func(ctx context.Context, name string, arg ...string) *exec.Cmd

// LocalHostController 实现了 interfaces.HostController 接口
type LocalHostController struct {
	execCommand commandExecutor
}

// NewLocalHostController 创建并返回本地宿主机控制器实例
func NewLocalHostController() *LocalHostController {
	return &LocalHostController{
		execCommand: exec.CommandContext,
	}
}
