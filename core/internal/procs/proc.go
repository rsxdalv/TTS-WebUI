// Package procs spawns and supervises child processes, capturing their output
// in a ring buffer and ensuring no orphans survive the supervisor's exit.
package procs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

const subBufSize = 256 // per-subscriber channel buffer

// subscriber is a fan-out recipient for process output lines.
type subscriber struct {
	ch chan string
}

// Proc manages a single supervised child process.
type Proc struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	ps   *platformState
	ring ring
	subs []*subscriber
	done chan struct{}
}

// Start spawns name with args, working directory cwd (empty = inherit), and
// additional environment variables env (nil = inherit).
// Returns an error if the process cannot be started.
func (p *Proc) Start(ctx context.Context, name string, args []string, cwd string, env []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done != nil {
		return fmt.Errorf("proc already started")
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if env != nil {
		cmd.Env = env
	}

	ps, err := newPlatformState(cmd)
	if err != nil {
		return fmt.Errorf("platform setup: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting process: %w", err)
	}
	if err := ps.assign(cmd); err != nil {
		// Non-fatal: log and continue (process is running; kill falls back to cmd.Process.Kill).
		_ = cmd.Process.Kill()
		return fmt.Errorf("assigning to job/process-group: %w", err)
	}

	p.cmd = cmd
	p.ps = ps
	p.done = make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		p.readLines(stdoutPipe, "")
	}()
	go func() {
		defer wg.Done()
		p.readLines(stderrPipe, "[stderr] ")
	}()
	go func() {
		wg.Wait()
		_ = cmd.Wait()
		close(p.done)
	}()

	return nil
}

// readLines reads lines from r, adding them to the ring and notifying subscribers.
func (p *Proc) readLines(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		p.addLine(prefix + scanner.Text())
	}
}

func (p *Proc) addLine(line string) {
	p.ring.push(line)
	p.mu.Lock()
	subs := make([]*subscriber, len(p.subs))
	copy(subs, p.subs)
	p.mu.Unlock()
	for _, sub := range subs {
		select {
		case sub.ch <- line:
		default: // drop; subscriber is too slow
		}
	}
}

// Stop sends a graceful signal, waits up to 5 seconds, then force-kills.
func (p *Proc) Stop() error {
	p.mu.Lock()
	cmd := p.cmd
	ps := p.ps
	done := p.done
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil || done == nil {
		return nil
	}

	_ = ps.graceful(cmd.Process.Pid)

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return ps.force()
	}
}

// Wait blocks until the process exits.
func (p *Proc) Wait() {
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Lines returns a snapshot of all buffered output lines.
func (p *Proc) Lines() []string { return p.ring.lines() }

// Subscribe returns a channel that receives future output lines and a function
// to cancel the subscription. The channel is buffered; slow readers drop lines.
func (p *Proc) Subscribe() (<-chan string, func()) {
	sub := &subscriber{ch: make(chan string, subBufSize)}
	p.mu.Lock()
	p.subs = append(p.subs, sub)
	p.mu.Unlock()

	unsub := func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		for i, s := range p.subs {
			if s == sub {
				p.subs = append(p.subs[:i], p.subs[i+1:]...)
				close(sub.ch)
				return
			}
		}
	}
	return sub.ch, unsub
}
