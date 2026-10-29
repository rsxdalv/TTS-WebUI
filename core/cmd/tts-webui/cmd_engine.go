package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/catalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/lifecycle"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/native"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/update"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/venv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// defaultCatalogURL is the default catalog endpoint.
const defaultCatalogURL = "https://raw.githubusercontent.com/rsxdalv/TTS-WebUI/main/extensions.json"

func init() {
	register(command{
		name:    "engine",
		summary: "manage TTS engine manifests",
		run:     runEngine,
	})
}

func runEngine(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("engine", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	catalogURL := fs.String("catalog-url", defaultCatalogURL, "catalog URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(fs.Output(), "usage: tts-webui engine <subcommand>")
		fmt.Fprintln(fs.Output(), "  list                       print engine manifests as JSON")
		fmt.Fprintln(fs.Output(), "  catalog                    show catalog engines")
		fmt.Fprintln(fs.Output(), "  catalog --refresh          force a catalog refresh")
		fmt.Fprintln(fs.Output(), "  install <id>               create the Python venv for an engine")
		fmt.Fprintln(fs.Output(), "  native install <id>        download/verify/extract native binary")
		fmt.Fprintln(fs.Output(), "  start <id>                 start an engine")
		fmt.Fprintln(fs.Output(), "  stop <id>                  stop an engine")
		fmt.Fprintln(fs.Output(), "  logs <id>                  stream engine logs (SSE)")
		fmt.Fprintln(fs.Output(), "  update <id>                blue/green update (venv or native engine)")
		return nil
	}

	switch fs.Arg(0) {
	case "list":
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		enc := json.NewEncoder(fs.Output())
		enc.SetIndent("", "  ")
		return enc.Encode(manifests)

	case "catalog":
		sub := flag.NewFlagSet("engine catalog", flag.ContinueOnError)
		refresh := sub.Bool("refresh", false, "force a catalog fetch")
		if err := sub.Parse(fs.Args()[1:]); err != nil {
			return err
		}
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		cacheDir := root.CoreData() + "/catalog"
		var manifests []manifest.Manifest
		if *refresh {
			manifests, err = catalog.Fetch(ctx, *catalogURL, cacheDir)
		} else {
			manifests, err = catalog.Load(cacheDir)
			if err != nil {
				// Fall back to fetch if cache is absent.
				manifests, err = catalog.Fetch(ctx, *catalogURL, cacheDir)
			}
		}
		if err != nil {
			return fmt.Errorf("catalog: %w", err)
		}
		enc := json.NewEncoder(fs.Output())
		enc.SetIndent("", "  ")
		return enc.Encode(manifests)

	case "install":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: tts-webui engine install <id>")
		}
		engineID := fs.Arg(1)
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		var m manifest.Manifest
		found := false
		for _, mm := range manifests {
			if mm.PackageName == engineID {
				m = mm
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("engine %q not found in manifests", engineID)
		}
		gpuResult, gpuErr := gpu.Detect(ctx)
		if gpuErr != nil {
			gpuResult = gpu.Result{Label: gpu.CPU}
		}
		fmt.Fprintf(fs.Output(), "installing venv for %s (gpu=%s)...\n", engineID, gpuResult.Label)
		if err := venv.Install(ctx, root, m, string(gpuResult.Label)); err != nil {
			return fmt.Errorf("venv install: %w", err)
		}
		fmt.Fprintln(fs.Output(), "done")
		return nil

	case "start", "stop", "logs":
		// These subcommands require a running server; handled via HTTP in daemon mode.
		// For direct CLI use, create a one-off manager.
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: tts-webui engine %s <id>", fs.Arg(0))
		}
		engineID := fs.Arg(1)
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		mgr := lifecycle.New()
		for _, m := range manifests {
			_ = mgr.Register(m)
		}
		switch fs.Arg(0) {
		case "start":
			return mgr.Start(ctx, engineID)
		case "stop":
			return mgr.Stop(engineID)
		case "logs":
			ch, cancel := mgr.LogStream(engineID)
			defer cancel()
			for line := range ch {
				fmt.Fprintln(fs.Output(), line)
			}
			return nil
		}

	case "native":
		if fs.NArg() < 3 || fs.Arg(1) != "install" {
			return fmt.Errorf("usage: tts-webui engine native install <id> [--gpu <label>]")
		}
		sub := flag.NewFlagSet("engine native install", flag.ContinueOnError)
		gpuFlag := sub.String("gpu", "", "GPU label (e.g. cpu, cuda, rocm); auto-detected if empty")
		if err := sub.Parse(fs.Args()[2:]); err != nil {
			return err
		}
		if sub.NArg() == 0 {
			return fmt.Errorf("usage: tts-webui engine native install <id>")
		}
		engineID := sub.Arg(0)
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		var m manifest.Manifest
		found := false
		for _, mn := range manifests {
			if mn.PackageName == engineID {
				m = mn
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("engine %q not found in manifests", engineID)
		}
		gpuLabel := *gpuFlag
		if gpuLabel == "" {
			gpuResult, gpuErr := gpu.Detect(ctx)
			if gpuErr != nil {
				gpuResult = gpu.Result{Label: gpu.CPU}
			}
			gpuLabel = string(gpuResult.Label)
		}
		cacheDir := root.CoreData() + "/native"
		fmt.Fprintf(fs.Output(), "resolving native binary for %s (gpu=%s)...\n", engineID, gpuLabel)
		binPath, err := native.Resolve(ctx, m, gpuLabel, cacheDir)
		if err != nil {
			return fmt.Errorf("native resolve: %w", err)
		}
		fmt.Fprintf(fs.Output(), "binary ready: %s\n", binPath)
		return nil

	case "update":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: tts-webui engine update <id> [--root <dir>]")
		}
		engineID := fs.Arg(1)
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		var m manifest.Manifest
		found := false
		for _, mm := range manifests {
			if mm.PackageName == engineID {
				m = mm
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("engine %q not found in manifests", engineID)
		}
		gpuResult, gpuErr := gpu.Detect(ctx)
		if gpuErr != nil {
			gpuResult = gpu.Result{Label: gpu.CPU}
		}
		switch m.Engine.Kind {
		case manifest.KindPythonVenv:
			fmt.Fprintf(fs.Output(), "updating venv for %s (gpu=%s)...\n", engineID, gpuResult.Label)
			if err := update.UpdateVenv(ctx, root, m, string(gpuResult.Label)); err != nil {
				return fmt.Errorf("engine update: %w", err)
			}
		case manifest.KindNative:
			cacheDir := root.CoreData() + "/native"
			fmt.Fprintf(fs.Output(), "updating native binary for %s (gpu=%s)...\n", engineID, gpuResult.Label)
			if err := update.UpdateNative(ctx, m, string(gpuResult.Label), cacheDir, nil); err != nil {
				return fmt.Errorf("engine update: %w", err)
			}
		default:
			return fmt.Errorf("engine %q: kind %q does not support update", engineID, m.Engine.Kind)
		}
		fmt.Fprintln(fs.Output(), "done")
		return nil
	}
	return fmt.Errorf("unknown engine subcommand %q", fs.Arg(0))
}

// RegisterEngineRoutes adds the /api/engines/* endpoints to the server.
// Called from the daemon's startup so the lifecycle manager is shared.
func RegisterEngineRoutes(srv *server.Server, mgr *lifecycle.Manager) {
	srv.Handle("GET /api/engines", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := mgr.IDs()
		type statusEntry struct {
			ID     string                 `json:"id"`
			Status lifecycle.EngineStatus `json:"status"`
		}
		out := make([]statusEntry, 0, len(ids))
		for _, id := range ids {
			out = append(out, statusEntry{ID: id, Status: mgr.Status(id)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))

	srv.Handle("POST /api/engines/{id}/start", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := mgr.Start(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		st := mgr.Status(id)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)
	}))

	srv.Handle("POST /api/engines/{id}/stop", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := mgr.Stop(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	srv.Handle("GET /api/engines/{id}/logs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ch, cancel := mgr.LogStream(id)
		defer cancel()

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		for {
			select {
			case <-r.Context().Done():
				return
			case line, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	}))
}
