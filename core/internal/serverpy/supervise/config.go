package supervise

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/detect"
)

// Defaults applied by New for zero Config fields.
const (
	DefaultPublicPort    = 7770
	DefaultReactPort     = 3000
	DefaultGradioEntry   = "server_core.py"
	DefaultReadyTimeout  = 10 * time.Minute
	DefaultProbeInterval = 250 * time.Millisecond
)

// frontPort is the port the React UI answers on: the public port when the router
// serves it, ReactPort otherwise.
func (c Config) frontPort() int {
	if c.FrontInRouter {
		return c.PublicPort
	}
	return c.ReactPort
}

// DefaultGradioArgs returns the default arguments passed to the gradio entry
// script. The React UI is started by the supervisor, never by server.py itself.
func DefaultGradioArgs() []string { return []string{"--no-react"} }

// Config describes the app process group. The zero value is not usable: Root
// is required. All other fields have defaults (see New).
type Config struct {
	// Root is the install root (cwd of every child).
	Root paths.Root

	// PublicPort is the port the user-facing Gradio entry answers on (the proxy
	// tree's port today). It is used for readiness, AppLink, the browser URL
	// and the GRADIO_BACKEND_AUTOMATIC value handed to React. 0 = 7770.
	PublicPort int
	// PublicTLS is set when the router serves the public port over TLS (the
	// readiness probe and the app link then use https). Plan.PublicTLS sets it
	// for one run.
	PublicTLS bool
	// ReactPort is the port the Go front serves the React UI on. 0 = 3000.
	ReactPort int
	// FrontInRouter serves the React UI from the router on the public port (at
	// "/") instead of a port of its own, when the Go side offers a router
	// (GradioSide.Router). The status of the React part then reports the public
	// port. Without a router the front keeps ReactPort.
	FrontInRouter bool
	// GradioPrefix is where Gradio lives on the public port: "" is "/", otherwise
	// a prefix such as "/gradio-legacy". It is where readiness is probed and the
	// app link and the browser tab point.
	GradioPrefix string

	// GradioEntry is the Python script run for Gradio, relative to Root.
	// "" = server_core.py.
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

	// SystemPython selects the system platform: python, npm and uv straight
	// from PATH, for a container, Colab or any env the user manages. It
	// replaces Platform.
	SystemPython bool

	// SkipReactTab opens only the public URL, not the React one, when
	// OpenBrowser is on. The launcher sets it: the React UI is not a separate
	// page for the user.
	SkipReactTab bool

	// Prepare runs at the start of every Start, after the GPU check and before
	// anything is started (switch checks, orphan sweep, React recover and
	// build). Its error aborts the Start. nil = the zero Plan.
	Prepare func(ctx context.Context) (Plan, error)
	// PrepareFront runs before the React UI front starts, in a Start and in a
	// later start of that part alone (it builds the static export). Its error
	// aborts that start. nil = nothing to prepare.
	PrepareFront func(ctx context.Context, plan Plan) error
	// StartGradioSide starts the Go side of the gradio child, on the platform the
	// children run on. nil = none.
	StartGradioSide func(p platform.Platform) (*GradioSide, error)
	// OnStarted is called with the child processes after a successful Start
	// (the launcher records them for the orphan sweep).
	OnStarted func([]ProcRef)
	// OnStopped is called after every Stop, even when nothing ran, and after
	// a failed Start has stopped what it started.
	OnStopped func()

	// Platform is the runtime the children run on. nil = the platform of the
	// install folder (detect.Detect), with ExtraEnv added to its commands.
	Platform platform.Platform
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
	if c.Platform == nil || c.SystemPython {
		c.Platform = detect.Detect(detect.Options{Root: c.Root, SystemPython: c.SystemPython, Env: c.ExtraEnv})
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

// baseEnv is the environment shared by all children: the platform's, then
// ExtraEnv (again, so it wins even for a platform built without it).
func (c Config) baseEnv() []string {
	cmd, err := c.Platform.Command(nil, []string{"python"})
	if err != nil {
		return append([]string(nil), c.ExtraEnv...)
	}
	return append(cmd.Env, c.ExtraEnv...)
}
