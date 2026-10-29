//go:build windows

package tools

import "os/exec"

// tarCommand uses the bsdtar bundled in Windows 10+ (via System32).
func tarCommand(src, destDir string) *exec.Cmd {
	return exec.Command("tar", "-xJf", src, "-C", destDir)
}
