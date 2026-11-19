package procs

import (
	"errors"
	"os/exec"
)

// ErrProcessExited means the process ended before it could be attached to its
// tree. The process ran normally, so callers carry on and collect its exit code.
var ErrProcessExited = errors.New("process already exited")

// Tree lets a caller that owns its own exec.Cmd (and its pipes) kill the whole
// process tree, using the same job object or process group as Proc.
type Tree struct {
	ps *platformState
}

// NewTree prepares cmd for tree-kill. Call it before cmd.Start.
func NewTree(cmd *exec.Cmd) (*Tree, error) {
	ps, err := newPlatformState(cmd)
	if err != nil {
		return nil, err
	}
	return &Tree{ps: ps}, nil
}

// Assign attaches the started process to the tree. Call it after cmd.Start. It
// returns ErrProcessExited if the process has already ended.
func (t *Tree) Assign(cmd *exec.Cmd) error { return t.ps.assign(cmd) }

// Kill force-kills the process and all its descendants.
func (t *Tree) Kill() error { return t.ps.force() }

// Release frees the tree's resources once the process has exited.
func (t *Tree) Release() { t.ps.release() }
