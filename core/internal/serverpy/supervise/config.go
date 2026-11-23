package supervise

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// Defaults applied by New for zero Config fields.
const (
	DefaultPublicPort    = 7770
	DefaultReactPort     = 3000
	DefaultGradioEntry   = "server.py"
	DefaultReadyTimeout  = 10 * time.Minute
	DefaultProbeInterval = 250 * time.Millisecond
)

// DefaultGradioArgs returns the default arguments passed to the gradio entry
// script. The React UI is started by the supervisor, never by server.py itself.
func DefaultGradioArgs() []string { return []string{"--no-react"} }

// Config describes the app process group. The zero value is not usable: Root
// is required. All other fields have defaults (see New).
type Config struct {
	// Root is the install root (cwd of every child, home of installer_files).
	Root paths.Root

	// PublicPort is the port the user-facing Gradio entry answers on (the proxy
	// tree's port today). It is used for readiness, AppLink, the browser URL
	// and the GRADIO_BACKEND_AUTOMATIC value handed to React. 0 = 7770.
	PublicPort int
	// ReactPort is the port `next start` gets via -p. 0 = 3000.
	ReactPort int

	// GradioEntry is the Python script run for Gradio, relative to Root.
	// "" = server.py. Integration switches this to server_core.py.
	GradioEntry string
	// GradioArgs are the arguments after the entry script. nil = --no-react,
	// an empty non-nil slice = no arguments. Used verbatim.
	GradioArgs []string
	// Forwarded launcher flags, appended to the gradio arguments when they are
	// not already there.
	Share       bool // --share
	Docker      bool // --docker
	NoTreeProxy bool // --no-tree-proxy

	// NoReact skips the React child (and its browser tab).
	NoReact bool

	// ExtraEnv (KEY=VALUE) is added to every child's environment after the
	// inherited one, so it overrides inherited values.
	ExtraEnv []string
	// GradioEnv (KEY=VALUE) is added to the gradio child only. Secrets such as
	// TTS_CORE_TOKEN belong here, not in ExtraEnv.
	GradioEnv []string

	// OpenBrowser opens the public URL once Gradio is ready and then the React
	// URL once React is ready.
	OpenBrowser bool

	// ReadyTimeout bounds how long a child may stay in "starting". 0 = 10 min.
	// A child that misses it is marked failed; it is not killed.
	ReadyTimeout time.Duration

	// SkipGPUCheck disables the "installer_scripts/.gpu must exist and parse"
	// gate that the legacy supervisor enforced before any start.
	SkipGPUCheck bool
	// SystemPython runs python, npm and uv straight from PATH, with no
	// micromamba prefix: a container, Colab or any env the user manages. It
	// implies SkipGPUCheck (there is no installer_scripts/.gpu without the
	// installer) and CommandPrefix stays empty.
	SystemPython bool

	// SkipReactTab opens only the public URL, not the React one, when
	// OpenBrowser is on. The launcher sets it: the React UI is not a separate
	// page for the user.
	SkipReactTab bool

	// Prepare runs at the start of every Start, after the GPU check and before
	// anything is started (switch checks, orphan sweep, React recover and
	// build). Its error aborts the Start. nil = the zero Plan.
	Prepare func(ctx context.Context) (Plan, error)
	// StartGradioSide starts the Go side of the gradio child. prefix is the
	// command prefix (`micromamba -p <env> run`), baseEnv the environment of
	// every child. nil = none.
	StartGradioSide func(prefix, baseEnv []string) (*GradioSide, error)
	// OnStarted is called with the child processes after a successful Start
	// (the launcher records them for the orphan sweep).
	OnStarted func([]ProcRef)
	// OnStopped is called after every Stop, even when nothing ran, and after
	// a failed Start has stopped what it started.
	OnStopped func()

	// CommandPrefix replaces `micromamba -p installer_files/env run`. The first
	// element is the executable; the rest precede `python server.py ...`,
	// `npm ...` etc. Tests point it at a fake child.
	CommandPrefix []string
	// Opener opens a URL in a browser. nil = the system default browser.
	Opener func(url string) error
	// ProbeInterval is the readiness polling period. 0 = 250 ms.
	ProbeInterval time.Duration
}

