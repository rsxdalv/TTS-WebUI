//go:build !linux && !windows

package procs

import "syscall"

// setParentDeathSignal has no equivalent here; the launcher's orphan sweep
// stops what a killed launcher left behind.
func setParentDeathSignal(*syscall.SysProcAttr) {}
