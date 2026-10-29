//go:build !windows

package native

import "os/exec"

func tarXZCommand(src, destDir string) *exec.Cmd {
	return exec.Command("tar", "-xJf", src, "-C", destDir)
}
