package launcher

// How the React UI runs (S2, D18). With the proxy on, Go serves the static
// export and no Next process runs. An install made by the old scripts has only
// the server build, so Go proxies `next start` and says so. With
// TTS_GO_PROXY=0 Next runs on port 3000 as before, built first if needed.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

type reactUIMode int

const (
	// modeStatic: Go serves react-ui/out. No Next process.
	modeStatic reactUIMode = iota
	// modeNextProxied: Go proxies `next start` on a private port.
	modeNextProxied
	// modeNextDirect: TTS_GO_PROXY=0, Next owns port 3000.
	modeNextDirect
)

// reactUIPlan is what Start must do before starting the processes.
type reactUIPlan struct {
	mode    reactUIMode
	recover bool        // run `node build-static.js --recover` first (CONTRACTS.md section 10)
	build   buildReason // run `npm run build` first, and why
	// missing names the export's claims that TTS_GO_ROUTES turns off, when
	// they are why Next serves the UI instead of the export.
	missing string
	// rebuildExport runs `npm run build:static` first: the export comes from
	// another commit than the install (R2-6).
	rebuildExport bool
}

// buildReason says why the server build must be built before Next starts.
type buildReason string

const (
	buildNone    buildReason = ""
	buildMissing buildReason = "missing" // TTS_GO_PROXY=0 and no usable server build
	buildStale   buildReason = "stale"   // the server build is older than the export (S2F)
)

// serverBuildStale reports whether a usable server build is older than the
// static export: an Update rebuilt only the export, so next start would serve
// the old UI. Both times are file modification times, of .next/BUILD_ID (next
// build writes it anew, and build:static's rename and restore keep its time)
// and of out/_tts-export.json (written last in a complete export).
func serverBuildStale(root paths.Root) bool {
	if !usableServerBuild(root) {
		return false
	}
	if commit, ok := buildCommit(filepath.Join(root.Dir(), "react-ui", ".next", "tts-build.json")); ok {
		if head := gitHead(root); head != "" {
			return commit != head
		}
	}
	if !react_ui_backend.HasExport(root.Dir()) {
		return false
	}
	server, err := os.Stat(filepath.Join(root.Dir(), "react-ui", ".next", "BUILD_ID"))
	if err != nil {
		return false
	}
	export, err := os.Stat(filepath.Join(react_ui_backend.ExportDir(root.Dir()), react_ui_backend.ExportMarker))
	if err != nil {
		return false
	}
	return server.ModTime().Before(export.ModTime())
}

// exportStale reports whether the static export comes from another commit than
// the install's HEAD, for example after an Update that ran the original
// scripts and rebuilt only the server build (R2-6). An export without a
// recorded commit is stale when it is older than a usable server build.
func exportStale(root paths.Root) bool {
	if !react_ui_backend.HasExport(root.Dir()) {
		return false
	}
	marker := filepath.Join(react_ui_backend.ExportDir(root.Dir()), react_ui_backend.ExportMarker)
	if commit, ok := buildCommit(marker); ok {
		if head := gitHead(root); head != "" {
			return commit != head
		}
		return false
	}
	if !usableServerBuild(root) {
		return false
	}
	server, err1 := os.Stat(filepath.Join(root.Dir(), "react-ui", ".next", "BUILD_ID"))
	export, err2 := os.Stat(marker)
	return err1 == nil && err2 == nil && export.ModTime().Before(server.ModTime())
}

// buildCommit reads the "commit" a build recorded (out/_tts-export.json or
// .next/tts-build.json). ok is false when there is none.
func buildCommit(file string) (string, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	var rec struct {
		Commit string `json:"commit"`
	}
	if json.Unmarshal(data, &rec) != nil || rec.Commit == "" {
		return "", false
	}
	return rec.Commit, true
}

