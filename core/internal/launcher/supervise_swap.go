package launcher

// The launcher supervises the app with serverpy/supervise by default. The
// older Supervisor (supervisor.go) stays reachable with TTS_GO_SUPERVISOR=legacy
// as a rollback for one release.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

// EnvSupervisor picks the app supervisor: unset, "" or "supervise" runs
// serverpy/supervise; "legacy" runs the old launcher.Supervisor.
const EnvSupervisor = "TTS_GO_SUPERVISOR"

// noopBrowser makes Python's webbrowser.open fail quietly in the gradio
// child: with supervise the launcher opens the tab, so server.py must not.
const noopBrowser = "tts-go-noop-browser"

// appProc names one app process, for the orphan record.
type appProc struct {
	Name string
	PID  int
}

// appSupervisor is what the launcher needs from either supervisor.
type appSupervisor interface {
	Start(ctx context.Context) error
	Stop()
	Restart(ctx context.Context) error
	Statuses() any // JSON-encodable list of per-process statuses
	SubscribeStatus() (<-chan string, func())
	AppLink() string
	Running() bool // any app process is running
}

// legacySup adapts the old Supervisor.
type legacySup struct{ *Supervisor }

func (l legacySup) Statuses() any { return l.Supervisor.Statuses() }
func (l legacySup) Running() bool {
	for _, st := range l.Supervisor.Statuses() {
		if st.Running {
			return true
		}
	}
	return false
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

// badSup reports a bad switch value on every Start.
type badSup struct {
	appSupervisor
	err error
}

func (b badSup) Start(context.Context) error   { return b.err }
func (b badSup) Restart(context.Context) error { return b.err }

// Option configures New.
type Option func(*options)

type options struct{ noBrowser, share, docker bool }

// WithNoBrowser stops the supervisor from opening the app in a browser.
func WithNoBrowser() Option { return func(o *options) { o.noBrowser = true } }

// WithShare is server.py's --share: a public Gradio link.
func WithShare() Option { return func(o *options) { o.share = true } }

// WithDocker is server.py's --docker: the router binds 0.0.0.0:7770.
func WithDocker() Option { return func(o *options) { o.docker = true } }

func buildOptions(opts []Option) options {
	var o options
	for _, f := range opts {
		f(&o)
	}
	return o
}

// flags are the server.py switches the launcher passes on: it runs React
// itself (--no-react), and --share and --docker are the user's.
func (o options) flags() appenv.Flags {
	return appenv.Flags{NoReact: true, Share: o.share, Docker: o.docker}
}

func newAppSupervisor(root paths.Root, opts ...Option) appSupervisor {
	o := buildOptions(opts)
	v, _ := os.LookupEnv(EnvSupervisor)
	switch strings.TrimSpace(v) {
	case "legacy":
		return legacySup{newSupervisor(root, opts...)}
	case "", "supervise":
	default:
		return badSup{legacySup{newSupervisor(root, opts...)}, fmt.Errorf("%s must be unset, supervise or legacy, got %q", EnvSupervisor, v)}
	}
	sup, err := supervise.New(superviseConfig(root, o))
	if err != nil {
		return badSup{legacySup{newSupervisor(root, opts...)}, err}
	}
	return superviseSup{sup}
}

// superviseConfig wires supervise to the launcher's React preparation, switch
// checks, orphan handling and the TTS_GO_SERVERPY=1 Go side.
func superviseConfig(root paths.Root, o options) supervise.Config {
	var pyOn bool
	cfg := supervise.Config{
		Root:         root,
		SkipReactTab: true,
		GradioArgs:   o.flags().Args(),
	}
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
		if pyOn, err = serverPyEnabled(os.LookupEnv); err != nil {
			return supervise.Plan{}, err
		}
		// A launcher that was killed may have left the app running on Linux
		// and macOS; stop it before ports collide.
		sweepOrphans(root)

		rp, err := (&Supervisor{root: root}).prepareReactUI(ctx, sw)
		if err != nil {
			return supervise.Plan{}, err
		}
		plan := supervise.Plan{Switches: sw, React: supervise.ReactNextDirect}
		if sw.Proxy {
			plan.React = supervise.ReactNextProxied
			if rp.mode == modeStatic {
				plan.React = supervise.ReactStatic
			}
		}
		if pyOn {
			plan.GradioEntry = serverPyEntry
		}
		// The public port and inbrowser come from config.json, as in server.py.
		inBrowser := false
		if c, err := appenv.LoadConfig(root.Dir()); err == nil {
			plan.PublicPort, inBrowser = appenv.PublicPortFor(c.Port, o.flags()), c.InBrowser
		}
		open := inBrowser && !o.noBrowser
		plan.OpenBrowser = &open
		// The launcher opens the tab; server.py must not open a second one.
		plan.GradioEnv = []string{"BROWSER=" + noopBrowser}
		return plan, nil
	}
	cfg.StartGradioSide = func(prefix, baseEnv []string) (*supervise.GradioSide, error) {
		if !pyOn {
			return nil, nil
		}
		py, err := startServerPy(root, prefix, baseEnv, o.flags())
		if err != nil {
			return nil, fmt.Errorf("%s=1: %w", EnvServerPy, err)
		}
		return &supervise.GradioSide{Env: py.gradioEnv, Close: py.Close}, nil
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
