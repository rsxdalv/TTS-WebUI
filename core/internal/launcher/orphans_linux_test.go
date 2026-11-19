//go:build linux

package launcher

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// startGroup starts `sleep` in its own process group with cwd dir, like an
// app process.
func startGroup(t *testing.T, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd
}

func TestSweepStopsOrphansOfThisInstallOnly(t *testing.T) {
	root, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root.InstallerFiles(), 0o755); err != nil {
		t.Fatal(err)
	}
	ours := startGroup(t, root.Dir())
	other := startGroup(t, t.TempDir()) // another install
	start, ok := processStart(ours.Process.Pid)
	if !ok {
		t.Fatal("no start time for a live process")
	}
	otherStart, _ := processStart(other.Process.Pid)
	writeRecord(t, root, appRecord{Root: root.Dir(), Groups: []groupRecord{
		{Name: "gradio", Pgid: ours.Process.Pid, Start: start},
		{Name: "elsewhere", Pgid: other.Process.Pid, Start: otherStart},
		// The same group number, recorded for a later process: a reused PID.
		{Name: "reused", Pgid: other.Process.Pid, Start: otherStart + 1_000_000},
	}})

	done := make(chan struct{})
	go func() { _ = ours.Wait(); close(done) }()
	if n := sweepOrphans(root); n != 1 {
		t.Fatalf("stopped %d processes, want 1", n)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the orphan of this install is still running")
	}
	if err := syscall.Kill(other.Process.Pid, 0); err != nil {
		t.Fatal("a process of another install was killed")
	}
	if _, err := os.Stat(recordPath(root)); !os.IsNotExist(err) {
		t.Fatal("the record was not removed")
	}
}

func TestProcStatOfSelf(t *testing.T) {
	pg, start, ok := procStat(os.Getpid())
	if !ok || pg <= 0 || start == 0 {
		t.Fatalf("procStat(self) = %d %d %v", pg, start, ok)
	}
}
