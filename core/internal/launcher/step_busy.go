package launcher

import "net/http"

// installSteps change the install. Only one of them runs at a time (R2-3).
var installSteps = []string{"download", "mamba", "install", "update"}

// runningStep returns the install step that is running, or "".
func (l *Launcher) runningStep() string {
	for _, s := range installSteps {
		if s == "download" {
			if l.downloadRunning() {
				return s
			}
			continue
		}
		if s == "update" && l.updateRunning() {
			return s
		}
		if l.isStepRunning(s) {
			return s
		}
	}
	return ""
}

// downloadRunning reports whether the download goroutine has not finished.
func (l *Launcher) downloadRunning() bool {
	lg := l.getLog("download")
	if lg == nil {
		return false
	}
	select {
	case <-lg.Done():
		return false
	default:
		return true
	}
}

// refuseWhileBusy answers 409 and returns true when an install step is
// running, so a second click or a click on another step after a page reload
// cannot start anything meanwhile. Callers hold l.stepMu, so the check and the
// start of the new step cannot interleave with another request.
func (l *Launcher) refuseWhileBusy(w http.ResponseWriter, step string) bool {
	busy := l.runningStep()
	if busy == "" {
		return false
	}
	msg := busy + " is still running; wait for it to finish before starting " + step
	if busy == step {
		msg = step + " is already running"
	}
	http.Error(w, msg, http.StatusConflict)
	return true
}
