package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// errRestart is returned by runLaunch when the page asked for a restart.
var errRestart = errors.New("restart requested")

func runLaunch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tts-webui", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root directory (default: parent of binary)")
	port := fs.Int("port", 7700, "port to listen on")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser automatically")
	share := fs.Bool("share", false, "ask Gradio for a public share link (Gradio share)")
	docker := fs.Bool("docker", false, "listen on 0.0.0.0:7770 for a container (router on 0.0.0.0:7770)")
	start := fs.Bool("start", false, "start the app as soon as the launcher is up, without opening the launcher page (Docker, Colab, start scripts)")
	systemPython := fs.Bool("system-python", false, "run python and npm from PATH, not the installer's micromamba env (Docker, Colab)")
	noReact := fs.Bool("no-react", false, "leave the React UI out (no build, no front on port 3000)")
	noAutostart := fs.Bool("no-autostart", false, "do not start the React UI when the launcher starts (for development; start it from the launcher page)")
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

	opts := []launcher.Option{launcher.WithLauncherPort(*port)}
	if *noBrowser {
		opts = append(opts, launcher.WithNoBrowser())
	}
	if *share {
		opts = append(opts, launcher.WithShare())
	}
	if *docker {
		opts = append(opts, launcher.WithDocker())
	}
	if *noReact {
		opts = append(opts, launcher.WithNoReact())
	}
	if *systemPython {
		opts = append(opts, launcher.WithSystemPython())
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

	// With --start the app is the point: it opens its own tab (config.json
	// inbrowser), so the launcher page stays closed.
	if !*noBrowser && !*start {
		if err := launcher.OpenBrowser(launchURL); err != nil {
			slog.Warn("could not open browser", "err", err)
		}
	}

	if *start {
		go func() {
			if err := l.StartApp(ctx, os.Stdout); err != nil {
				slog.Error("starting the app failed", "err", err)
			}
		}()
	}

	// The frontend starts with the launcher; Gradio waits for its button. --start
	// has already started both, and --no-react has no frontend.
	if !*start && !*noReact && !*noAutostart {
		go func() {
			if err := l.StartFrontend(ctx); err != nil {
				slog.Error("starting the frontend failed (use Launch frontend on the launcher page to see why)", "err", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-l.RestartRequested():
		slog.Info("launcher restarting: stopping the app")
		l.Shutdown()
		return errRestart
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
