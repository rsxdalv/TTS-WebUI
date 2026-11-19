package launcher

import (
	"errors"
	"os"
	"path/filepath"
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
		{"proxy off without a server build builds it", export, proxyOff, reactUIPlan{mode: modeNextDirect, build: true}, nil},
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
