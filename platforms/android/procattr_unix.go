//go:build !windows

package android

import (
	"os/exec"
	"syscall"
)

// setProcAttr configures cmd to survive parent death by starting it in its own process group.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
