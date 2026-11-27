//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

func applyPlatformAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
