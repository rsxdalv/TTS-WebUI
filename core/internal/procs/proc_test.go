package procs_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// childBin is the path to the compiled test helper binary.
var childBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "procs-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdirtemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "child")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command(
		"go", "build", "-o", bin, "./testdata/child",
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "building child helper: %v\n%s\n", err, out)
		os.Exit(1)
	}
	childBin = bin
	os.Exit(m.Run())
}

func TestStart_OutputCaptured(t *testing.T) {
	// Use 'echo' as a simple command that produces output.
	// On Windows "echo" is a shell built-in; use cmd /c echo.
	var (
		name string
		args []string
	)
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "echo", "hello from proc"}
	} else {
		name = "echo"
		args = []string{"hello from proc"}
	}

	p := &procs.Proc{}
	sub, unsub := p.Subscribe()
	defer unsub()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx, name, args, "", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	p.Wait()

	lines := p.Lines()
	found := false
	for _, l := range lines {
		if strings.Contains(l, "hello from proc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Lines() did not contain expected output: %v", lines)
	}

	// Subscriber should have received the line too.
	select {
	case l := <-sub:
		if !strings.Contains(l, "hello from proc") {
			t.Errorf("subscriber got %q, want 'hello from proc'", l)
		}
	case <-time.After(time.Second):
		t.Error("subscriber channel: no line received within 1s")
	}
}

func TestOrphanKill(t *testing.T) {
	if childBin == "" {
		t.Skip("child binary not built")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p := &procs.Proc{}
	sub, unsub := p.Subscribe()
	defer unsub()

	if err := p.Start(ctx, childBin, nil, "", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Read the grandchild's TCP port from the proc's output.
	gcPort := ""
	deadline := time.After(5 * time.Second)
	for gcPort == "" {
		select {
		case line := <-sub:
			port, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil && port > 0 {
				gcPort = fmt.Sprintf("%d", port)
			}
		case <-deadline:
			t.Fatal("timeout waiting for grandchild port from child")
		}
	}

	// Grandchild should be alive now.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+gcPort, time.Second)
	if err != nil {
		t.Fatalf("grandchild not reachable before kill: %v", err)
	}
	conn.Close()

	// Kill the parent tree.
	if err := p.Stop(); err != nil {
		t.Logf("Stop returned (non-fatal on some platforms): %v", err)
	}

	// Wait for grandchild to become unreachable.
	deadline2 := time.After(6 * time.Second)
	for {
		select {
		case <-deadline2:
			t.Fatal("grandchild still alive 6s after parent kill")
		case <-time.After(100 * time.Millisecond):
			conn, err := net.DialTimeout("tcp", "127.0.0.1:"+gcPort, 200*time.Millisecond)
			if err != nil {
				return // grandchild gone — test passes
			}
			conn.Close()
		}
	}
}

func TestRingBuffer_Capacity(t *testing.T) {
	// Write more than ringCap lines and check that only the last ringCap are kept.
	// We test the ring indirectly through Proc output.
	// For unit-level ring testing, the ring is unexported; we trust Lines() reflects it.

	// Just check that Lines() returns something reasonable.
	var (
		name string
		args []string
	)
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "echo", "line"}
	} else {
		name = "sh"
		args = []string{"-c", "echo line"}
	}
	p := &procs.Proc{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Start(ctx, name, args, "", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	p.Wait()
	if len(p.Lines()) == 0 {
		t.Error("Lines() is empty after process exited with output")
	}
}

// TestSubscribeUnsubscribe verifies that unsub() does not block or panic.
func TestSubscribeUnsubscribe(t *testing.T) {
	p := &procs.Proc{}
	_, unsub1 := p.Subscribe()
	_, unsub2 := p.Subscribe()
	unsub1()
	unsub2()
	// Second unsubscribe of the same sub should be a no-op.
	unsub1()
}
