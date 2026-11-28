package launcher

// The launcher supervises the app with serverpy/supervise: the Go router on the
// public port, the Python Gradio child (server_core.py), the React front and the
// extension processes the app asks for.

import (
	"context"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"os"
	"strconv"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

// reactPortFromEnv is the Vite UI's port: REACT_UI_PORT in the launcher's
// environment, else 0 (supervise then uses its default, 3000).
func reactPortFromEnv() (int, error) {
	v := strings.TrimSpace(os.Getenv("REACT_UI_PORT"))
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("REACT_UI_PORT=%q is not a port", v)
	}
	return n, nil
}

// noopBrowser makes Python's webbrowser.open fail quietly in the gradio
// child: the launcher opens the tab, so Gradio must not.
const noopBrowser = "tts-go-noop-browser"

// appProc names one app process, for the orphan record.
type appProc struct {
	Name string
	PID  int
}

// appSupervisor is what the launcher needs from the app supervisor.
type appSupervisor interface {
	Start(ctx context.Context) error
	// StartParts and StopPart act on one part (gradio, react-ui) and leave the other as it is.
	StartParts(ctx context.Context, parts ...string) error
	StopPart(name string)
	Stop()
	Restart(ctx context.Context) error
	Statuses() any // JSON-encodable list of per-process statuses
	SubscribeStatus() (<-chan string, func())
	AppLink() string
	Running() bool // any app process is running
	// SubscribeLogs streams a child's live output; ok is false when it is not running.
	SubscribeLogs(name string) (lines <-chan string, cancel func(), ok bool)
}

// superviseSup adapts supervise.Supervisor.
type superviseSup struct{ *supervise.Supervisor }

func (s superviseSup) Statuses() any { return s.Supervisor.Statuses() }
func (s superviseSup) Running() bool {
	for _, st := range s.Supervisor.Statuses() {
		if st.Running {
			return true
		}
	}
	return false
}

// badSup stands in when the supervisor cannot be built: every Start says why.
type badSup struct{ err error }

func (b badSup) Start(context.Context) error                 { return b.err }
func (b badSup) StartParts(context.Context, ...string) error { return b.err }
func (b badSup) StopPart(string)                             {}
func (b badSup) Restart(context.Context) error               { return b.err }
func (b badSup) Stop()                                       {}
func (b badSup) Statuses() any                               { return []supervise.ProcessStatus{} }
func (b badSup) AppLink() string                             { return "" }
func (b badSup) Running() bool                               { return false }
func (b badSup) SubscribeLogs(string) (<-chan string, func(), bool) {
	return nil, nil, false
}
func (b badSup) SubscribeStatus() (<-chan string, func()) {
	ch := make(chan string)
	close(ch)
	return ch, func() {}
}

// Option configures New.
type Option func(*options)

type options struct {
	noBrowser, share, docker, systemPython, noReact bool
	launcherPort                                    int
	frontDevURL                                     string // a Vite dev server answering the React UI's pages (WithDev)
}

// WithDev has the React UI server forward the app's pages and assets to a Vite dev
// server (hot reload), such as http://127.0.0.1:3002 (npm run dev in frontend/).
// The API routes, uploads and the export are the server's as ever; frontend/out
// is not needed and not built.
func WithDev(devURL string) Option { return func(o *options) { o.frontDevURL = devURL } }

// WithLauncherPort tells the app where this launcher listens, so the Vite UI can
// link to the launcher's Extensions page (the Gradio host is installed there).
func WithLauncherPort(port int) Option { return func(o *options) { o.launcherPort = port } }

// WithNoBrowser stops the supervisor from opening the app in a browser.
func WithNoBrowser() Option { return func(o *options) { o.noBrowser = true } }

// WithShare is Gradio's share option: a public Gradio link.
func WithShare() Option { return func(o *options) { o.share = true } }

// WithSystemPython runs python, npm and uv from PATH instead of the installer's
// legacy-env (Docker, Colab, an env the user manages: the system platform). The GPU gate is off.
func WithSystemPython() Option { return func(o *options) { o.systemPython = true } }

// WithNoReact leaves the React UI out: no build and no front.
func WithNoReact() Option { return func(o *options) { o.noReact = true } }

// WithDocker makes the router bind 0.0.0.0:7770.
func WithDocker() Option { return func(o *options) { o.docker = true } }

func buildOptions(opts []Option) options {
	var o options
	for _, f := range opts {
		f(&o)
	}
	return o
}

// gradioLegacyPrefix is where Gradio lives on the public port when the React UI
// is served from the router at "/".
const gradioLegacyPrefix = "/gradio-legacy"

