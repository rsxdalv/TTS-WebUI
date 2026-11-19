//go:build !linux && !windows

package launcher

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// processStart has no cheap equivalent outside Linux; the group check below
// uses the command line instead.
func processStart(int) (uint64, bool) { return 0, false }

// groupMembers lists live processes of a recorded group (macOS and other
// Unixes, through ps) whose command line names this install or one of the app
// programs, so a reused group number of an unrelated program is left alone.
// Not run on macOS by the port's tests; Linux uses /proc instead.
func groupMembers(root string, g groupRecord) []int {
	out, err := exec.Command("ps", "-A", "-o", "pid=", "-o", "pgid=", "-o", "command=").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		pg, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil || pg != g.Pgid || pid == os.Getpid() {
			continue
		}
		cmd := strings.Join(fields[2:], " ")
		if strings.Contains(cmd, root) || strings.Contains(cmd, "server.py") ||
			strings.Contains(cmd, "extension_manager.py") || strings.Contains(cmd, "micromamba") {
			pids = append(pids, pid)
		}
	}
	return pids
}
