package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

func runLaunch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tts-webui", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root directory (default: parent of binary)")
	port := fs.Int("port", 7700, "port to listen on")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser automatically")
	share := fs.Bool("share", false, "ask Gradio for a public share link (server.py --share)")
	docker := fs.Bool("docker", false, "listen on 0.0.0.0:7770 for a container (server.py --docker)")
	if err := fs.Parse(args); err != nil {
		return err
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

	srv, err := server.New(*port)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	var opts []launcher.Option
	if *noBrowser {
		opts = append(opts, launcher.WithNoBrowser())
	}
	if *share {
		opts = append(opts, launcher.WithShare())
	}
	if *docker {
		opts = append(opts, launcher.WithDocker())
	}
	l := launcher.New(root, opts...)
	l.CleanupOrphans()
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
		slog.Info("launcher exiting: stopping the app")
		done := make(chan struct{})
		go func() {
			l.Shutdown()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			slog.Warn("the app did not stop within 20 s; exiting anyway")
		}
		return nil
	case err := <-errCh:
		l.Shutdown()
		return err
	}
}
