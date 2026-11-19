package launcher

// How the React UI runs (S2, D18). With the proxy on, Go serves the static
// export and no Next process runs. An install made by the old scripts has only
// the server build, so Go proxies `next start` and says so. With
// TTS_GO_PROXY=0 Next runs on port 3000 as before, built first if needed.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	recover bool // run `node build-static.js --recover` first (CONTRACTS.md section 10)
	build   bool // run `npm run build` first (escape hatch without a server build)
	// missing names the export's claims that TTS_GO_ROUTES turns off, when
	// they are why Next serves the UI instead of the export.
	missing string
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
			return reactUIPlan{mode: modeStatic}, nil
		}
		// Without these claims the export's generation and upload cannot
		// work, so Next serves the UI when it can, and otherwise nothing starts.
		if usableServerBuild(root) || recover {
			return reactUIPlan{mode: modeNextProxied, recover: recover, missing: missing}, nil
		}
		return reactUIPlan{}, fmt.Errorf("%w: TTS_GO_ROUTES turns off %s, and there is no Next server build to fall back to; unset TTS_GO_ROUTES or add those claims", errClaimsOff, missing)
	}
	if sw.Proxy {
		if !usableServerBuild(root) && !recover {
			return reactUIPlan{}, errUINotBuilt
		}
		return reactUIPlan{mode: modeNextProxied, recover: recover}, nil
	}
	return reactUIPlan{mode: modeNextDirect, recover: recover, build: !usableServerBuild(root) && !recover}, nil
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
		if err := s.runReactUITool(ctx, "node", "build-static.js", "--recover"); err != nil {
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
	if plan.build {
		slog.Warn("react-ui: TTS_GO_PROXY=0 needs the Next server build, which is missing; building it once with npm run build (this takes a few minutes)")
		if err := s.runReactUITool(ctx, "npm", "run", "build"); err != nil {
			return plan, fmt.Errorf("building the React UI (npm run build): %w", err)
		}
		if !usableServerBuild(s.root) {
			return plan, errors.New("npm run build finished but react-ui/.next is still not a usable server build")
		}
		slog.Info("react-ui: server build finished")
	}
	switch plan.mode {
	case modeStatic:
		slog.Info("react-ui: serving the static export from react-ui/out; no Next process")
	case modeNextProxied:
		if plan.missing != "" {
			slog.Warn("react-ui: the static export needs the route claims " + plan.missing + ", which TTS_GO_ROUTES turns off, so the Next server build is proxied through next start instead")
		} else {
			slog.Warn("react-ui: no static export in react-ui/out, so the Next server build is proxied through next start; run Update to switch to the static export")
		}
	}
	return plan, nil
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
