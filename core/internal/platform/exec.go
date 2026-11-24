package platform

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
)

// Run runs c and passes each output line (stdout and stderr merged, blanks
// dropped) to emit. It returns the command's exit error.
func Run(ctx context.Context, c Command, emit Emit) error {
	cmd := exec.CommandContext(ctx, c.Program, c.Args...)
	cmd.Dir = c.Dir
	if c.Env != nil {
		cmd.Env = c.Env
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { pw.CloseWithError(cmd.Wait()) }()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	sc.Split(splitLines)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			emit(l)
		}
	}
	return sc.Err() // the command's exit error, passed through CloseWithError
}

// splitLines splits on \n and \r, like universal_newlines (pip's progress
// bars use \r).
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	for i, c := range data {
		if c == '\n' || c == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
