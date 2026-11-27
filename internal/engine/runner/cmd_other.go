//go:build !windows

package runner

import "os/exec"

func applyPlatformAttr(cmd *exec.Cmd) {}
