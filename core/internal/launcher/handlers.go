package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/preflight"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// --- download ---

type downloadRequest struct {
	URL string `json:"url"`
}

// handleDownload starts an async download+extract and returns immediately.
func (l *Launcher) handleDownload() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req downloadRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		url := req.URL
		if url == "" {
			url = canonicalDownloadURL
		}

		lg := newLog()
		l.setLog("download", lg)

		// Extract into the parent of the install root.
		destDir := filepath.Dir(l.root.Dir())

		go func() {
			if err := downloadAndExtract(context.Background(), url, destDir, lg); err != nil {
				lg.AddLine(fmt.Sprintf("ERROR: %v", err))
			} else {
				lg.AddLine("done")
			}
			lg.Close()
		}()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"started":true}`))
	})
}

// --- preflight ---

type preflightResponse struct {
	Checks []preflight.Check `json:"checks"`
	OK     bool              `json:"ok"`
}

// handlePreflight runs all preflight checks synchronously and writes the marker.
func (l *Launcher) handlePreflight() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checks, err := preflight.Run(r.Context(), l.root, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("preflight error: %v", err), http.StatusInternalServerError)
			return
		}
		ok := preflight.AllPassed(checks)
		if ok {
			if err := preflight.WriteMarker(l.root); err != nil {
				http.Error(w, fmt.Sprintf("writing marker: %v", err), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(preflightResponse{Checks: checks, OK: ok})
	})
}

// handlePreflightFixLongPaths returns guidance for enabling NTFS long paths.
// Actual registry elevation requires running as Administrator; we explain rather than attempt.
func (l *Launcher) handlePreflightFixLongPaths() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"message":"Run as Administrator to enable long paths, or enable it manually via Group Policy (Computer Configuration \u2192 Administrative Templates \u2192 System \u2192 Filesystem \u2192 Enable Win32 long paths)."}`))
	})
}

// handlePreflightFixMSVC returns a download link for Visual Studio Build Tools.
func (l *Launcher) handlePreflightFixMSVC() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"message":"Install Visual Studio Build Tools from https://visualstudio.microsoft.com/visual-cpp-build-tools/ and select the C++ build tools workload."}`))
	})
}

// --- mamba ---

// handleMamba spawns the platform mamba init script and returns immediately.
func (l *Launcher) handleMamba() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if existing := l.getProc("mamba"); existing != nil && existing.Running() {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"started":false,"already_running":true}`))
			return
		}

		script := mambaScript(l.root)

		p := &procs.Proc{}
		l.setProc("mamba", p)

		var name string
		var args []string
		if runtime.GOOS == "windows" {
			name = "cmd"
			args = []string{"/c", script}
		} else {
			name = "bash"
			args = []string{script}
		}

		if err := p.Start(context.Background(), name, args, l.root.Dir(), nil); err != nil {
			http.Error(w, fmt.Sprintf("starting mamba: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"started":true}`))
	})
}

// --- gpu ---

type gpuRequest struct {
	Label string `json:"label"`
}

// handleGPU writes a validated PyTorchChoice GPU label to installer_scripts/.gpu.
func (l *Launcher) handleGPU() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gpuRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Label == "" {
			http.Error(w, "body must be {\"label\": \"<gpu-label>\"}", http.StatusBadRequest)
			return
		}
		if _, err := gpu.Parse(req.Label); err != nil {
			http.Error(w, fmt.Sprintf("invalid gpu label: %v", err), http.StatusBadRequest)
			return
		}
		marker := l.root.MarkerFile(".gpu")
		if err := os.WriteFile(marker, []byte(req.Label), 0o644); err != nil {
			http.Error(w, fmt.Sprintf("writing .gpu: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// --- install ---

// handleInstall spawns micromamba run node init_app.js --silent.
func (l *Launcher) handleInstall() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if existing := l.getProc("install"); existing != nil && existing.Running() {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"started":false,"already_running":true}`))
			return
		}
		if !mambaDone(l.root) {
			http.Error(w, "mamba step not complete", http.StatusBadRequest)
			return
		}
		if !gpuDone(l.root) {
			http.Error(w, "GPU step not complete", http.StatusBadRequest)
			return
		}

		p := &procs.Proc{}
		l.setProc("install", p)

		mamba := micromambaPath(l.root)
		envPath := installEnvPath(l.root)
		initScript := filepath.Join(l.root.InstallerScripts(), "init_app.js")

		// Use filepath.ToSlash only for the node arg (not the binary paths).
		name := mamba
		args := []string{"-p", envPath, "run", "node", filepath.ToSlash(initScript), "--silent"}

		if err := p.Start(context.Background(), name, args, l.root.Dir(), mambaEnv(l.root)); err != nil {
			http.Error(w, fmt.Sprintf("starting install: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"started":true}`))
	})
}

// --- diagnostic ---

// handleAppDiagnostic spawns installer_scripts/tools/diagnostic.{bat,sh} and returns immediately.
func (l *Launcher) handleAppDiagnostic() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if existing := l.getProc("diagnostic"); existing != nil && existing.Running() {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"started":false,"already_running":true}`))
			return
		}

		var script string
		if runtime.GOOS == "windows" {
			script = filepath.Join(l.root.InstallerScripts(), "tools", "diagnostic.bat")
		} else {
			script = filepath.Join(l.root.InstallerScripts(), "tools", "diagnostic.sh")
		}

		p := &procs.Proc{}
		l.setProc("diagnostic", p)

		var name string
		var args []string
		if runtime.GOOS == "windows" {
			name = "cmd"
			args = []string{"/c", script}
		} else {
			name = "bash"
			args = []string{script}
		}

		if err := p.Start(context.Background(), name, args, l.root.Dir(), nil); err != nil {
			http.Error(w, fmt.Sprintf("starting diagnostic: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"started":true}`))
	})
}

// --- update ---

// handleUpdate checks isGitUpdated and re-runs install if needed.
func (l *Launcher) handleUpdate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if existing := l.getProc("update"); existing != nil && existing.Running() {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"started":false,"already_running":true}`))
			return
		}

		updated, err := isGitUpdated(r.Context(), l.root)
		if err != nil {
			// Non-fatal; proceed.
			updated = true
		}
		if updated {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"started":false,"already_updated":true}`))
			return
		}

		// Run the same install command to update.
		p := &procs.Proc{}
		l.setProc("update", p)

		mamba := micromambaPath(l.root)
		envPath := installEnvPath(l.root)
		initScript := filepath.Join(l.root.InstallerScripts(), "init_app.js")

		name := mamba
		args := []string{"-p", envPath, "run", "node", filepath.ToSlash(initScript), "--silent"}

		if err := p.Start(context.Background(), name, args, l.root.Dir(), mambaEnv(l.root)); err != nil {
			http.Error(w, fmt.Sprintf("starting update: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"started":true}`))
	})
}
