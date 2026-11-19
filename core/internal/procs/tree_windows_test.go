//go:build windows

package procs

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestAssignReportsAnExitedProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "exit 0")
	tree, err := NewTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Wait() }()

	time.Sleep(1500 * time.Millisecond) // let it exit; cmd.Wait has not run yet

	if err := tree.Assign(cmd); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("Assign = %v, want ErrProcessExited", err)
	}
	if err := tree.Kill(); err != nil {
		t.Fatalf("Kill after a failed Assign: %v", err)
	}
}

func TestAssignAttachesARunningProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "ping -n 30 127.0.0.1 >nul")
	tree, err := NewTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Wait() }()

	if err := tree.Assign(cmd); err != nil {
		t.Fatalf("Assign = %v", err)
	}
	if err := tree.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
}
