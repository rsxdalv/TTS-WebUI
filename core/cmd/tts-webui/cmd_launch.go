package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/legacy"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/lifecycle"
	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/router"
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
	proxyMode := fs.String("proxy", "python", "proxy mode: python (default) or core")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *envMode != "conda" && *envMode != "uv" {
		return fmt.Errorf("--env must be 'conda' or 'uv', got %q", *envMode)
	}
	if *proxyMode != "python" && *proxyMode != "core" {
		return fmt.Errorf("--proxy must be 'python' or 'core', got %q", *proxyMode)
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

	l := launcher.NewWithOptions(root, launcher.LauncherOptions{
		EnvMode:   *envMode,
		ProxyMode: *proxyMode,
	})
	l.Register(srv)

	// C5.5: in --proxy=core mode, start server.py as a managed engine and route
	// "/" to it via the router.
	if *proxyMode == "core" {
		if err := startCoreProxy(ctx, root, srv); err != nil {
			slog.Warn("core proxy setup failed — falling back to python mode", "err", err)
		}
	}

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

// startCoreProxy starts the legacy Gradio server and native extensions as
// managed engines, then mounts a router on the HTTP server that proxies "/" to
// Gradio (C5.5, --proxy=core mode).
func startCoreProxy(ctx context.Context, root paths.Root, srv *server.Server) error {
	mgr := lifecycle.New()

	// Register and start the legacy Gradio engine.
	gradioManifest := legacy.LegacyGradioManifest(root)
	if err := mgr.Register(gradioManifest); err != nil {
		return fmt.Errorf("registering legacy-gradio: %w", err)
	}
	if err := mgr.Start(ctx, "legacy-gradio"); err != nil {
		return fmt.Errorf("starting legacy-gradio: %w", err)
	}

	gradioPort := mgr.Status("legacy-gradio").Port
	gradioURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", gradioPort))

	// Register and start each native extension engine.
	nativeExts, err := legacy.LoadNativeExtensions(root.Dir())
	if err != nil {
		slog.Warn("loading native extensions", "err", err)
	}
	for _, m := range nativeExts {
		if regErr := mgr.Register(m); regErr != nil {
			slog.Warn("registering native extension", "id", m.PackageName, "err", regErr)
			continue
		}
		if startErr := mgr.Start(ctx, m.PackageName); startErr != nil {
			slog.Warn("starting native extension", "id", m.PackageName, "err", startErr)
		}
	}

	// Wire the router: "/" → Gradio, each extension at its own prefix.
	rt := router.New()
	rt.AddRoute("/", gradioURL)

	// Mount the router on the server for all non-/api requests.
	srv.Handle("/", rt)
	return nil
}
