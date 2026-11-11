//go:build windows

package native

import "os/exec"

// tarXZCommand uses the bsdtar bundled with Windows 10+ (System32/tar.exe).
func tarXZCommand(src, destDir string) *exec.Cmd {
	return exec.Command("tar", "-xJf", src, "-C", destDir)
}
