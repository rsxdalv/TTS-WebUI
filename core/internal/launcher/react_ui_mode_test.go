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

func modeSwitches(t *testing.T, env map[string]string) switches.Set {
	t.Helper()
	s, err := switches.Parse(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func touch(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		full := filepath.Join(root, "react-ui", filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanReactUI(t *testing.T) {
	export := []string{"out/_tts-export.json", "out/index.html"}
	server := []string{".next/BUILD_ID", ".next/prerender-manifest.json"}
	proxyOn := map[string]string{}
	proxyOff := map[string]string{switches.EnvProxy: "0"}

	cases := []struct {
		name  string
		files []string
		env   map[string]string
		want  reactUIPlan
		err   error
	}{
		{"export, default switches", append(export, server...), proxyOn, reactUIPlan{mode: modeStatic}, nil},
		{"export only", export, proxyOn, reactUIPlan{mode: modeStatic}, nil},
		{"old install: server build only", server, proxyOn, reactUIPlan{mode: modeNextProxied}, nil},
		{"export without its marker", append([]string{"out/index.html"}, server...), proxyOn, reactUIPlan{mode: modeNextProxied}, nil},
		{"nothing built", nil, proxyOn, reactUIPlan{}, errUINotBuilt},
		{"BUILD_ID only (killed first export)", []string{".next/BUILD_ID"}, proxyOn, reactUIPlan{}, errUINotBuilt},
		{"killed export with the build set aside", []string{".next-server-saved/BUILD_ID", ".next-export-scratch"}, proxyOn, reactUIPlan{mode: modeNextProxied, recover: true}, nil},
		{"proxy off ignores the export", append(export, server...), proxyOff, reactUIPlan{mode: modeNextDirect}, nil},
		{"claims off, server build: Next serves the UI", append(export, server...), map[string]string{switches.EnvRoutes: "none"},
			reactUIPlan{mode: modeNextProxied, missing: "gradio, gradio-file-proxy, upload, webui-generations, data-outputs, data-bark-voices"}, nil},
		{"some claims off, server build", append(export, server...), map[string]string{switches.EnvRoutes: "gradio,gradio-file-proxy,webui-generations,data-outputs,data-bark-voices"},
			reactUIPlan{mode: modeNextProxied, missing: "upload"}, nil},
		{"claims off, no server build", export, map[string]string{switches.EnvRoutes: "none"}, reactUIPlan{}, errClaimsOff},
		{"every needed claim listed", export, map[string]string{switches.EnvRoutes: "gradio,gradio-file-proxy,upload,webui-generations,data-outputs,data-bark-voices"},
			reactUIPlan{mode: modeStatic}, nil},
		{"proxy off without a server build builds it", export, proxyOff, reactUIPlan{mode: modeNextDirect, build: buildMissing}, nil},
		{"proxy off with the build set aside recovers", []string{".next-server-saved/BUILD_ID"}, proxyOff, reactUIPlan{mode: modeNextDirect, recover: true}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, c.files...)
			root, err := paths.New(dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := planReactUI(root, modeSwitches(t, c.env))
			if !errors.Is(err, c.err) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			if err == nil && got != c.want {
				t.Fatalf("plan = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestNotBuiltErrorTellsWhatToDo(t *testing.T) {
	if msg := errUINotBuilt.Error(); !contains(msg, "Install") || !contains(msg, "Update") {
		t.Fatalf("message %q does not name Install or Update", msg)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// staleFixture writes an export and a server build with the given ages.
func staleFixture(t *testing.T, serverAge, exportAge time.Duration) paths.Root {
	t.Helper()
	dir := t.TempDir()
	touch(t, dir, "out/_tts-export.json", "out/index.html", ".next/BUILD_ID", ".next/prerender-manifest.json")
	now := time.Now()
	set := func(name string, age time.Duration) {
		if err := os.Chtimes(filepath.Join(dir, "react-ui", filepath.FromSlash(name)), now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	set(".next/BUILD_ID", serverAge)
	set("out/_tts-export.json", exportAge)
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStaleServerBuildIsRebuilt(t *testing.T) {
	proxyOff := map[string]string{switches.EnvProxy: "0"}
	claimsOff := map[string]string{switches.EnvRoutes: "none"}
	cases := []struct {
		name              string
		serverAge, export time.Duration
		env               map[string]string
		want              reactUIPlan
	}{
		{"proxy off, server build newer than the export", time.Hour, 2 * time.Hour, proxyOff, reactUIPlan{mode: modeNextDirect}},
		{"proxy off, server build older than the export", 2 * time.Hour, time.Hour, proxyOff, reactUIPlan{mode: modeNextDirect, build: buildStale}},
		{"claims off, server build older than the export", 2 * time.Hour, time.Hour, claimsOff,
			reactUIPlan{mode: modeNextProxied, build: buildStale, missing: "gradio, gradio-file-proxy, upload, webui-generations, data-outputs, data-bark-voices"}},
		{"static mode never builds", 2 * time.Hour, time.Hour, map[string]string{}, reactUIPlan{mode: modeStatic}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := planReactUI(staleFixture(t, c.serverAge, c.export), modeSwitches(t, c.env))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("plan = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestStaleNeedsAnExport(t *testing.T) {
	root := staleFixture(t, 2*time.Hour, time.Hour)
	if err := os.Remove(filepath.Join(root.Dir(), "react-ui", "out", "_tts-export.json")); err != nil {
		t.Fatal(err)
	}
	if serverBuildStale(root) {
		t.Fatal("a server build counts as stale without a complete export")
	}
}

// build:static renames .next away and back; that must keep BUILD_ID's time, or
// a stale build would look fresh.
func TestRenameKeepsTheServerBuildTime(t *testing.T) {
	root := staleFixture(t, 2*time.Hour, time.Hour)
	next := filepath.Join(root.Dir(), "react-ui", ".next")
	saved := filepath.Join(root.Dir(), "react-ui", ".next-server-saved")
	if err := os.Rename(next, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, next); err != nil {
		t.Fatal(err)
	}
	if !serverBuildStale(root) {
		t.Fatal("renaming .next away and back made a stale build look fresh")
	}
}

func TestPrepareRebuildsOrRefuses(t *testing.T) {
	sw := modeSwitches(t, map[string]string{switches.EnvProxy: "0"})

	t.Run("failed rebuild starts nothing", func(t *testing.T) {
		s := newSupervisor(staleFixture(t, 2*time.Hour, time.Hour))
		var ran []string
		s.runTool = func(_ context.Context, name string, args ...string) error {
			ran = append(ran, name+" "+strings.Join(args, " "))
			return errors.New("exit code 1")
		}
		_, err := s.prepareReactUI(context.Background(), sw)
		if err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("err = %v, want a refusal", err)
		}
		if len(ran) != 1 || ran[0] != "npm run build" {
			t.Fatalf("ran %q", ran)
		}
	})

	t.Run("rebuild that leaves the build old is refused", func(t *testing.T) {
		s := newSupervisor(staleFixture(t, 2*time.Hour, time.Hour))
		s.runTool = func(context.Context, string, ...string) error { return nil }
		if _, err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "still older") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("successful rebuild", func(t *testing.T) {
		root := staleFixture(t, 2*time.Hour, time.Hour)
		s := newSupervisor(root)
		s.runTool = func(context.Context, string, ...string) error {
			now := time.Now()
			return os.Chtimes(filepath.Join(root.Dir(), "react-ui", ".next", "BUILD_ID"), now, now)
		}
		plan, err := s.prepareReactUI(context.Background(), sw)
		if err != nil || plan.mode != modeNextDirect {
			t.Fatalf("plan %+v, err %v", plan, err)
		}
		if serverBuildStale(root) {
			t.Fatal("still stale after the rebuild")
		}
	})

	t.Run("fresh build is not rebuilt", func(t *testing.T) {
		s := newSupervisor(staleFixture(t, time.Hour, 2*time.Hour))
		s.runTool = func(context.Context, string, ...string) error {
			t.Fatal("a fresh server build was rebuilt")
			return nil
		}
		if _, err := s.prepareReactUI(context.Background(), sw); err != nil {
			t.Fatal(err)
		}
	})
}
