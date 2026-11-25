// Package legacyenv is the legacy-env platform: the micromamba main env under
// installer_files/env, adopted as it is. Go never installs, repairs or removes
// it (the installer step creates it, and stays for existing installs). It is
// the one package that knows what `micromamba -p <env> run` looks like.
package legacyenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Name is the platform's name.
const Name = "legacy-env"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// Env (KEY=VALUE) is added to every command's environment after the
	// inherited one and MAMBA_ROOT_PREFIX, so it overrides them.
	Env []string
	// Prefix replaces `micromamba -p installer_files/env run` (tests point it
	// at a fake child). Empty means the real one.
	Prefix []string
	// Packages returns the main env's distributions, for EntryStatus.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Platform is the legacy-env platform.
type Platform struct {
	platform.Entries
	root   paths.Root
	env    []string
	prefix []string
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, env: append([]string(nil), o.Env...)}
	p.prefix = append([]string(nil), o.Prefix...)
	if len(p.prefix) == 0 {
		p.prefix = []string{micromambaPath(o.Root), "-p", envPath(o.Root), "run"}
	}
	p.Entries = platform.Entries{Root: o.Root.Dir(), Cmd: p.Command, Packages: o.Packages}
	return p
}

// WithEnv returns a copy whose commands also carry env.
func (p *Platform) WithEnv(env []string) platform.Platform {
	q := *p
	q.env = append(append([]string(nil), p.env...), env...)
	q.Entries.Cmd = q.Command
	return &q
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{"gradio-host"} }

// Status says whether Python can be started: the prefix's program exists and,
// for `micromamba -p <env> run`, so does the env.
func (p *Platform) Status(context.Context) platform.Status {
	if _, err := exec.LookPath(p.prefix[0]); err != nil {
		return platform.Status{Detail: "no Python environment: " + p.prefix[0] + " was not found (the install has not been done)"}
	}
	for i := 1; i+1 < len(p.prefix); i++ {
		if p.prefix[i] == "-p" || p.prefix[i] == "--prefix" {
			if st, err := os.Stat(p.prefix[i+1]); err != nil || !st.IsDir() {
				return platform.Status{Detail: "no Python environment: " + p.prefix[i+1] + " does not exist (the install has not been done)"}
			}
		}
	}
	return platform.Status{Ready: true}
}

func (p *Platform) Install(context.Context, platform.Emit) error   { return platform.ErrNotInstallable }
func (p *Platform) Repair(context.Context, platform.Emit) error    { return platform.ErrNotInstallable }
func (p *Platform) Uninstall(context.Context, platform.Emit) error { return platform.ErrNotInstallable }

// Command runs argv in the main env: `micromamba -p <env> run argv...`, with
// the arguments escaped for the batch file micromamba uses on Windows. "python"
// of an own-venv entry is the venv's python, run directly.
func (p *Platform) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	env := p.environ()
	if c, ok := platform.OwnVenvCommand(p.root.Dir(), e, argv, env); ok {
		return c, nil
	}
	args := append([]string(nil), p.prefix[1:]...)
	rest := argv
	if runsThroughCmd(p.prefix) {
		rest = escapeForMambaRun(argv)
	}
	return platform.Command{Program: p.prefix[0], Args: append(args, rest...), Env: env, Dir: p.root.Dir()}, nil
}

// environ is the inherited environment, MAMBA_ROOT_PREFIX (so micromamba does
// not fall back to ~/.micromamba) and the options' own variables.
func (p *Platform) environ() []string {
	env := append(os.Environ(), "MAMBA_ROOT_PREFIX="+filepath.Join(p.root.InstallerFiles(), "mamba"))
	return append(env, p.env...)
}

func micromambaPath(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "mamba", "micromamba.exe")
	}
	return filepath.Join(root.InstallerFiles(), "mamba", "micromamba")
}

func envPath(root paths.Root) string { return filepath.Join(root.InstallerFiles(), "env") }

// runsThroughCmd reports whether prefix is `micromamba run` on Windows.
func runsThroughCmd(prefix []string) bool {
	if len(prefix) == 0 || runtime.GOOS != "windows" {
		return false
	}
	base := strings.ToLower(filepath.Base(prefix[0]))
	return base == "micromamba.exe" || base == "micromamba"
}

// escapeForMambaRun makes arguments reach the program unchanged through
// `micromamba run` on Windows, which hands them to cmd.exe in a batch file: there
// < > & | ( ) ^ are operators (so "gradio>=1" would redirect output to a file
// named "=1") and % starts a variable. A caret escapes the operators; the batch
// file wants %% for a percent sign.
func escapeForMambaRun(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		var b strings.Builder
		for _, c := range a {
			switch c {
			case '^', '&', '|', '<', '>', '(', ')':
				b.WriteByte('^')
			case '%':
				b.WriteByte('%')
			}
			b.WriteRune(c)
		}
		out[i] = b.String()
	}
	return out
}
