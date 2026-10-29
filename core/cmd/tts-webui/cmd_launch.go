package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

func init() {
	register(command{
		name:    "launch",
		summary: "start the Go launcher server and open the browser",
		run:     runLaunch,
	})
}

func runLaunch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root directory (default: parent of binary)")
	port := fs.Int("port", 7700, "port to listen on (default: D-open-1)")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser automatically")
	uiDev := fs.String("ui-dev", "", "proxy non-/api requests to this Vite dev server URL")
	envMode := fs.String("env", "conda", "Python environment to use: conda (default) or uv")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *envMode != "conda" && *envMode != "uv" {
		return fmt.Errorf("--env must be 'conda' or 'uv', got %q", *envMode)
	}

	var root paths.Root
	var err error
	if *rootDir != "" {
		root, err = paths.New(*rootDir)
		if err != nil {
			return fmt.Errorf("invalid root %q: %w", *rootDir, err)
		}
	} else {
		root, err = paths.FromBinary()
		if err != nil {
			return fmt.Errorf("detecting install root: %w", err)
		}
	}

	srv, err := server.NewWithOptions(*port, server.Options{UIDev: *uiDev})
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	l := launcher.NewWithOptions(root, launcher.LauncherOptions{EnvMode: *envMode})
	l.Register(srv)

	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr(), err)
	}
	defer ln.Close()

	url := "http://" + srv.Addr()
	slog.Info("launcher started", "url", url, "root", root.Dir())
	fmt.Println(url)

	if !*noBrowser {
		if err := launcher.OpenBrowser(url); err != nil {
			slog.Warn("could not open browser", "err", err)
		}
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}
