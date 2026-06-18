//go:build darwin

package services

import (
	"log"
	"os/exec"
)

type platformKeepAwake struct {
	cmd *exec.Cmd
}

func newPlatformKeepAwake() *platformKeepAwake {
	return &platformKeepAwake{}
}

func (k *platformKeepAwake) start() {
	if k.cmd != nil {
		return
	}
	// caffeinate -s: Prevent system sleep (display sleep is still allowed)
	k.cmd = exec.Command("caffeinate", "-s")
	if err := k.cmd.Start(); err != nil {
		log.Printf("Failed to start caffeinate: %v", err)
		k.cmd = nil
	}
}

func (k *platformKeepAwake) stop() {
	if k.cmd == nil {
		return
	}
	if k.cmd.Process != nil {
		_ = k.cmd.Process.Kill()
	}
	_ = k.cmd.Wait()
	k.cmd = nil
}
