//go:build linux

package procs

import "syscall"

// setParentDeathSignal asks Linux to send SIGTERM to the child when the
// launcher dies, even by SIGKILL. It reaches only the direct child (micromamba
// run, which passes it on); the launcher's orphan sweep covers the rest.
func setParentDeathSignal(attr *syscall.SysProcAttr) { attr.Pdeathsig = syscall.SIGTERM }
