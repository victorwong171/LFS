//go:build windows

package services

import (
	"log"
	"syscall"
)

const (
	esSystemRequired uint32 = 0x00000001
	esContinuous     uint32 = 0x80000000
)

type platformKeepAwake struct{}

func newPlatformKeepAwake() *platformKeepAwake {
	return &platformKeepAwake{}
}

func (k *platformKeepAwake) start() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setThreadExecutionState := kernel32.NewProc("SetThreadExecutionState")
	
	// Prevent system sleep, but allow display sleep
	r, _, err := setThreadExecutionState.Call(uintptr(esSystemRequired | esContinuous))
	if r == 0 {
		log.Printf("Failed to set thread execution state: %v", err)
	}
}

func (k *platformKeepAwake) stop() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setThreadExecutionState := kernel32.NewProc("SetThreadExecutionState")
	
	// Restore default power savings behavior
	r, _, err := setThreadExecutionState.Call(uintptr(esContinuous))
	if r == 0 {
		log.Printf("Failed to restore thread execution state: %v", err)
	}
}
