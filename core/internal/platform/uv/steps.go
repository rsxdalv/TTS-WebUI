package uv

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// The steps an install reports (platform.Reporter). The ids are what the page
// keys its rows on: the platform's own steps are "uv", "python" and the tools'
// names; an entry's venv steps are "<package>:venv", ":torch" and ":requirements".

// StepUV and StepPython are the ids of the platform's own steps.
const (
	StepUV     = "uv"
	StepPython = "python"
)

// InstallPlan is the platform's install, step by step.
func (p *Platform) InstallPlan() []platform.PlanItem {
	items := []platform.PlanItem{
		{ID: StepUV, Label: "Download uv " + Version},
		{ID: StepPython, Label: "Install Python " + PythonVersion},
	}
	for _, t := range p.pinnedTools() {
		items = append(items, platform.PlanItem{ID: t.Name, Label: "Download " + t.Name + " " + t.Version})
	}
	return items
}

// CreatePlan is the venv of one entry: the environment, torch, the requirements.
func CreatePlan(pkg, label string) []platform.PlanItem {
	return []platform.PlanItem{
		{ID: pkg + ":venv", Label: "Create the " + label + " environment"},
		{ID: pkg + ":torch", Label: "Install torch"},
		{ID: pkg + ":wheels", Label: "Fetch the " + label + " wheels (alongside torch)"},
		{ID: pkg + ":requirements", Label: "Install the " + label + " requirements"},
	}
}

// report is platform.Step with a state: the common shape of a step starting,
// finishing or being skipped.
func report(ctx context.Context, id, state, detail string) {
	platform.Step(ctx, platform.StepEvent{ID: id, State: state, Detail: detail})
}

// downloadProgress reports a download of known size through the Reporter.
func downloadProgress(ctx context.Context, id string) func(done, total int64) {
	return func(done, total int64) {
		platform.Step(ctx, platform.StepEvent{ID: id, State: platform.StepRunning, Done: done, Total: total})
	}
}

var (
	reDownloading = regexp.MustCompile(`^Downloading (\S+) \(([\d.]+)\s*([KMG]i?B)\)`)
	reDownloaded  = regexp.MustCompile(`^Downloaded (\S+)`)
	reBuilt       = regexp.MustCompile(`^(Built|Building) (\S+)`)
	reResolved    = regexp.MustCompile(`^Resolved (\d+) packages? in`)
	rePrepared    = regexp.MustCompile(`^Prepared (\d+) packages? in (.+)$`)
	reInstalled   = regexp.MustCompile(`^Installed (\d+) packages? in`)
)

// uvLines reads uv's own lines for what they say about progress: how many
// packages it has to download and how many it has. uv prints no percentages
// when its output is not a terminal, so these lines are all there is.
type uvLines struct {
	ctx   context.Context
	id    string
	mu    sync.Mutex
	need  int64
	have  int64
	built int64
}

func (u *uvLines) observe(line string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	ev := platform.StepEvent{ID: u.id, State: platform.StepRunning}
	switch {
	case reResolved.MatchString(line):
		ev.Detail = "resolved " + reResolved.FindStringSubmatch(line)[1] + " packages"
	case reDownloading.MatchString(line):
		u.need++
		ev.Detail = "downloading packages"
	case reDownloaded.MatchString(line):
		u.have++
		ev.Detail = "downloading packages"
	case reBuilt.MatchString(line):
		if strings.HasPrefix(line, "Built") {
			u.built++
		}
		ev.Detail = "building " + reBuilt.FindStringSubmatch(line)[2]
	case rePrepared.MatchString(line):
		m := rePrepared.FindStringSubmatch(line)
		ev.Detail = fmt.Sprintf("%s packages ready in %s, installing", m[1], m[2])
	case reInstalled.MatchString(line):
		ev.Detail = "installed " + reInstalled.FindStringSubmatch(line)[1] + " packages"
	default:
		return
	}
	if u.need > 0 {
		ev.Done, ev.Total, ev.Unit = u.have, u.need, "packages"
	}
	platform.Step(u.ctx, ev)
}

// cacheBytes is the size of the torch wheel uv has unpacked into its cache so
// far: the archive folders that have a top-level torch/ in them. uv unpacks a
// wheel while it downloads it, so the growth follows the download of a wheel it
// prints no progress for (torch is 2.4 GB of it). Other wheels (the prefetch
// fills the cache with them at the same time) are not counted.
func cacheBytes(cacheDir string) int64 {
	root := filepath.Join(cacheDir, "archive-v0")
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	var n int64
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		if fi, err := os.Stat(filepath.Join(dir, "torch")); err != nil || !fi.IsDir() {
			continue
		}
		_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
			return nil
		})
	}
	return n
}

// torchUnpackedBytes is a guess of what a torch row takes in the cache once
// unpacked: the download size the table gives, and the wheels unpack to about
// four thirds of it (cu126 measured 4.1 GB for a 3000 MB row).
func torchUnpackedBytes(approxMB int) int64 { return int64(approxMB) * (1 << 20) * 4 / 3 }

// watchCache reports the step's progress from the cache folder's growth until
// stop is closed. Total is the guess, so the bar says "about".
func watchCache(ctx context.Context, id, cacheDir string, total int64, every time.Duration) (stop func()) {
	base := cacheBytes(cacheDir)
	quit := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-quit:
				return
			case <-t.C:
				done := cacheBytes(cacheDir) - base
				if done < 0 {
					done = 0
				}
				if done > total {
					done = total
				}
				platform.Step(ctx, platform.StepEvent{ID: id, State: platform.StepRunning, Done: done, Total: total, Estimated: true})
			}
		}
	}()
	return func() { close(quit); <-finished }
}
