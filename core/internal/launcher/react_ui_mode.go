package launcher

// How the React UI starts. Go serves the static export in react-ui/out (built by
// Vite, `npm run build:static`) on the React port; no Node process runs. Before
// the app starts the launcher checks that the export exists, that the route
// claims it needs are on, and that it was built from the install's commit.

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

// reactPrep runs the React UI build step before the app starts.
type reactPrep struct {
	root         paths.Root
	systemPython bool // run node tools from PATH, not the micromamba env
	// runTool, when set, replaces the real tool runner (tests).
	runTool func(ctx context.Context, name string, args ...string) error
}

func newReactPrep(root paths.Root, systemPython bool) *reactPrep {
	return &reactPrep{root: root, systemPython: systemPython}
}

// reactUIPlan is what Start must do before starting the processes.
type reactUIPlan struct {
	// buildExport runs `npm run build:static` first: there is no export yet.
	buildExport bool
	// rebuildExport runs `npm run build:static` first: the export comes from
	// another commit than the install (R2-6).
	rebuildExport bool
}

// exportStale reports whether the static export comes from another commit than
// the install's HEAD, for example after an Update that pulled new sources but
// did not rebuild the export (R2-6). An export that recorded no commit, or an
// install whose HEAD is unknown, is never called stale.
func exportStale(root paths.Root) bool {
	if !react_ui_backend.HasExport(root.Dir()) {
		return false
	}
	marker := filepath.Join(react_ui_backend.ExportDir(root.Dir()), react_ui_backend.ExportMarker)
	commit, ok := buildCommit(marker)
	if !ok {
		return false
	}
	head := gitHead(root)
	return head != "" && commit != head
}

// buildCommit reads the "commit" a build recorded (out/_tts-export.json). ok is
// false when there is none.
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

// errClaimsOff is the error for an install whose needed claims are off.
var errClaimsOff = errors.New("the React UI needs route claims that are off")

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

// errUINotBuilt is the error for an install with no export and no way to build one.
var errUINotBuilt = errors.New("the React UI is not built: run Install or Update in the launcher, then start the app again")

// canBuildExport reports whether `npm run build:static` can run: react-ui has
// the script and its dependencies are installed.
func canBuildExport(root paths.Root) bool {
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
	if _, ok := pkg.Scripts["build:static"]; !ok {
		return false
	}
	st, err := os.Stat(filepath.Join(root.Dir(), "react-ui", "node_modules"))
	return err == nil && st.IsDir()
}

// planReactUI decides what must happen from the switches and what is on disk.
func planReactUI(root paths.Root, sw switches.Set) (reactUIPlan, error) {
	// Without these claims the export's generation and upload cannot work, and
	// nothing else answers those API paths.
	if missing := missingExportClaims(sw); missing != "" {
		return reactUIPlan{}, fmt.Errorf("%w: TTS_GO_ROUTES turns off %s; unset TTS_GO_ROUTES or add those claims", errClaimsOff, missing)
	}
	if !react_ui_backend.HasExport(root.Dir()) {
		if !canBuildExport(root) {
			return reactUIPlan{}, errUINotBuilt
		}
		return reactUIPlan{buildExport: true}, nil
	}
	return reactUIPlan{rebuildExport: exportStale(root)}, nil
}

// prepareReactUI builds the export when the plan asks for it.
func (s *reactPrep) prepareReactUI(ctx context.Context, sw switches.Set) error {
	plan, err := planReactUI(s.root, sw)
	if err != nil {
		return err
	}
	switch {
	case plan.buildExport:
		slog.Warn("react-ui: there is no static export in react-ui/out; building it once with npm run build:static (this takes a few minutes)")
	case plan.rebuildExport:
		slog.Warn("react-ui: the static export was built from another commit than this install, so it would serve an old UI; rebuilding it once with npm run build:static (this takes a few minutes)")
	default:
		slog.Info("react-ui: serving the static export from react-ui/out")
		return nil
	}
	// A failed build never falls back to an old export.
	if err := s.tool()(ctx, "npm", "run", "build:static"); err != nil {
		return fmt.Errorf("building the React UI (npm run build:static) failed, so the app was not started: %w", err)
	}
	if !react_ui_backend.HasExport(s.root.Dir()) || exportStale(s.root) {
		return errors.New("npm run build:static finished but react-ui/out is still missing or stale, so the app was not started")
	}
	slog.Info("react-ui: static export built")
	return nil
}

// tool returns the function that runs react-ui tools; tests replace it.
func (s *reactPrep) tool() func(ctx context.Context, name string, args ...string) error {
	if s.runTool != nil {
		return s.runTool
	}
	return s.runReactUITool
}

// runReactUITool runs a node tool in react-ui, inside the conda environment (or
// straight from PATH with system python), and logs its output.
func (s *reactPrep) runReactUITool(ctx context.Context, name string, args ...string) error {
	p := &procs.Proc{}
	dir := filepath.Join(s.root.Dir(), "react-ui")
	var err error
	if s.systemPython {
		err = p.Start(ctx, name, args, dir, os.Environ())
	} else {
		full := append([]string{"-p", installEnvPath(s.root), "run", name}, args...)
		err = p.Start(ctx, micromambaPath(s.root), full, dir, mambaEnv(s.root))
	}
	if err != nil {
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