// normalized returns a copy of c with defaults applied and slices copied, or
// an error when the configuration cannot work.
func (c Config) normalized() (Config, error) {
	if c.Root.Dir() == "" {
		return Config{}, errors.New("supervise: Config.Root is required")
	}
	if c.PublicPort == 0 {
		c.PublicPort = DefaultPublicPort
	}
	if c.ReactPort == 0 {
		c.ReactPort = DefaultReactPort
	}
	for _, p := range []struct {
		name string
		port int
	}{{"PublicPort", c.PublicPort}, {"ReactPort", c.ReactPort}} {
		if p.port < 0 || p.port > 65535 {
			return Config{}, fmt.Errorf("supervise: %s %d out of range", p.name, p.port)
		}
	}
	if !c.NoReact && c.ReactPort == c.PublicPort {
		return Config{}, fmt.Errorf("supervise: ReactPort and PublicPort are both %d", c.PublicPort)
	}
	if c.GradioEntry == "" {
		c.GradioEntry = DefaultGradioEntry
	}
	if c.GradioArgs == nil {
		c.GradioArgs = DefaultGradioArgs()
	} else {
		c.GradioArgs = append([]string(nil), c.GradioArgs...)
	}
	if c.ReadyTimeout <= 0 {
		c.ReadyTimeout = DefaultReadyTimeout
	}
	if c.ProbeInterval <= 0 {
		c.ProbeInterval = DefaultProbeInterval
	}
	if c.SystemPython {
		c.CommandPrefix, c.SkipGPUCheck = nil, true
	} else if len(c.CommandPrefix) == 0 {
		c.CommandPrefix = micromambaPrefix(c.Root)
	} else {
		c.CommandPrefix = append([]string(nil), c.CommandPrefix...)
	}
	if c.Opener == nil {
		c.Opener = OpenURL
	}
	c.ExtraEnv = append([]string(nil), c.ExtraEnv...)
	c.GradioEnv = append([]string(nil), c.GradioEnv...)
	return c, nil
}

// gradioArgs returns the arguments after the entry script: GradioArgs plus the
// forwarded flags that are not already present.
func (c Config) gradioArgs() []string {
	out := append([]string(nil), c.GradioArgs...)
	add := func(on bool, flag string) {
		if !on {
			return
		}
		for _, a := range out {
			if a == flag {
				return
			}
		}
		out = append(out, flag)
	}
	add(c.Share, "--share")
	add(c.Docker, "--docker")
	add(c.NoTreeProxy, "--no-tree-proxy")
	return out
}

// micromambaPath is the platform-specific micromamba binary path. It mirrors
// the launcher's unexported helper (init_mamba.sh writes mamba/micromamba with
// no bin/ subdirectory).
func micromambaPath(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "mamba", "micromamba.exe")
	}
	return filepath.Join(root.InstallerFiles(), "mamba", "micromamba")
}

// installEnvPath is the conda env micromamba runs in.
func installEnvPath(root paths.Root) string {
	return filepath.Join(root.InstallerFiles(), "env")
}

// micromambaPrefix is `micromamba -p installer_files/env run`.
func micromambaPrefix(root paths.Root) []string {
	return []string{micromambaPath(root), "-p", installEnvPath(root), "run"}
}

// mambaRootEnv pins MAMBA_ROOT_PREFIX to the install's mamba directory so
// micromamba does not fall back to ~/.micromamba.
func mambaRootEnv(root paths.Root) string {
	return "MAMBA_ROOT_PREFIX=" + filepath.Join(root.InstallerFiles(), "mamba")
}

// baseEnv is the environment shared by all children.
func (c Config) baseEnv() []string {
	env := append(os.Environ(), mambaRootEnv(c.Root))
	return append(env, c.ExtraEnv...)
}

// CommandPrefix is the command that runs a program in the main env
// (`micromamba -p <root>/installer_files/env run`), for callers that run
// pip or uv while the app is stopped.
func CommandPrefix(root paths.Root) []string { return micromambaPrefix(root) }

// MambaRootEnv is MAMBA_ROOT_PREFIX=<root>/installer_files/mamba, which
// micromamba needs to find its own state.
func MambaRootEnv(root paths.Root) string { return mambaRootEnv(root) }
