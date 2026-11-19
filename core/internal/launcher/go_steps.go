package launcher

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rsxdalv/TTS-WebUI/core/installer_scripts"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

// knownGoSteps are the names TTS_GO_STEPS accepts. Each one has a Go port of the
// script the step normally runs.
var knownGoSteps = []string{"mamba", "install", "update", "diagnostic"}

// useGoStep reports whether step name runs its Go port instead of the script.
// It is on unless TTS_GO_STEPS=none or a list that leaves the step out (decision
// D19). A bad value is an error, so a typo never silently runs the other code path. Only TTS_GO_STEPS
// is read here: the proxy and route switches belong to the app supervisor.
func useGoStep(name string) (bool, error) {
	sw, err := switches.Parse(func(k string) (string, bool) {
		if k == switches.EnvSteps {
			return os.LookupEnv(k)
		}
		return "", false
	})
	if err != nil {
		return false, err
	}
	if err := sw.CheckSteps(knownGoSteps); err != nil {
		return false, err
	}
	return sw.Step(name), nil
}

// goStepFunc is the shape of every installer_scripts port.
type goStepFunc func(ctx context.Context, s *installer_scripts.Step) (int, error)

// serveGoStep handles a step request with the Go port when the switch is on.
// It returns true when it has answered the request, false when the caller
// should run the script as before.
func (l *Launcher) serveGoStep(w http.ResponseWriter, name string, fn goStepFunc) bool {
	use, err := useGoStep(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("starting %s: %v", name, err), http.StatusInternalServerError)
		return true
	}
	if !use {
		return false
	}
	p := &procs.Proc{}
	l.setProc(name, p)
	p.Note(fmt.Sprintf("[launcher] %s: running the Go port (TTS_GO_STEPS=none runs the script instead)", name))
	err = p.StartFunc(context.Background(), func(ctx context.Context, log func(string)) int {
		code, err := fn(ctx, installer_scripts.NewStep(l.root.Dir(), log))
		if err != nil {
			log("ERROR: " + err.Error())
			if code == 0 {
				code = 1
			}
		}
		return code
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("starting %s: %v", name, err), http.StatusInternalServerError)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"started":true}`))
	return true
}

// scriptNote is the log line of a step that runs its original script.
func scriptNote(name string) string {
	return fmt.Sprintf("[launcher] %s: running the original script (TTS_GO_STEPS selects the Go port)", name)
}
