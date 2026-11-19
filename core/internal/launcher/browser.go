package launcher

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the system default browser.
// Platform-specific; no shell string interpolation.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// cmd /c start "" <url>  — the empty "" is the window title,
		// required so that a URL containing & is not split by cmd.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux and others
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	// Detach; we do not wait for the browser to close.
	go func() { _ = cmd.Wait() }()
	return nil
}
