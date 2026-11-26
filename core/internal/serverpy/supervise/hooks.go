package supervise

import (
	"context"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

// Plan is what Config.Prepare decides for one Start. Zero fields keep the
// Config values.
type Plan struct {
	// Switches are the TTS_GO_* switches; they feed the React front.
	Switches switches.Set
	// PublicPort overrides Config.PublicPort for this run (the port from
	// config.json, which the Go router or the proxy tree binds).
	PublicPort int
	// ReactPort overrides Config.ReactPort for this run (REACT_UI_PORT in the
	// launcher's environment). 0 keeps the Config value (3000).
	ReactPort int
	// PublicTLS overrides Config.PublicTLS for this run (config.json names a
	// certificate and key, and the router terminates TLS).
	PublicTLS bool
	// GradioEntry overrides Config.GradioEntry for this run.
	GradioEntry string
	// GradioEnv is added to the gradio child's environment after
	// Config.GradioEnv.
	GradioEnv []string
	// OpenBrowser overrides Config.OpenBrowser for this run.
	OpenBrowser *bool
}

// GradioSide is the Go side of the gradio child, started before it (the
// server.py port: router and extension processes).
type GradioSide struct {
	// Env is added to the gradio child's environment only.
	Env []string
	// Close ends the Go side. It runs when gradio exits and on Stop, at most
	// once, and also when gradio could not be started.
	Close func()
	// CoreURL and CoreToken are the control API the React front calls.
	CoreURL, CoreToken string
}

// ProcRef names a running child process, for the orphan record.
type ProcRef struct {
	Name string
	PID  int
}

// extraRunner is the part of a runner the wrappers need beyond the interface.
type pidder interface{ Pid() int }

// closingRunner runs closers when its process is done, when it is stopped
// and when it fails to start. The closers run at most once.
type closingRunner struct {
	runner
	once    sync.Once
	closers []func()
}

func (c *closingRunner) closeAll() {
	c.once.Do(func() {
		var wg sync.WaitGroup
		for _, f := range c.closers {
			if f == nil {
				continue
			}
			wg.Add(1)
			go func(f func()) { defer wg.Done(); f() }(f)
		}
		wg.Wait()
	})
}

func (c *closingRunner) Start(ctx context.Context, name string, args []string, cwd string, env []string) error {
	if err := c.runner.Start(ctx, name, args, cwd, env); err != nil {
		c.closeAll()
		return err
	}
	go func() { <-c.runner.Done(); c.closeAll() }()
	return nil
}

// Stop closes the extras while the process stops, so a slow closer does not
// delay the process and the other way round.
func (c *closingRunner) Stop() error {
	done := make(chan struct{})
	go func() { c.closeAll(); close(done) }()
	err := c.runner.Stop()
	<-done
	return err
}

// Subscribe passes the process's live output through the wrapper, which is what
// SubscribeLogs asks for. A runner that cannot stream gives an already closed
// channel.
func (c *closingRunner) Subscribe() (<-chan string, func()) {
	if s, ok := c.runner.(interface {
		Subscribe() (<-chan string, func())
	}); ok {
		return s.Subscribe()
	}
	ch := make(chan string)
	close(ch)
	return ch, func() {}
}

func (c *closingRunner) Pid() int {
	if p, ok := c.runner.(pidder); ok {
		return p.Pid()
	}
	return 0
}

// frontRunner stands in for the React child: Go serves the static export, so no
// process runs. It is alive until stopped, and stopping closes the front.
type frontRunner struct {
	closeFront func()
	once       sync.Once
	done       chan struct{}
}

func newFrontRunner(closeFront func()) *frontRunner {
	return &frontRunner{closeFront: closeFront, done: make(chan struct{})}
}

func (f *frontRunner) Start(context.Context, string, []string, string, []string) error {
	return nil
}
func (f *frontRunner) Stop() error {
	f.once.Do(func() { f.closeFront(); close(f.done) })
	return nil
}
func (f *frontRunner) Done() <-chan struct{} { return f.done }
func (f *frontRunner) ExitCode() int         { return 0 }
func (f *frontRunner) Lines() []string       { return nil }
