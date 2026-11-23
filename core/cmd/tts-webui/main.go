// Command tts-webui sets up and runs TTS WebUI. It serves a local setup page,
// runs the install steps, then starts and stops the app.
//
//	tts-webui [launch] [--root DIR] [--port N] [--no-browser] [--share] [--docker]
//	          [--start] [--system-python] [--no-react]
//	tts-webui --version
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Printf("tts-webui %s\n", version.String())
		return
	}
	if len(args) > 0 && args[0] == "launch" { // accepted for compatibility
		args = args[1:]
	}

	// Ctrl+C, a closed terminal (SIGHUP) and SIGTERM all stop the app before
	// the launcher exits. SIGKILL cannot be caught; the next launcher sweeps.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if err := runLaunch(ctx, args); err != nil {
		fmt.Fprintf(os.Stderr, "tts-webui: %v\n", err)
		os.Exit(1)
	}
}
