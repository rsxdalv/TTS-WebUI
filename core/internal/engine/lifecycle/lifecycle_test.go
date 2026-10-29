package lifecycle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// fakeEngineBin is the path to the compiled fake_engine binary; set in TestMain.
var fakeEngineBin string

func TestMain(m *testing.M) {
	// Build the fake engine binary once for all tests.
	bin, err := buildFakeEngine()
	if err != nil {
		// If go build fails (e.g. no toolchain), skip engine tests gracefully.
		println("SKIP: could not build fake_engine:", err.Error())
		os.Exit(0)
	}
	fakeEngineBin = bin
	os.Exit(m.Run())
}

func buildFakeEngine() (string, error) {
	dir, err := os.MkdirTemp("", "fake-engine-*")
	if err != nil {
		return "", err
	}

	out := filepath.Join(dir, "fake_engine")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	src := filepath.Join("testdata", "fake_engine")
	cmd := exec.Command("go", "build", "-o", out, "./"+src)
	cmd.Dir, _ = filepath.Abs(".")
	if data, err := cmd.CombinedOutput(); err != nil {
		return "", err
	} else {
		_ = data
	}
	return out, nil
}

// testManifest returns a manifest that launches the fake engine with --port.
func testManifest(id string) manifest.Manifest {
	return manifest.Manifest{
		PackageName: id,
		Name:        id,
		Engine: manifest.Engine{
			Kind: manifest.KindPythonVenv,
			Launch: &manifest.Launch{
				Cmd:     []string{fakeEngineBin},
				PortArg: "--port",
			},
			Health: &manifest.Health{
				Path:      "/health",
				IntervalS: 1,
				TimeoutS:  10,
			},
		},
	}
}

func TestStartStop(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-start")
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-start"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := mgr.Status("test-engine-start")
	if !st.Running {
		t.Error("expected running=true after Start")
	}
	if st.Port == 0 {
		t.Error("expected non-zero port after Start")
	}
	if st.PID == 0 {
		t.Error("expected non-zero PID after Start")
	}

	if err := mgr.Stop("test-engine-start"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Give the monitor goroutine time to mark not-running.
	time.Sleep(100 * time.Millisecond)
	st = mgr.Status("test-engine-start")
	if st.Running {
		t.Error("expected running=false after Stop")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-dup")
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-dup"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop("test-engine-dup") //nolint:errcheck

	if err := mgr.Start(ctx, "test-engine-dup"); err == nil {
		t.Error("expected error starting already-running engine")
	}
}

func TestCancelDisconnect(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-cancel")
	man.Engine.Cancel = &manifest.Cancel{Mode: manifest.CancelModeDisconnect}
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-cancel"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop("test-engine-cancel") //nolint:errcheck

	if err := mgr.Cancel(ctx, "test-engine-cancel"); err != nil {
		t.Fatalf("Cancel(disconnect): %v", err)
	}
}

func TestCancelEndpoint(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-cancel-ep")
	man.Engine.Cancel = &manifest.Cancel{
		Mode: manifest.CancelModeEndpoint,
		Path: "/api/cancel",
	}
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-cancel-ep"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop("test-engine-cancel-ep") //nolint:errcheck

	if err := mgr.Cancel(ctx, "test-engine-cancel-ep"); err != nil {
		t.Fatalf("Cancel(endpoint): %v", err)
	}
}

func TestIdleStop(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-idle")
	man.Engine.KeepAlive = 1 // 1 second idle timeout
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-idle"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for idle timer to fire and stop the engine.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if !mgr.Status("test-engine-idle").Running {
			return // success
		}
	}
	t.Error("engine still running after idle timeout")
	_ = mgr.Stop("test-engine-idle")
}

func TestLogStream(t *testing.T) {
	mgr := New()
	man := testManifest("test-engine-logs")
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx, "test-engine-logs"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop("test-engine-logs") //nolint:errcheck

	ch, cancel := mgr.LogStream("test-engine-logs")
	defer cancel()
	if ch == nil {
		t.Error("LogStream returned nil channel")
	}
}

func TestNotRegistered(t *testing.T) {
	mgr := New()
	ctx := context.Background()

	if err := mgr.Start(ctx, "nosuchengine"); err == nil {
		t.Error("expected error for unregistered engine Start")
	}
	if err := mgr.Stop("nosuchengine"); err == nil {
		t.Error("expected error for unregistered engine Stop")
	}
}
