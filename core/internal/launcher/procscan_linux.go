//go:build linux

package launcher

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// procStat reads the process group and start time from /proc/<pid>/stat.
func procStat(pid int) (pgid int, start uint64, ok bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, 0, false
	}
	// The command name is in parentheses and may contain spaces, so count
	// fields from the last ')'. After it: state(3) ppid(4) pgrp(5) ... starttime(22).
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, 0, false
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 20 {
		return 0, 0, false
	}
	pg, err1 := strconv.Atoi(fields[2])
	st, err2 := strconv.ParseUint(fields[19], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return pg, st, true
}

// processStart returns the start time of pid in clock ticks since boot.
func processStart(pid int) (uint64, bool) {
	_, start, ok := procStat(pid)
	return start, ok
}

// groupMembers lists the live processes of a recorded group that still belong
// to this install: same process group, started no earlier than the recorded
// leader, and running in the install root. A group whose number was reused by
// an unrelated program fails the last two checks.
func groupMembers(root string, g groupRecord) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	roots := map[string]bool{filepath.Clean(root): true}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		roots[filepath.Clean(real)] = true
	}
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 || pid == os.Getpid() {
			continue
		}
		pg, start, ok := procStat(pid)
		if !ok || pg != g.Pgid || start < g.Start {
			continue
		}
		cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		if err != nil || !roots[filepath.Clean(cwd)] {
			continue
		}
		out = append(out, pid)
	}
	return out
}
