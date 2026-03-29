//go:build windows

package executor

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in a new process group on Windows.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup terminates the process group on Windows.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// killPid sends a kill signal to a single process by PID.
func killPid(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	_ = p.Kill()
}