// gitHead returns the install's HEAD commit, or "" when it cannot tell.
// Tests replace it.
var gitHead = func(root paths.Root) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := defaultGitRunner(ctx, root.Dir(), "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// staleBuild returns buildStale when the server build that Next would serve
// is older than the export. It cannot tell while the build is set aside; the
// plan made after the recover decides then.
func staleBuild(root paths.Root, recover bool) buildReason {
	if !recover && serverBuildStale(root) {
		return buildStale
	}
	return buildNone
}

// errClaimsOff is the error for a static install whose needed claims are off.
var errClaimsOff = errors.New("the static React UI needs route claims that are off")

// missingExportClaims lists, comma separated, the claims the export needs that
// the switches turn off.
func missingExportClaims(sw switches.Set) string {
	var missing []string
	for _, name := range react_ui_backend.ExportClaims {
		if !sw.Route(name) {
			missing = append(missing, name)
		}
	}
	return strings.Join(missing, ", ")
}

// errUINotBuilt is the error for an install with neither build.
var errUINotBuilt = errors.New("the React UI is not built: run Install or Update in the launcher, then start the app again")

// usableServerBuild is CONTRACTS.md section 10's rule: .next/BUILD_ID and
// .next/prerender-manifest.json both exist.
func usableServerBuild(root paths.Root) bool {
	next := filepath.Join(root.Dir(), "react-ui", ".next")
	for _, name := range []string{"BUILD_ID", "prerender-manifest.json"} {
		if st, err := os.Stat(filepath.Join(next, name)); err != nil || !st.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// serverBuildSetAside reports whether a killed export left the server build in
// .next-server-saved, which build-static.js --recover puts back.
func serverBuildSetAside(root paths.Root) bool {
	st, err := os.Stat(filepath.Join(root.Dir(), "react-ui", ".next-server-saved"))
	return err == nil && st.IsDir()
}

// planReactUI decides the mode from the switches and what is on disk. It is
// called again after a recover, because that can change the answer.
func planReactUI(root paths.Root, sw switches.Set) (reactUIPlan, error) {
	recover := serverBuildSetAside(root)
	if sw.Proxy && react_ui_backend.HasExport(root.Dir()) {
		missing := missingExportClaims(sw)
		if missing == "" {
			return reactUIPlan{mode: modeStatic, rebuildExport: !recover && exportStale(root)}, nil
		}
		// Without these claims the export's generation and upload cannot
		// work, so Next serves the UI when it can, and otherwise nothing starts.
		if usableServerBuild(root) || recover {
			return reactUIPlan{mode: modeNextProxied, recover: recover, missing: missing, build: staleBuild(root, recover)}, nil
		}
		return reactUIPlan{}, fmt.Errorf("%w: TTS_GO_ROUTES turns off %s, and there is no Next server build to fall back to; unset TTS_GO_ROUTES or add those claims", errClaimsOff, missing)
	}
	if sw.Proxy {
		if !usableServerBuild(root) && !recover {
			return reactUIPlan{}, errUINotBuilt
		}
		return reactUIPlan{mode: modeNextProxied, recover: recover}, nil
	}
	build := staleBuild(root, recover)
	if !usableServerBuild(root) && !recover {
		build = buildMissing
	}
	return reactUIPlan{mode: modeNextDirect, recover: recover, build: build}, nil
}

// prepareReactUI runs the recover and build steps the plan asks for and
// returns the final plan.
func (s *Supervisor) prepareReactUI(ctx context.Context, sw switches.Set) (reactUIPlan, error) {
	plan, err := planReactUI(s.root, sw)
	if err != nil {
		return plan, err
	}
	if plan.recover {
		slog.Info("react-ui: a static export build was interrupted; restoring the server build (node build-static.js --recover)")
		if err := s.tool()(ctx, "node", "build-static.js", "--recover"); err != nil {
			return plan, fmt.Errorf("restoring the React UI server build: %w", err)
		}
		if plan, err = planReactUI(s.root, sw); err != nil {
			return plan, err
		}
		if plan.recover {
			return plan, errors.New("react-ui/.next-server-saved is still there after build-static.js --recover")
		}
		if plan.mode != modeStatic && !usableServerBuild(s.root) && sw.Proxy {
			return plan, errUINotBuilt
		}
	}
	switch plan.build {
	case buildMissing:
		slog.Warn("react-ui: TTS_GO_PROXY=0 needs the Next server build, which is missing; building it once with npm run build (this takes a few minutes)")
	case buildStale:
		slog.Warn("react-ui: the Next server build is older than the static export (an Update rebuilt only the export), so it would serve the old UI; rebuilding it once with npm run build (this takes a few minutes)")
	}
	if plan.build != buildNone {
		// A failed build never falls back to the old server build.
		if err := s.tool()(ctx, "npm", "run", "build"); err != nil {
			return plan, fmt.Errorf("building the React UI (npm run build) failed, so the app was not started: %w", err)
		}
		if !usableServerBuild(s.root) {
			return plan, errors.New("npm run build finished but react-ui/.next is still not a usable server build, so the app was not started")
		}
		if serverBuildStale(s.root) {
			return plan, errors.New("npm run build finished but react-ui/.next is still older than the static export, so the app was not started")
		}
		slog.Info("react-ui: server build finished")
	}
	if plan.rebuildExport {
		slog.Warn("react-ui: the static export was built from another commit than this install (an Update rebuilt only the Next server build), so it would serve an old UI; rebuilding it once with npm run build:static (this takes a few minutes)")
		// A failed export build never falls back to the old export.
		if err := s.tool()(ctx, "npm", "run", "build:static"); err != nil {
			return plan, fmt.Errorf("building the static React UI (npm run build:static) failed, so the app was not started: %w", err)
		}
		if !react_ui_backend.HasExport(s.root.Dir()) || exportStale(s.root) {
			return plan, errors.New("npm run build:static finished but react-ui/out is still missing or stale, so the app was not started")
		}
		slog.Info("react-ui: static export rebuilt")
	}
	switch plan.mode {
	case modeStatic:
		slog.Info("react-ui: serving the static export from react-ui/out; no Next process")
	case modeNextProxied:
		if plan.missing != "" {
			slog.Warn("react-ui: the static export needs the route claims " + plan.missing + ", which TTS_GO_ROUTES turns off, so the Next server build is proxied through next start instead")
		} else if hasBuildStaticScript(s.root) {
			slog.Warn("react-ui: no static export in react-ui/out, so the Next server build is proxied through next start; run Update to build the static export")
		} else {
			slog.Warn("react-ui: this UI version has no static export (react-ui has no build:static script), so it is served through next start")
		}
	}
	return plan, nil
}

// hasBuildStaticScript reports whether react-ui/package.json defines the
// build:static script that builds the static export (R2-4).
func hasBuildStaticScript(root paths.Root) bool {
	data, err := os.ReadFile(filepath.Join(root.Dir(), "react-ui", "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	_, ok := pkg.Scripts["build:static"]
	return ok
}

// tool returns the function that runs react-ui tools; tests replace it.
func (s *Supervisor) tool() func(ctx context.Context, name string, args ...string) error {
	if s.runTool != nil {
		return s.runTool
	}
	return s.runReactUITool
}

// runReactUITool runs a node tool in react-ui inside the conda environment and
// logs its output.
func (s *Supervisor) runReactUITool(ctx context.Context, name string, args ...string) error {
	full := append([]string{"-p", installEnvPath(s.root), "run", name}, args...)
	p := &procs.Proc{}
	dir := filepath.Join(s.root.Dir(), "react-ui")
	if err := p.Start(ctx, micromambaPath(s.root), full, dir, mambaEnv(s.root)); err != nil {
		return err
	}
	lines, unsub := p.Subscribe()
	defer unsub()
	go func() {
		for line := range lines {
			slog.Info("react-ui build", "line", line)
		}
	}()
	p.Wait()
	if code := p.ExitCode(); code != 0 {
		return fmt.Errorf("exit code %d", code)
	}
	return nil
}
