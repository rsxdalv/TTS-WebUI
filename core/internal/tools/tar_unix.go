//go:build !windows

package tools

import "os/exec"

func tarCommand(src, destDir string) *exec.Cmd {
	return exec.Command("tar", "-xJf", src, "-C", destDir)
}
