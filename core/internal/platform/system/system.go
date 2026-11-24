// Package system is the system platform: the Python, uv and npm on PATH. It is
// what a container, Colab or any environment the user manages looks like.
// Adopted, never installed.
package system

import (
	"context"
	"os"
	"os/exec"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Name is the platform's name.
const Name = "system"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// Env (KEY=VALUE) is added to every command's environment after the
	// inherited one, so it overrides it.
	Env []string
	// Python, when set, is the program used for "python" instead of the one on
	// PATH (the dev servers extcatalogd and pipd take it from a flag).
	Python string
	// Packages returns the env's distributions, for EntryStatus.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Platform is the system platform.
type Platform struct {
	platform.Entries
	root   paths.Root
	env    []string
	python string
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, env: append([]string(nil), o.Env...), python: o.Python}
	p.Entries = platform.Entries{Root: o.Root.Dir(), Cmd: p.Command, Packages: o.Packages}
	return p
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{"gradio-host", "uv"} }

// Status says whether python can be started.
func (p *Platform) Status(context.Context) platform.Status {
	if p.python != "" {
		if _, err := exec.LookPath(p.python); err != nil {
			return platform.Status{Detail: "no Python environment: " + p.python + " was not found"}
		}
		return platform.Status{Ready: true}
	}
	if _, err := exec.LookPath("python"); err != nil {
		return platform.Status{Detail: "no Python environment: python was not found on PATH"}
	}
	return platform.Status{Ready: true}
}

func (p *Platform) Install(context.Context, platform.Emit) error   { return platform.ErrNotInstallable }
func (p *Platform) Repair(context.Context, platform.Emit) error    { return platform.ErrNotInstallable }
func (p *Platform) Uninstall(context.Context, platform.Emit) error { return platform.ErrNotInstallable }

// Command runs argv straight from PATH. "python" of an own-venv entry is the
// venv's python.
func (p *Platform) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	env := append(os.Environ(), p.env...)
	if c, ok := platform.OwnVenvCommand(p.root.Dir(), e, argv, env); ok {
		return c, nil
	}
	prog := argv[0]
	if prog == "python" && p.python != "" {
		prog = p.python
	}
	return platform.Command{Program: prog, Args: append([]string(nil), argv[1:]...), Env: env, Dir: p.root.Dir()}, nil
}
