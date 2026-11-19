package installer_scripts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// This file ports installer_scripts/js/shell.js, the command helpers that
// init_app.js and js/initializeApp.js use. Every command runs inside the
// installer_files/env environment, because the launcher starts init_app.js
// with `micromamba -p installer_files/env run node`.
//
// The promises of shell.js become errors. Their messages are the ones the
// promises reject with, because the flow logs them.

// jsProcess is what the Node process running init_app.js shares between its
// modules: the step, and the commands it started without awaiting them.
type jsProcess struct {
	s *Step

	// newPipPackagesVersion is String(versions.json .pip_packages), read when
	// init_app.js loads js/initializeApp.js.
	newPipPackagesVersion string

	// flowCtx is the context of the awaited flow. A failing un-awaited $sh
	// cancels it, see background.
	flowCtx    context.Context
	flowCancel context.CancelFunc

	// Un-awaited commands run on bgCtx. They are stopped when the flow ends,
	// because process.exit ends the original.
	bgCtx    context.Context
	bgCancel context.CancelFunc
	bgWG     sync.WaitGroup

	mu    sync.Mutex
	crash error
}

func newJSProcess(ctx context.Context, s *Step) *jsProcess {
	p := &jsProcess{s: s}
	p.flowCtx, p.flowCancel = context.WithCancel(ctx)
	p.bgCtx, p.bgCancel = context.WithCancel(ctx)
	return p
}

// mustRethrow reports whether a catch block of the original never sees err:
// process.exit ends Node before any catch runs, and a cancelled context means
// the port stops.
func mustRethrow(ctx context.Context, err error) bool {
	var exit processExitError
	return ctx.Err() != nil || errors.As(err, &exit)
}

// commandFailed is the error runCommand and $sh reject with on a non-zero exit.
func commandFailed(code int) error {
	return fmt.Errorf("Command failed with exit code %d", code)
}

// dollar ports $: log ">command", split it on spaces and run it without a shell.
func (p *jsProcess) dollar(ctx context.Context, command string) error {
	p.consoleLog(">" + command)
	parts := strings.Split(command, " ")
	return p.runCommand(ctx, parts[0], parts[1:])
}

// dollarArgs ports $$: like $, with the arguments already split.
func (p *jsProcess) dollarArgs(ctx context.Context, command []string) error {
	p.consoleLog(">" + strings.Join(command, " "))
	return p.runCommand(ctx, command[0], command[1:])
}

// runCommand ports runCommand: spawn and wait for the process to exit.
func (p *jsProcess) runCommand(ctx context.Context, name string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	res, err := p.s.Run.Run(ctx, Command{Name: name, Args: args, Activated: true})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The child's "error" event, such as a program that is not found.
		return fmt.Errorf("Error: %v", err)
	}
	if res.ExitCode != 0 {
		return commandFailed(res.ExitCode)
	}
	return nil
}

// sh ports an awaited $sh: run a line through the shell and wait for its
// output to close.
func (p *jsProcess) sh(ctx context.Context, cmd string) error {
	p.consoleLog(">[async] " + cmd)
	return p.shRun(ctx, cmd)
}

func (p *jsProcess) shRun(ctx context.Context, cmd string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	res, err := p.s.Run.Run(ctx, Command{Shell: cmd, Activated: true})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if res.ExitCode != 0 {
		return commandFailed(res.ExitCode)
	}
	return nil
}

// shBackground ports a $sh whose promise is never awaited or caught. The
// command line is logged at once and the flow carries on. If the command fails
// while the original would still be running, Node 22 treats the rejection as
// unhandled and ends the process with exit code 1; background records that.
func (p *jsProcess) shBackground(cmd string) {
	p.consoleLog(">[async] " + cmd)
	p.background(func(ctx context.Context) error { return p.shRun(ctx, cmd) }, true)
}

// background runs fn without waiting for it. When unhandled is true, a failure
// before the flow ends stops the flow and makes the exit code 1.
func (p *jsProcess) background(fn func(ctx context.Context) error, unhandled bool) {
	p.bgWG.Add(1)
	go func() {
		defer p.bgWG.Done()
		err := fn(p.bgCtx)
		if err == nil || !unhandled || p.bgCtx.Err() != nil {
			return
		}
		p.mu.Lock()
		if p.crash == nil {
			p.crash = err
		}
		p.mu.Unlock()
		p.flowCancel()
	}()
}

// crashed returns the failure of an un-awaited $sh, if one ended the process.
func (p *jsProcess) crashed() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.crash
}

// stopBackground lets un-awaited commands run for up to grace, then stops the
// ones still running. The original leaves them behind when Node exits; the port
// leaves nothing running.
func (p *jsProcess) stopBackground(grace time.Duration) {
	done := make(chan struct{})
	go func() {
		p.bgWG.Wait()
		close(done)
	}()
	if grace > 0 {
		select {
		case <-done:
		case <-time.After(grace):
		}
	}
	p.bgCancel()
	<-done
}

// execCapture ports execCapture: run a line through the shell and return its
// stdout.
func (p *jsProcess) execCapture(ctx context.Context, cmd string) (string, error) {
	p.consoleLog(">[capture] " + cmd)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	res, err := p.s.Run.Run(ctx, Command{Shell: cmd, Activated: true, Capture: true})
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("Command failed: %v", err)
	}
	if res.ExitCode != 0 {
		// exec's own message is "Command failed: <cmd>", wrapped once more.
		return "", fmt.Errorf("Command failed: Command failed: %s", cmd)
	}
	return res.Stdout, nil
}
