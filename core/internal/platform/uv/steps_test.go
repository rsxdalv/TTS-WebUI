package uv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// recorder is a Reporter that keeps what it is told.
type recorder struct {
	mu    sync.Mutex
	plans [][]platform.PlanItem
	steps []platform.StepEvent
}

func (r *recorder) Plan(items []platform.PlanItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans = append(r.plans, items)
}

func (r *recorder) Step(ev platform.StepEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, ev)
}

// states is the last state each step reported, and the order they first reported in.
func (r *recorder) states() (map[string]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	last := map[string]string{}
	var order []string
	for _, e := range r.steps {
		if e.State == "" {
			continue
		}
		if _, ok := last[e.ID]; !ok {
			order = append(order, e.ID)
		}
		last[e.ID] = e.State
	}
	return last, order
}

func TestInstallReportsItsSteps(t *testing.T) {
	f := newFixture(t, gpu.CPU)
	rec := &recorder{}
	ctx := platform.WithReporter(context.Background(), rec)
	if err := f.p.Install(ctx, f.emit); err != nil {
		t.Fatal(err)
	}
	last, order := rec.states()
	if last[StepUV] != platform.StepDone || last[StepPython] != platform.StepDone {
		t.Fatalf("states = %v", last)
	}
	if len(order) < 2 || order[0] != StepUV || order[1] != StepPython {
		t.Errorf("order = %v, want uv then python first", order)
	}
	// A second install finds uv there.
	rec2 := &recorder{}
	if err := f.p.Install(platform.WithReporter(context.Background(), rec2), f.emit); err != nil {
		t.Fatal(err)
	}
	if last2, _ := rec2.states(); last2[StepUV] != platform.StepSkipped {
		t.Errorf("second install: uv is %q, want skipped", last2[StepUV])
	}
}

func TestInstallPlanNamesUVPythonAndTheTools(t *testing.T) {
	f := newFixture(t, gpu.CPU)
	items := f.p.InstallPlan()
	if len(items) < 2 || items[0].ID != StepUV || items[1].ID != StepPython {
		t.Fatalf("plan = %+v", items)
	}
	for _, it := range items {
		if it.Label == "" {
			t.Errorf("%s has no label", it.ID)
		}
	}
}

func TestCreateReportsTheVenvTorchAndRequirements(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the NVIDIA row does not exist on macOS")
	}
	f := installed(t, gpu.NVIDIA)
	rec := &recorder{}
	ctx := platform.WithReporter(context.Background(), rec)
	e := platform.Entry{Package: "tts_webui_extension.bark", Name: "Bark", Own: true, Requirements: []string{"bark>=1"}}
	if !f.p.Create(ctx, e, f.emit) {
		t.Fatalf("create failed:\n%s", strings.Join(*f.out, "\n"))
	}
	last, order := rec.states()
	for _, g := range []string{"venv", "torch", "requirements"} {
		if last[e.Package+":"+g] != platform.StepDone {
			t.Errorf("%s: %q, want done (%v)", g, last[e.Package+":"+g], last)
		}
	}
	var venv, torch, req int = -1, -1, -1
	for i, id := range order {
		switch id {
		case e.Package + ":venv":
			venv = i
		case e.Package + ":torch":
			torch = i
		case e.Package + ":requirements":
			req = i
		}
	}
	if !(venv >= 0 && venv < torch && torch < req) {
		t.Errorf("order = %v", order)
	}
	// The torch step carries the row in its label.
	var labelled bool
	for _, ev := range rec.steps {
		if ev.ID == e.Package+":torch" && strings.Contains(ev.Label, "cu128") {
			labelled = true
		}
	}
	if !labelled {
		t.Error("the torch step does not say which row it installs")
	}
}

func TestCreatePlanIDs(t *testing.T) {
	items := CreatePlan("pkg", "Thing")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if strings.Join(ids, ",") != "pkg:venv,pkg:torch,pkg:wheels,pkg:requirements" {
		t.Errorf("ids = %v", ids)
	}
}

func TestUVLinesCountsPackages(t *testing.T) {
	rec := &recorder{}
	u := &uvLines{ctx: platform.WithReporter(context.Background(), rec), id: "x:requirements"}
	for _, l := range []string{
		"Resolved 162 packages in 28.54s",
		"Downloading gradio (60.6MiB)",
		"Downloading scipy (39.3MiB)",
		"Downloaded gradio",
		"Building jieba==0.42.1",
		"Prepared 149 packages in 1m 38s",
		"Installed 149 packages in 12.42s",
		"+ gradio==5.49.1", // not a progress line
	} {
		u.observe(l)
	}
	if len(rec.steps) != 7 {
		t.Fatalf("%d events: %+v", len(rec.steps), rec.steps)
	}
	var withCount platform.StepEvent
	for _, e := range rec.steps {
		if e.Unit == "packages" && e.Done == 1 && e.Total == 2 {
			withCount = e
		}
	}
	if withCount.ID == "" {
		t.Errorf("no event counted 1 of 2 packages: %+v", rec.steps)
	}
}

