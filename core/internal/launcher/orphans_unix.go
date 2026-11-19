//go:build !windows

package launcher

// Orphan handling on Linux and macOS (R2-1). Each app process runs in its own
// process group, so a launcher that dies by SIGKILL leaves the groups running.
// Start records each group, and the next launcher (or the next Start) stops
// what is left of them. A recorded group is only signalled after checking that
// its members still belong to this install, so a reused PID is never killed.

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// appRecord is the file that lists the running app's process groups.
type appRecord struct {
	Root   string        `json:"root"`
	Groups []groupRecord `json:"groups"`
}

type groupRecord struct {
	Name string `json:"name"`
	Pgid int    `json:"pgid"`
	// Start is the leader's start time as the platform reports it (Linux:
	// clock ticks since boot, from /proc/<pid>/stat). Members started later.
	Start uint64 `json:"start"`
}

func recordPath(root paths.Root) string {
	return filepath.Join(root.InstallerFiles(), "app-processes.json")
}

// recordProcesses writes the process groups of the running app.
func (s *Supervisor) recordProcesses() {
	s.mu.Lock()
	var refs []appProc
	for _, ap := range s.procs {
		if ap.proc != nil {
			refs = append(refs, appProc{Name: ap.name, PID: ap.proc.Pid()})
		}
	}
	s.mu.Unlock()
	recordGroups(s.root, refs)
}

// recordGroups writes the record for the given app processes.
func recordGroups(root paths.Root, procs []appProc) {
	rec := appRecord{Root: root.Dir()}
	for _, p := range procs {
		if p.PID <= 0 {
			continue
		}
		start, _ := processStart(p.PID)
		rec.Groups = append(rec.Groups, groupRecord{Name: p.Name, Pgid: p.PID, Start: start})
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(recordPath(root), data, 0o644); err != nil {
		slog.Warn("could not record the app processes", "err", err)
	}
}

// clearRecord removes the record after a clean Stop.
func (s *Supervisor) clearRecord() { clearRecordFor(s.root) }

func clearRecordFor(root paths.Root) { _ = os.Remove(recordPath(root)) }

// sweepOrphans stops processes a killed launcher left running for this
// install, logs one line when it finds any, and removes the record.
func sweepOrphans(root paths.Root) int {
	data, err := os.ReadFile(recordPath(root))
	if err != nil {
		return 0
	}
	defer os.Remove(recordPath(root))
	var rec appRecord
	if json.Unmarshal(data, &rec) != nil || filepath.Clean(rec.Root) != filepath.Clean(root.Dir()) {
		return 0
	}

	var pids []int
	var names []string
	for _, g := range rec.Groups {
		members := groupMembers(root.Dir(), g)
		if len(members) > 0 {
			pids = append(pids, members...)
			names = append(names, g.Name)
		}
	}
	if len(pids) == 0 {
		return 0
	}
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && anyAlive(pids) {
		time.Sleep(100 * time.Millisecond)
	}
	for _, pid := range pids {
		if alive(pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	slog.Warn("stopped processes a previous launcher left running for this install",
		"count", len(pids), "apps", strings.Join(names, ", "), "pids", pids)
	return len(pids)
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func anyAlive(pids []int) bool {
	for _, pid := range pids {
		if alive(pid) {
			return true
		}
	}
	return false
}
