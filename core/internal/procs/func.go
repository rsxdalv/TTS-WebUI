package procs

import (
	"context"
	"fmt"
	"sync/atomic"
)

// funcJob is the state of a job started with StartFunc.
type funcJob struct {
	cancel context.CancelFunc
	code   atomic.Int64 // exit code once done; -1 before
	ended  atomic.Bool
}

func (j *funcJob) exitCode() int {
	if !j.ended.Load() {
		return -1
	}
	return int(j.code.Load())
}

// StartFunc runs fn in this process and presents it like a child process: its
// log lines go to the ring and the subscribers, Running, Done and ExitCode
// work, and Stop cancels the context fn receives. The launcher uses it for
// install steps that run a Go port instead of a script (TTS_GO_STEPS).
//
// fn returns the exit code. A panic in fn is logged as a line and counts as
// exit code 1, so one step cannot take the launcher down.
func (p *Proc) StartFunc(ctx context.Context, fn func(ctx context.Context, log func(line string)) int) error {
	p.mu.Lock()
	if p.done != nil {
		p.mu.Unlock()
		return fmt.Errorf("proc already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	job := &funcJob{cancel: cancel}
	job.code.Store(-1)
	p.fn = job
	p.done = make(chan struct{})
	done := p.done
	p.mu.Unlock()

	go func() {
		code := 1
		defer func() {
			if r := recover(); r != nil {
				p.addLine(fmt.Sprintf("panic: %v", r))
				code = 1
			}
			job.code.Store(int64(code))
			job.ended.Store(true)
			cancel()
			close(done)
		}()
		code = fn(ctx, p.addLine)
	}()
	return nil
}

// Note adds a line to the log without running anything. The launcher uses it to
// say which implementation a step ran.
func (p *Proc) Note(line string) { p.addLine(line) }
