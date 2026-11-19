package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

// commitFixture writes an export and a server build that record commits.
func commitFixture(t *testing.T, exportCommit, serverCommit string) paths.Root {
	t.Helper()
	dir := t.TempDir()
	touch(t, dir, "out/index.html", ".next/BUILD_ID", ".next/prerender-manifest.json")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, "react-ui", filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("out/_tts-export.json", `{"buildId":"x","commit":"`+exportCommit+`"}`)
	write(".next/tts-build.json", `{"commit":"`+serverCommit+`"}`)
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
		name                     string
		export, server, head     string
		exportStale, serverStale bool
	}{
		{"both from HEAD", "aaa", "aaa", "aaa", false, false},
		{"script Update rebuilt only the server build", "old", "aaa", "aaa", true, false},
		{"Go Update rebuilt only the export", "aaa", "old", "aaa", false, true},
		// Without HEAD the commits cannot be compared, so the file times decide:
		// the fixture's export marker is newer than BUILD_ID.
		{"HEAD unknown: file times decide", "old", "aaa", "", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHead(t, c.head)
			root := commitFixture(t, c.export, c.server)
			if got := exportStale(root); got != c.exportStale {
				t.Errorf("exportStale = %v, want %v", got, c.exportStale)
			}
			if got := serverBuildStale(root); got != c.serverStale {
				t.Errorf("serverBuildStale = %v, want %v", got, c.serverStale)
			}
		})
	}
}

func TestRebuildingOneBuildKeepsTheOtherFresh(t *testing.T) {
	withHead(t, "aaa")
	root := commitFixture(t, "aaa", "aaa")
	// Make the server build much newer than the export, as a later rebuild
	// of it would: with commits recorded, the export must stay fresh.
	now := time.Now()
	_ = os.Chtimes(filepath.Join(root.Dir(), "react-ui", ".next", "BUILD_ID"), now, now)
	if exportStale(root) || serverBuildStale(root) {
		t.Fatal("a newer build of the same commit made the other one stale")
	}
}

func TestStaleExportWithoutRecordIsOlderThanServerBuild(t *testing.T) {
	withHead(t, "")
	root := staleFixture(t, time.Hour, 2*time.Hour) // export older
	if !exportStale(root) {
		t.Fatal("an export older than the server build, with no commit, must count as stale")
	}
	plan, err := planReactUI(root, modeSwitches(t, map[string]string{}))
	if err != nil || plan.mode != modeStatic || !plan.rebuildExport {
		t.Fatalf("plan %+v, err %v", plan, err)
	}
}

func TestStaleExportIsRebuiltOrRefused(t *testing.T) {
	sw := modeSwitches(t, map[string]string{})
	withHead(t, "new")

	t.Run("failed export build starts nothing", func(t *testing.T) {
		s := newSupervisor(commitFixture(t, "old", "new"))
		s.runTool = func(_ context.Context, name string, args ...string) error {
			if strings.Join(args, " ") != "run build:static" {
				t.Fatalf("ran %s %v", name, args)
			}
			return errors.New("exit code 1")
		}
		if _, err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("export build that leaves it stale is refused", func(t *testing.T) {
		s := newSupervisor(commitFixture(t, "old", "new"))
		s.runTool = func(context.Context, string, ...string) error { return nil }
		if _, err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "still missing or stale") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("successful export build", func(t *testing.T) {
		root := commitFixture(t, "old", "new")
		s := newSupervisor(root)
		s.runTool = func(context.Context, string, ...string) error {
			return os.WriteFile(filepath.Join(root.Dir(), "react-ui", "out", "_tts-export.json"), []byte(`{"commit":"new"}`), 0o644)
		}
		plan, err := s.prepareReactUI(context.Background(), sw)
		if err != nil || plan.mode != modeStatic {
			t.Fatalf("plan %+v, err %v", plan, err)
		}
	})
}

func TestBuildStaticScriptDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "react-ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, _ := paths.New(dir)
	pkg := filepath.Join(dir, "react-ui", "package.json")
	_ = os.WriteFile(pkg, []byte(`{"scripts":{"build":"next build"}}`), 0o644)
	if hasBuildStaticScript(root) {
		t.Fatal("a tree without build:static reported one")
	}
	_ = os.WriteFile(pkg, []byte(`{"scripts":{"build":"next build","build:static":"node build-static.js"}}`), 0o644)
	if !hasBuildStaticScript(root) {
		t.Fatal("build:static not found")
	}
}

func TestBadRoutesFailBeforeAnyBuild(t *testing.T) {
	t.Setenv(switches.EnvRoutes, "uplod")
	s := newSupervisor(staleFixture(t, 2*time.Hour, time.Hour))
	s.runTool = func(context.Context, string, ...string) error {
		t.Fatal("a build ran before the routes value was checked")
		return nil
	}
	err := s.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "uplod") {
		t.Fatalf("err = %v", err)
	}
}
