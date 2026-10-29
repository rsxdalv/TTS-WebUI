//go:build windows

package procs

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platformState holds the Windows Job Object handle for a supervised process.
type platformState struct {
	job windows.Handle
}

// newPlatformState sets Windows-specific SysProcAttr and returns a zero state.
// The Job Object is assigned after the process starts (see assign).
func newPlatformState(cmd *exec.Cmd) (*platformState, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	return &platformState{}, nil
}

// assign creates a Job Object, sets kill-on-close, and assigns the process.
// Must be called after cmd.Start().
func (ps *platformState) assign(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("CreateJobObject: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("SetInformationJobObject: %w", err)
	}

	hProc, err := windows.OpenProcess(
		windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid),
	)
	if err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("OpenProcess: %w", err)
	}
	defer windows.CloseHandle(hProc)

	if err := windows.AssignProcessToJobObject(job, hProc); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	ps.job = job
	return nil
}

// graceful sends CTRL_BREAK to the process group.
func (ps *platformState) graceful(pid int) error {
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid))
}

// force closes the Job Object handle, which triggers kill-on-close for all
// processes in the job (child and all descendants).
func (ps *platformState) force() error {
	if ps.job == 0 {
		return nil
	}
	err := windows.CloseHandle(ps.job)
	ps.job = 0
	return err
}

