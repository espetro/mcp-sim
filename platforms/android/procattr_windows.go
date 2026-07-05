//go:build windows

package android

import (
	"os/exec"
	"syscall"
)

// setProcAttr configures cmd to survive parent death by placing it in a new process group.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