func TestWatchCacheReportsTheGrowthOfTheCache(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "archive-v0", "abc", "torch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Another wheel's folder is not torch's, and something already in the
	// cache is not part of this install.
	if err := os.MkdirAll(filepath.Join(cache, "archive-v0", "other", "numpy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "archive-v0", "other", "numpy", "x.bin"), make([]byte, 9000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.bin"), make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	ctx := platform.WithReporter(context.Background(), rec)
	stop := watchCache(ctx, "torch", cache, 10000, 10*time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "new.bin"), make([]byte, 4000), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	stop()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.steps) == 0 {
		t.Fatal("no progress reported")
	}
	last := rec.steps[len(rec.steps)-1]
	if last.Done != 4000 || last.Total != 10000 || !last.Estimated || last.State != platform.StepRunning {
		t.Errorf("last = %+v, want 4000 of an estimated 10000", last)
	}
}

func TestWithoutAReporterNothingIsNeeded(t *testing.T) {
	// Every call is safe on a context that has none.
	platform.Plan(context.Background(), platform.PlanItem{ID: "a"})
	platform.Step(context.Background(), platform.StepEvent{ID: "a"})
}

func TestDownloadProgressReportsBytes(t *testing.T) {
	rec := &recorder{}
	downloadProgress(platform.WithReporter(context.Background(), rec), "git")(5<<20, 38<<20)
	if len(rec.steps) != 1 || rec.steps[0] != (platform.StepEvent{ID: "git", State: platform.StepRunning, Done: 5 << 20, Total: 38 << 20}) {
		t.Errorf("events = %+v", rec.steps)
	}
}

func TestRunParallelRunsTogetherAndTheFirstErrorCancelsTheRest(t *testing.T) {
	started := make(chan struct{}, 2)
	var wasCancelled bool
	err := runParallel(context.Background(),
		func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done() // waits for the other task's failure
			wasCancelled = true
			return ctx.Err()
		},
		func(ctx context.Context) error {
			started <- struct{}{}
			// Both tasks are running at once: the first has started too.
			for i := 0; i < 2; i++ {
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					return errors.New("the other task did not start")
				}
			}
			return errors.New("boom")
		},
	)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v, want the first error", err)
	}
	if !wasCancelled {
		t.Error("the other task was not cancelled")
	}
}

func TestRequirementLines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("numpy\n\ngradio==5 # note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, ok := requirementLines(root, []string{"pip", "-r", "requirements.txt", "scipy"})
	if !ok || strings.Join(lines, "|") != "pip|numpy||gradio==5 # note|scipy" {
		t.Errorf("lines = %q, ok = %v", lines, ok)
	}
	if _, ok := requirementLines(root, []string{"--extra-index-url", "https://x", "numpy"}); ok {
		t.Error("an option must make the set unhandled")
	}
	if _, ok := requirementLines(root, []string{"-r", "missing.txt"}); ok {
		t.Error("a missing file must make the set unhandled")
	}
}

func TestPrefetchPinsLeaveOutTheTorchFamily(t *testing.T) {
	got := prefetchPins("# header\nnumpy==2.0\ntorch==2.14.1\ntorchaudio==2.1\ntorchcodec==0.1\nnvidia-cublas-cu12==12\ntriton==3\nxformers==0.0.35\ntorchmetrics==1.0\nscipy==1.2\n")
	if strings.Join(got, ",") != "numpy==2.0,torchmetrics==1.0,scipy==1.2" {
		t.Errorf("pins = %v", got)
	}
}

func TestCreatePrefetchesTheWheelsBesideTorchAndTheRequirementsStillRun(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the NVIDIA row does not exist on macOS")
	}
	f := installed(t, gpu.NVIDIA)
	f.p.prefetch = true
	rec := &recorder{}
	ctx := platform.WithReporter(context.Background(), rec)
	e := platform.Entry{Package: "tts_webui_extension.bark", Name: "Bark", Own: true, Requirements: []string{"numpy", "scipy"}}
	if !f.p.Create(ctx, e, f.emit) {
		t.Fatalf("create failed:\n%s", strings.Join(*f.out, "\n"))
	}
	calls := strings.Join(f.calls(), "\n")
	for _, want := range []string{"pip compile ", "--no-deps --target "} {
		if !strings.Contains(calls, want) {
			t.Errorf("uv was not asked for %q:\n%s", want, calls)
		}
	}
	last, _ := rec.states()
	if last[e.Package+":wheels"] != platform.StepDone || last[e.Package+":requirements"] != platform.StepDone {
		t.Errorf("states = %v", last)
	}
	// The prefetch left nothing behind in the cache folder.
	if m, _ := filepath.Glob(filepath.Join(f.p.CacheDir(), ".prefetch-*")); len(m) != 0 {
		t.Errorf("left %v", m)
	}
}

func TestCreateWithoutPrefetchSaysSo(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the NVIDIA row does not exist on macOS")
	}
	f := installed(t, gpu.NVIDIA)
	rec := &recorder{}
	e := platform.Entry{Package: "tts_webui_extension.bark", Name: "Bark", Own: true, Requirements: []string{"numpy"}}
	if !f.p.Create(platform.WithReporter(context.Background(), rec), e, f.emit) {
		t.Fatalf("create failed:\n%s", strings.Join(*f.out, "\n"))
	}
	if last, _ := rec.states(); last[e.Package+":wheels"] != platform.StepSkipped {
		t.Errorf("wheels: %v", last)
	}
}
