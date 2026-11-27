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

func modeSwitches(t *testing.T, env map[string]string) switches.Set {
	t.Helper()
	s, err := switches.Parse(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// touch writes small files under react-ui.
func touch(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		full := filepath.Join(root, "frontend", filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// buildable makes a react-ui that `npm run build:static` can run in.
func buildable(t *testing.T, dir string) {
	t.Helper()
	pkg := filepath.Join(dir, "frontend", "package.json")
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pkg, []byte(`{"scripts":{"build:static":"node build-static.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

const allClaims = "gradio,gradio-file-proxy,upload,webui-generations,data-outputs,data-bark-voices"

func TestPlanReactUI(t *testing.T) {
	export := []string{"out/_tts-export.json", "out/index.html"}

	cases := []struct {
		name      string
		files     []string
		buildable bool
		env       map[string]string
		want      reactUIPlan
		err       error
	}{
		{"export, default switches", export, false, nil, reactUIPlan{}, nil},
		{"export without its marker is no export", []string{"out/index.html"}, false, nil, reactUIPlan{}, errUINotBuilt},
		{"nothing built, cannot build", nil, false, nil, reactUIPlan{}, errUINotBuilt},
		{"nothing built, can build", nil, true, nil, reactUIPlan{buildExport: true}, nil},
		{"killed export build leaves no marker", []string{"out/index.html"}, true, nil, reactUIPlan{buildExport: true}, nil},
		{"every needed claim listed", export, false, map[string]string{switches.EnvRoutes: allClaims}, reactUIPlan{}, nil},
		{"all claims off is an error, no fallback", export, false, map[string]string{switches.EnvRoutes: "none"}, reactUIPlan{}, errClaimsOff},
		{"one needed claim off is an error", export, false,
			map[string]string{switches.EnvRoutes: "gradio,gradio-file-proxy,webui-generations,data-outputs,data-bark-voices"}, reactUIPlan{}, errClaimsOff},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, c.files...)
			if c.buildable {
				buildable(t, dir)
			}
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

func TestClaimsOffErrorNamesTheMissingClaims(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "out/_tts-export.json", "out/index.html")
	root, _ := paths.New(dir)
	_, err := planReactUI(root, modeSwitches(t, map[string]string{switches.EnvRoutes: "gradio"}))
	if err == nil || !strings.Contains(err.Error(), "upload") || !strings.Contains(err.Error(), "TTS_GO_ROUTES") {
		t.Fatalf("err = %v", err)
	}
}

func TestNotBuiltErrorTellsWhatToDo(t *testing.T) {
	if msg := errUINotBuilt.Error(); !strings.Contains(msg, "Install") || !strings.Contains(msg, "Update") {
		t.Fatalf("message %q does not name Install or Update", msg)
	}
}

func TestCanBuildExport(t *testing.T) {
	dir := t.TempDir()
	root, _ := paths.New(dir)
	if canBuildExport(root) {
		t.Fatal("an empty root can build")
	}
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, "frontend", "package.json")
	_ = os.WriteFile(pkg, []byte(`{"scripts":{"dev":"vite"}}`), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "frontend", "node_modules"), 0o755)
	if canBuildExport(root) {
		t.Fatal("a tree without build:static can build")
	}
	_ = os.WriteFile(pkg, []byte(`{"scripts":{"build:static":"node build-static.js"}}`), 0o644)
	if !canBuildExport(root) {
		t.Fatal("build:static with node_modules not found")
	}
	_ = os.RemoveAll(filepath.Join(dir, "frontend", "node_modules"))
	if canBuildExport(root) {
		t.Fatal("a tree without node_modules can build")
	}
}

func TestPrepareBuildsAMissingExport(t *testing.T) {
	sw := modeSwitches(t, nil)

	t.Run("failed build starts nothing", func(t *testing.T) {
		dir := t.TempDir()
		buildable(t, dir)
		root, _ := paths.New(dir)
		s := newReactPrep(root, false)
		var ran []string
		s.runTool = func(_ context.Context, name string, args ...string) error {
			ran = append(ran, name+" "+strings.Join(args, " "))
			return errors.New("exit code 1")
		}
		err := s.prepareReactUI(context.Background(), sw)
		if err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("err = %v, want a refusal", err)
		}
		if len(ran) != 1 || ran[0] != "npm run build:static" {
			t.Fatalf("ran %q", ran)
		}
	})

	t.Run("build that leaves no export is refused", func(t *testing.T) {
		dir := t.TempDir()
		buildable(t, dir)
		root, _ := paths.New(dir)
		s := newReactPrep(root, false)
		s.runTool = func(context.Context, string, ...string) error { return nil }
		if err := s.prepareReactUI(context.Background(), sw); err == nil || !strings.Contains(err.Error(), "still missing or stale") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("successful build", func(t *testing.T) {
		dir := t.TempDir()
		buildable(t, dir)
		root, _ := paths.New(dir)
		s := newReactPrep(root, false)
		s.runTool = func(context.Context, string, ...string) error {
			touch(t, dir, "out/_tts-export.json", "out/index.html")
			return nil
		}
		if err := s.prepareReactUI(context.Background(), sw); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("an export that is there is not rebuilt", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "out/_tts-export.json", "out/index.html")
		root, _ := paths.New(dir)
		s := newReactPrep(root, false)
		s.runTool = func(context.Context, string, ...string) error {
			t.Fatal("a complete export was rebuilt")
			return nil
		}
		if err := s.prepareReactUI(context.Background(), sw); err != nil {
			t.Fatal(err)
		}
	})
}
