//go:build !windows && !darwin

package services

type platformKeepAwake struct{}

func newPlatformKeepAwake() *platformKeepAwake {
	return &platformKeepAwake{}
}

func (k *platformKeepAwake) start() {}
func (k *platformKeepAwake) stop()  {}
