//go:build !windows

package procs

import (
	"fmt"
	"os/exec"
	"syscall"
)

// platformState holds the process group ID for tree-kill on Unix.
type platformState struct {
	pgid int // 0 = not yet assigned
}

func newPlatformState(cmd *exec.Cmd) (*platformState, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &platformState{}, nil
}

// assign captures the process group ID after cmd.Start().
// With Setpgid=true the pgid equals the child PID.
func (ps *platformState) assign(cmd *exec.Cmd) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("getpgid: %w", err)
	}
	ps.pgid = pgid
	return nil
}

// graceful sends SIGINT to the entire process group.
func (ps *platformState) graceful(_ int) error {
	if ps.pgid == 0 {
		return nil
	}
	return syscall.Kill(-ps.pgid, syscall.SIGINT)
}

// force sends SIGKILL to the entire process group.
func (ps *platformState) force() error {
	if ps.pgid == 0 {
		return nil
	}
	return syscall.Kill(-ps.pgid, syscall.SIGKILL)
}
