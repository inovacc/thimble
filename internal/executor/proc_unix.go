//go:build !windows

package executor

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in a new process group on Unix.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the entire process group on Unix.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// killPid sends a kill signal to a single process by PID.
func killPid(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	_ = p.Signal(os.Kill)
}
