package procs

import (
	"context"
	"testing"
	"time"
)

func TestStartFunc_LinesExitCodeAndDone(t *testing.T) {
	var p Proc
	err := p.StartFunc(context.Background(), func(ctx context.Context, log func(string)) int {
		log("one")
		log("two")
		return 7
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Wait()
	if p.Running() || p.ExitCode() != 7 {
		t.Fatalf("running=%v code=%d", p.Running(), p.ExitCode())
	}
	if l := p.Lines(); len(l) != 2 || l[0] != "one" || l[1] != "two" {
		t.Fatalf("lines %q", l)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("Done not closed")
	}
	if err := p.StartFunc(context.Background(), func(context.Context, func(string)) int { return 0 }); err == nil {
		t.Fatal("second start must fail")
	}
}

func TestStartFunc_ExitCodeIsMinusOneWhileRunningAndStopCancels(t *testing.T) {
	var p Proc
	started := make(chan struct{})
	p.StartFunc(context.Background(), func(ctx context.Context, log func(string)) int {
		close(started)
		<-ctx.Done()
		return 9
	})
	<-started
	if !p.Running() || p.ExitCode() != -1 {
		t.Fatalf("running=%v code=%d", p.Running(), p.ExitCode())
	}
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not cancel the job")
	}
	if p.Running() || p.ExitCode() != 9 {
		t.Fatalf("after stop: running=%v code=%d", p.Running(), p.ExitCode())
	}
}

func TestStartFunc_SubscribersGetLinesAndPanicIsExit1(t *testing.T) {
	var p Proc
	release := make(chan struct{})
	p.StartFunc(context.Background(), func(ctx context.Context, log func(string)) int {
		<-release
		log("before panic")
		panic("boom")
	})
	ch, unsub := p.Subscribe()
	defer unsub()
	close(release)
	p.Wait()
	if line := <-ch; line != "before panic" {
		t.Fatalf("got %q", line)
	}
	if p.ExitCode() != 1 {
		t.Fatalf("code %d", p.ExitCode())
	}
	lines := p.Lines()
	if len(lines) != 2 || lines[1] != "panic: boom" {
		t.Fatalf("lines %q", lines)
	}
}
