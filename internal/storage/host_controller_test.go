package storage

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

type commandRun struct {
	name string
	args []string
}

func getSuccessCommand(ctx context.Context) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd.exe", "/c", "exit 0")
	}
	return exec.CommandContext(ctx, "true")
}

func getFailureCommand(ctx context.Context) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd.exe", "/c", "exit 1")
	}
	return exec.CommandContext(ctx, "false")
}

func TestLocalHostControllerSuccessPaths(t *testing.T) {
	var runs []commandRun

	mockExecutor := func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		runs = append(runs, commandRun{name: name, args: arg})
		return getSuccessCommand(ctx)
	}

	controller := NewLocalHostController()
	controller.execCommand = mockExecutor
	ctx := context.Background()

	// 1. 测试关机 (0秒延迟)
	err := controller.Shutdown(ctx, 0)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// 2. 测试关机 (延迟30秒)
	err = controller.Shutdown(ctx, 30)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// 3. 测试取消关机
	err = controller.CancelShutdown(ctx)
	if err != nil {
		t.Errorf("CancelShutdown failed: %v", err)
	}

	// 4. 测试开启屏幕
	err = controller.SetDisplayPower(ctx, true)
	if err != nil {
		t.Errorf("SetDisplayPower true failed: %v", err)
	}

	// 5. 测试关闭屏幕
	err = controller.SetDisplayPower(ctx, false)
	if err != nil {
		t.Errorf("SetDisplayPower false failed: %v", err)
	}

	// 验证捕获到的命令
	if len(runs) == 0 {
		t.Fatalf("Expected commands to be run, but got 0")
	}

	// 平台特异性命令结构匹配验证
	if runtime.GOOS == "windows" {
		shutdownChecked := false
		cancelChecked := false
		displayOnChecked := false
		displayOffChecked := false

		for _, r := range runs {
			if r.name == "shutdown.exe" {
				if len(r.args) > 0 && r.args[0] == "/s" {
					shutdownChecked = true
				}
				if len(r.args) > 0 && r.args[0] == "/a" {
					cancelChecked = true
				}
			}
			if r.name == "powershell.exe" {
				// 检查 powershell 参数包含唤醒或黑屏特征
				argStr := strings.Join(r.args, " ")
				if strings.Contains(argStr, "Cursor") {
					displayOnChecked = true
				}
				if strings.Contains(argStr, "SendMessage") {
					displayOffChecked = true
				}
			}
		}

		if !shutdownChecked || !cancelChecked || !displayOnChecked || !displayOffChecked {
			t.Errorf("Windows commands mapping verification failed")
		}
	}

	if runtime.GOOS == "darwin" {
		osascriptChecked := false
		sudoShutdownChecked := false
		pmsetSleepChecked := false
		caffeinateChecked := false

		for _, r := range runs {
			if r.name == "osascript" {
				osascriptChecked = true
			}
			if r.name == "sudo" {
				sudoShutdownChecked = true
			}
			if r.name == "pmset" {
				pmsetSleepChecked = true
			}
			if r.name == "caffeinate" {
				caffeinateChecked = true
			}
		}

		// 注意：如果 delaySeconds == 0 且 osascript 成功，不会调用 sudo。
		// 但我们进行了 delaySeconds = 30 的测试，因此肯定有一次 sudo shutdown
		if !osascriptChecked || !sudoShutdownChecked || !pmsetSleepChecked || !caffeinateChecked {
			t.Errorf("macOS commands mapping verification failed")
		}
	}
}

func TestLocalHostControllerFallbackPaths(t *testing.T) {
	// 测试当直接执行系统指令失败时，是否进入了提权后备方案（例如 Windows 尝试 UAC）
	var runs []commandRun

	mockExecutor := func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		runs = append(runs, commandRun{name: name, args: arg})
		// 只有首次的特定原生命令返回错误，以触发 fallback
		if name == "shutdown.exe" {
			return getFailureCommand(ctx)
		}
		return getSuccessCommand(ctx)
	}

	controller := NewLocalHostController()
	controller.execCommand = mockExecutor
	ctx := context.Background()

	// 1. 测试关机失败后触发提权
	_ = controller.Shutdown(ctx, 60)

	// 2. 测试取消关机失败后触发提权
	_ = controller.CancelShutdown(ctx)

	if runtime.GOOS == "windows" {
		hasPowerShellRunAs := false
		for _, r := range runs {
			if r.name == "powershell.exe" {
				argStr := strings.Join(r.args, " ")
				if strings.Contains(argStr, "RunAs") {
					hasPowerShellRunAs = true
				}
			}
		}
		if !hasPowerShellRunAs {
			t.Errorf("Expected Windows fallback to invoke powershell RunAs")
		}
	}
}

func TestUnsupportedOperatingSystem(t *testing.T) {
	// 临时修改 GOOS (如果可能)，但在 Go 中 runtime.GOOS 是常量。
	// 为支持不支持操作系统的测试分支，我们可以直接手动检查不受支持 OS 的函数返回值。
	// 这里通过 mock 传递不支持的操作系统名称我们很难干预常量，但为了覆盖 default 分支，
	// 如果在非 Darwin/Windows 平台运行，我们的方法会自然进入 default 分支，我们在这里做个常规检验。
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		controller := NewLocalHostController()
		ctx := context.Background()
		err := controller.Shutdown(ctx, 0)
		if err == nil {
			t.Errorf("Expected error for unsupported OS shutdown")
		}
		err = controller.SetDisplayPower(ctx, true)
		if err == nil {
			t.Errorf("Expected error for unsupported OS display control")
		}
	}
}
