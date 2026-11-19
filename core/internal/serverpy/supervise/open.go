package supervise

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens url in the system default browser. It is the default
// Config.Opener; no shell string interpolation is involved.
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// The empty "" is the window title; without it a URL containing & is
		// split by cmd.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	go func() { _ = cmd.Wait() }() // detach; the browser outlives us
	return nil
}
