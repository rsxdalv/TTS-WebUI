package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

// commitFixture writes an export that records the commit it was built from.
func commitFixture(t *testing.T, exportCommit string) paths.Root {
	t.Helper()
	dir := t.TempDir()
	touch(t, dir, "out/index.html")
	buildable(t, dir)
	body := `{"buildId":"x"}`
	if exportCommit != "" {
		body = `{"buildId":"x","commit":"` + exportCommit + `"}`
	}
	if err := os.WriteFile(filepath.Join(dir, "react-ui", "out", "_tts-export.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func withHead(t *testing.T, head string) {
	t.Helper()
	old := gitHead
	gitHead = func(paths.Root) string { return head }
	t.Cleanup(func() { gitHead = old })
}

func TestFreshnessByCommit(t *testing.T) {
	cases := []struct {
		name          string
		export, head  string
		exportIsStale bool
	}{
		{"built from HEAD", "aaa", "aaa", false},
		{"built from another commit", "old", "aaa", true},
		{"HEAD unknown: cannot tell", "old", "", false},
		{"export recorded no commit: cannot tell", "", "aaa", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHead(t, c.head)
			if got := exportStale(commitFixture(t, c.export)); got != c.exportIsStale {
				t.Errorf("exportStale = %v, want %v", got, c.exportIsStale)
			}
		})
	}
}

func TestNoExportIsNotStale(t *testing.T) {
	withHead(t, "aaa")
	root, _ := paths.New(t.TempDir())
	if exportStale(root) {
		t.Fatal("a missing export counts as stale")
	}
}

func TestStaleExportIsPlannedForRebuild(t *testing.T) {
	withHead(t, "new")
	plan, err := planReactUI(commitFixture(t, "old"), modeSwitches(t, nil))
	if err != nil || !plan.rebuildExport || plan.buildExport {
		t.Fatalf("plan %+v, err %v", plan, err)
	}
	plan, err = planReactUI(commitFixture(t, "new"), modeSwitches(t, nil))
	if err != nil || plan != (reactUIPlan{}) {
		t.Fatalf("plan %+v, err %v", plan, err)
	}
}

func TestStaleExportIsRebuiltOrRefused(t *testing.T) {
	sw := modeSwitches(t, nil)
	withHead(t, "new")

	t.Run("failed export build starts nothing", func(t *testing.T) {
		s := newReactPrep(commitFixture(t, "old"), false)
		s.runTool = func(_ context.Context, name string, args ...string) error {
			if strings.Join(args, " ") != "run build:static" {
				t.Fatalf("ran %s %v", name, args)
			}
			return errors.New("exit code 1")
		}
		if err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("export build that leaves it stale is refused", func(t *testing.T) {
		s := newReactPrep(commitFixture(t, "old"), false)
		s.runTool = func(context.Context, string, ...string) error { return nil }
		if err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "still missing or stale") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("successful export build", func(t *testing.T) {
		root := commitFixture(t, "old")
		s := newReactPrep(root, false)
		s.runTool = func(context.Context, string, ...string) error {
			return os.WriteFile(filepath.Join(root.Dir(), "react-ui", "out", "_tts-export.json"), []byte(`{"commit":"new"}`), 0o644)
		}
		if err := s.prepareReactUI(context.Background(), sw); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBadRoutesFailBeforeAnyBuild(t *testing.T) {
	t.Setenv(switches.EnvRoutes, "uplod")
	// A stale export would build the React UI with the real tools (which fail
	// here with another error), so an error naming the value proves the check
	// ran first.
	withHead(t, "new")
	cfg := superviseConfig(commitFixture(t, "old"), options{})
	_, err := cfg.Prepare(context.Background())
	if err == nil || !strings.Contains(err.Error(), "uplod") {
		t.Fatalf("err = %v", err)
	}
}