// frontInRouter says whether the React UI is served by the router on the public
// port (and Gradio sits under gradioLegacyPrefix). It is the layout of a normal
// run; TTS_FRONT_IN_ROUTER=0 brings back the old one (React on its own port,
// Gradio at "/").
func (o options) frontInRouter() bool {
	return !o.noReact && os.Getenv("TTS_FRONT_IN_ROUTER") != "0"
}

// flags are the switches the launcher passes on: it runs React itself
// (NoReact for the gradio child), and share and docker are the user's.
func (o options) flags() appenv.Flags {
	return appenv.Flags{NoReact: true, Share: o.share, Docker: o.docker}
}

func newAppSupervisor(root paths.Root, opts ...Option) appSupervisor {
	sup, err := supervise.New(superviseConfig(root, buildOptions(opts)))
	if err != nil {
		return badSup{err}
	}
	return superviseSup{sup}
}

// superviseConfig wires supervise to the launcher's React preparation, switch
// checks, orphan handling and the Go side of the app (router, control API).
func superviseConfig(root paths.Root, o options) supervise.Config {
	cfg := supervise.Config{
		Root:         root,
		SkipReactTab: true,
		SystemPython: o.systemPython,
		NoReact:      o.noReact,
		GradioEntry:  serverPyEntry,
		GradioArgs:   o.flags().Args(),
	}
	if o.frontInRouter() {
		cfg.FrontInRouter = true
		cfg.GradioPrefix = gradioLegacyPrefix
	}
	cfg.FrontDevURL = o.frontDevURL
	cfg.Prepare = func(ctx context.Context) (supervise.Plan, error) {
		// Every switch is checked before anything runs, so a typo never starts
		// a build or a process first (R2-5).
		sw, err := switches.FromEnv()
		if err != nil {
			return supervise.Plan{}, err
		}
		if err := sw.CheckRoutes(react_ui_backend.ClaimNames()); err != nil {
			return supervise.Plan{}, err
		}
		if err := sw.CheckSteps(knownGoSteps); err != nil {
			return supervise.Plan{}, err
		}
		// A launcher that was killed may have left the app running on Linux
		// and macOS; stop it before ports collide.
		sweepOrphans(root)

		plan := supervise.Plan{Switches: sw}
		// The public port and inbrowser come from config.json.
		inBrowser := false
		if c, err := appenv.LoadConfig(root.Dir()); err == nil {
			plan.PublicPort, inBrowser = appenv.PublicPortFor(c.Port, o.flags()), c.InBrowser
			plan.PublicTLS = c.SSL.Enabled()
		}
		// REACT_UI_PORT in the launcher's environment moves the Vite UI off 3000
		// (two launchers on one machine, or something else holding 3000).
		n, err := reactPortFromEnv()
		if err != nil {
			return supervise.Plan{}, err
		}
		plan.ReactPort = n
		open := inBrowser && !o.noBrowser
		plan.OpenBrowser = &open
		// The launcher opens the tab; Gradio must not open a second one.
		// Unbuffered, so the output a headless start copies to stdout (Docker
		// logs, a Colab cell) and the status log tail arrive as it is printed.
		plan.GradioEnv = []string{"BROWSER=" + noopBrowser, "PYTHONUNBUFFERED=1"}
		return plan, nil
	}
	// The static export is built when the React UI front starts, so Gradio can
	// be launched alone without building it.
	cfg.PrepareFront = func(ctx context.Context, plan supervise.Plan) error {
		if o.noReact || o.frontDevURL != "" {
			return nil // development: Vite answers the pages, no export to build
		}
		return newReactPrep(root, o.systemPython).prepareReactUI(ctx, plan.Switches)
	}
	cfg.StartGradioSide = func(p platform.Platform) (*supervise.GradioSide, error) {
		var sopts []serverPyOpt
		if o.frontInRouter() {
			sopts = append(sopts, withGradioPrefix(gradioLegacyPrefix))
		}
		py, err := startServerPy(root, p, o.flags(), sopts...)
		if err != nil {
			return nil, err
		}
		side := &supervise.GradioSide{Env: py.gradioEnv, Close: py.Close, CoreURL: py.coreURL, CoreToken: py.coreToken}
		if o.frontInRouter() {
			side.Router = py.rt
		}
		return side, nil
	}
	cfg.OnStarted = func(refs []supervise.ProcRef) {
		procs := make([]appProc, len(refs))
		for i, r := range refs {
			procs[i] = appProc{Name: r.Name, PID: r.PID}
		}
		recordGroups(root, procs)
	}
	cfg.OnStopped = func() { clearRecordFor(root) }
	return cfg
}
