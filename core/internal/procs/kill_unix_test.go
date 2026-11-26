//go:build !windows

package procs

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

// A command that ended before Assign must not be reported as a failure. On
// macOS getpgid of an ended child fails with ESRCH; that has to come back as
// ErrProcessExited (or succeed, as on Linux where a zombie keeps its group).
func TestAssignAfterTheCommandExited(t *testing.T) {
	cmd := exec.Command("true")
	tree, err := NewTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // let it end, without reaping it yet
	if err := tree.Assign(cmd); err != nil && !errors.Is(err, ErrProcessExited) {
		t.Fatalf("Assign = %v, want nil or ErrProcessExited", err)
	}
	_ = cmd.Wait()
	tree.Release()
}
