package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
)

// pendingSuffix names the new launcher binary an update leaves next to the
// running one.
const pendingSuffix = ".new"

// runningExe is the running launcher binary (tests replace it).
var runningExe = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe
}

func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ia, ea := os.Stat(a)
	ib, eb := os.Stat(b)
	return ea == nil && eb == nil && os.SameFile(ia, ib)
}

// SwapPendingBinary puts a launcher binary an update left (<exe>.new) in place of
// the running one and runs it with the same arguments. It returns handled true and
// the new program's exit code when it did; the caller then exits with that code.
// A swap that fails is returned as an error and nothing is replaced, so the old
// launcher keeps running and says why.
func SwapPendingBinary(args []string) (handled bool, code int, err error) {
	exe := runningExe()
	if exe == "" {
		return false, 0, nil
	}
	_ = os.Remove(exe + ".old") // left by the previous swap
	pending := exe + pendingSuffix
	if st, e := os.Stat(pending); e != nil || !st.Mode().IsRegular() {
		return false, 0, nil
	}
	// A running exe can be renamed on Windows, not overwritten.
	if err := os.Rename(exe, exe+".old"); err != nil {
		return false, 0, err
	}
	if err := os.Rename(pending, exe); err != nil {
		_ = os.Rename(exe+".old", exe) // keep the old launcher
		return false, 0, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return true, ee.ExitCode(), nil
		}
		return true, 1, err
	}
	return true, 0, nil
}

// extractTarget gives the path a zip entry is written to: the running launcher
// binary is written beside itself with pendingSuffix.
func extractTarget(target string) string {
	if sameFile(target, runningExe()) {
		return target + pendingSuffix
	}
	return target
}
