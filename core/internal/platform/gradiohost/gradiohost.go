// Package gradiohost is the Gradio host as an entry (P3, D23, D24): the main
// Python env of a new install, made with the uv platform in .venvs/gradio-host.
// It holds torch on the default row of the GPU choice and requirements.txt, and
// every in-process tab installs into it. It stops being special in P6.
//
// As a Platform it answers for "the main env": Command("python") is the host
// venv's python, run directly, and the entry level (Create, Check, Remove,
// EntryStatus) is pip in that venv, with torch held at the version the venv has.
// It is not ready until the host is installed. An install folder that already has
// an install that already has the legacy env keeps running on legacy-env (detect chooses).
package gradiohost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

// Name is the platform's name and the package name of the entry (its venv is
// .venvs/gradio-host). The name is reserved: no catalog entry may take it.
const Name = "gradio-host"

// Title is the name shown in messages and on the Extensions page.
const Title = "Gradio host"

// ConstraintsFile is written beside the venv's marker files: the torch pins of
// the row the host was installed on, passed to every pip install so a tab's
// requirements cannot move torch.
const ConstraintsFile = "constraints.txt"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// UV makes the venv and runs uv.
	UV *uv.Platform
	// Env (KEY=VALUE) is added to every command's environment, last.
	Env []string
	// Packages returns the host venv's distributions, for EntryStatus.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Platform is the Gradio host.
type Platform struct {
	platform.Entries
	root paths.Root
	uv   *uv.Platform
	env  []string
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, uv: o.UV, env: append([]string(nil), o.Env...)}
	p.Entries = platform.Entries{Root: o.Root.Dir(), Cmd: p.Command, Packages: o.Packages,
		Constraints: p.ConstraintsPath()}
	return p
}

// Entry is the host as the uv platform sees it: an own-venv entry whose
// requirements are requirements.txt (pip is installed too, because the tabs'
// installs and the pip list run through it).
func Entry() platform.Entry {
	return platform.Entry{Package: Name, Name: Title, Own: true,
		Requirements: []string{"pip", "-r", "requirements.txt"}}
}

// WithEnv returns a copy whose commands also carry env.
func (p *Platform) WithEnv(env []string) platform.Platform {
	q := *p
	q.env = append(append([]string(nil), p.env...), env...)
	q.Entries.Cmd = q.Command
	return &q
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{Name} }

// Python is the host venv's python.
func (p *Platform) Python() string { return platform.VenvPython(p.root.Dir(), Name) }

// ConstraintsPath is the file of torch pins.
func (p *Platform) ConstraintsPath() string {
	return filepath.Join(platform.VenvDir(p.root.Dir(), Name), ConstraintsFile)
}

// Installed reports the state of the host venv: installed when its install
// finished (entry.json is there), partial when a directory is there and it did
// not.
func (p *Platform) Installed() platform.EntryStatus {
	return platform.OwnVenvStatus(p.root.Dir(), Name)
}

// Status is Ready when the host is installed and its python is there.
func (p *Platform) Status(context.Context) platform.Status {
	st := p.Installed()
	switch {
	case st.Installed && platform.OwnVenvExists(p.root.Dir(), Name):
		return platform.Status{Ready: true}
	case st.Partial:
		return platform.Status{Detail: "the Gradio host install did not finish: install it again from the Extensions page"}
	}
	return platform.Status{Detail: "the Gradio host is not installed: install it from the Extensions page (it installs the uv platform first)"}
}

// Install makes the host: the venv, torch, requirements.txt, then the torch pins
// for later installs. The uv platform must be installed. It can be run again
// after a cancel or a failure.
func (p *Platform) Install(ctx context.Context, emit platform.Emit) error {
	if p.uv == nil {
		return errors.New("the uv platform is not available")
	}
	if st := p.uv.Status(ctx); !st.Ready {
		return fmt.Errorf("the Gradio host needs the uv platform: %s", st.Detail)
	}
	if _, err := os.Stat(filepath.Join(p.root.Dir(), "requirements.txt")); err != nil {
		return fmt.Errorf("requirements.txt is not in %s: the Gradio host cannot be installed from here", p.root.Dir())
	}
	if !p.uv.Create(ctx, Entry(), emit) {
		return errors.New("the Gradio host install failed")
	}
	if err := p.writeConstraints(); err != nil {
		return err
	}
	emit("Successfully set up the Gradio host")
	return nil
}

// Repair is Install again.
func (p *Platform) Repair(ctx context.Context, emit platform.Emit) error { return p.Install(ctx, emit) }

// Uninstall removes the host venv. The caller refuses while a tab needs it.
func (p *Platform) Uninstall(ctx context.Context, emit platform.Emit) error {
	if p.uv == nil {
		return errors.New("the uv platform is not available")
	}
	if !p.uv.Remove(ctx, Entry(), emit) {
		return errors.New("removing the Gradio host failed")
	}
	return nil
}

// writeConstraints writes the torch pins of the row the venv was made with.
func (p *Platform) writeConstraints() error {
	st, ok := p.uv.EntryInfo(Name)
	if !ok {
		return errors.New("the Gradio host has no entry.json after its install")
	}
	row, err := torchrows.ByName(st.TorchRow, p.uv.OS())
	if err != nil {
		return err
	}
	var lines []string
	for _, r := range append(append([]string(nil), row.Packages...), row.Xformers) {
		if strings.Contains(r, "==") {
			lines = append(lines, r)
		}
	}
	return os.WriteFile(p.ConstraintsPath(), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// Command resolves a logical program: "python" is the host venv's python run
// directly (no shell, nothing to escape), "uv" is the uv platform's. An own-venv
// entry's python is its own venv's.
func (p *Platform) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	if len(argv) == 0 {
		return platform.Command{}, errors.New("empty command")
	}
	var base []string
	if p.uv != nil {
		if c, err := p.uv.Command(nil, []string{"uv"}); err == nil {
			base = c.Env
		}
	}
	if base == nil {
		base = os.Environ()
	}
	env := append(append([]string(nil), base...), p.env...)
	if c, ok := platform.OwnVenvCommand(p.root.Dir(), e, argv, env); ok {
		return c, nil
	}
	switch argv[0] {
	case "python":
		if !platform.OwnVenvExists(p.root.Dir(), Name) {
			return platform.Command{}, errors.New("the Gradio host is not installed")
		}
		return platform.Command{Program: p.Python(), Args: append([]string(nil), argv[1:]...), Env: env, Dir: p.root.Dir()}, nil
	case "uv":
		if p.uv == nil {
			return platform.Command{}, errors.New("the uv platform is not available")
		}
		c, err := p.uv.Command(nil, argv)
		if err != nil {
			return c, err
		}
		c.Env = env
		return c, nil
	}
	return platform.Command{}, fmt.Errorf("the Gradio host platform does not provide %q", argv[0])
}
