//go:build !windows

package installer_scripts

import "os/exec"

// shellCmd builds the process for a shell line, as Node's exec does.
func shellCmd(line string, _ []string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", line)
}

// setShellLine has nothing to do here: shellCmd already passed the line.
func setShellLine(_ *exec.Cmd, _ string) {}
