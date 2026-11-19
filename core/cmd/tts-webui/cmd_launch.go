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
	port := fs.Int("port", 7700, "port to listen on")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser automatically")
	uiDev := fs.String("ui-dev", "", "proxy non-/api requests to this Vite dev server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var root paths.Root
	var err error
	if *rootDir != "" {
		root, err = paths.NewUninitialized(*rootDir)
		if err != nil {
			return fmt.Errorf("invalid root %q: %w", *rootDir, err)
		}
	} else {
		root, err = paths.FromBinaryUninitialized()
		if err != nil {
			return fmt.Errorf("detecting install root: %w", err)
		}
	}

	srv, err := server.NewWithOptions(*port, server.Options{UIDev: *uiDev})
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	l := launcher.New(root)
	l.Register(srv)

	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr(), err)
	}
	defer ln.Close()

	launchURL := "http://" + srv.Addr()
	slog.Info("launcher started", "url", launchURL, "root", root.Dir())
	fmt.Println(launchURL)

	if !*noBrowser {
		if err := launcher.OpenBrowser(launchURL); err != nil {
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
