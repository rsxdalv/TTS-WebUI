package extcatalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
)

// The extensions.json of the repository: every entry gets a requires, derived
// from what it is, and every name in it exists.
func TestRealCatalogRequires(t *testing.T) {
	root, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "extensions.json")); err != nil {
		t.Skipf("no extensions.json at %s", root)
	}
	s := New(Options{Root: root, Token: "tok"})
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	uv, host, own, audio, onnx := 0, 0, 0, 0, 0
	for name, e := range g.Entries {
		if name == "gradio-host" { // the host is an entry of its own (host_test.go)
			own++
			continue
		}
		if err := g.Check(name); err != nil {
			t.Errorf("%v", err)
		}
		switch strings.Join(e.Requires, ",") {
		case "uv":
			uv++
		case "gradio-host":
			host++
		case "audiocpp": // the audio.cpp models (P7)
			audio++
		case "onnx": // the onnx entry (P8)
			onnx++
		default:
			t.Errorf("%s requires %v", name, e.Requires)
		}
	}
	// P6 moves one engine at a time: 20 at the start, +1 for each converted
	// engine (Kokoro 21st, MMS and Vall-E-X 22nd and 23rd, Chatterbox and StyleTTS2 24th and 25th; ACE-Step, Demucs and Vocos 26th to 28th, RVC, Audio Separator and Resemble Enhance 29th to 31st, AP-BWE, PyRNNoise and Seamless M4T 32nd to 34th, CosyVoice, XTTS and Higgs V2 35th to 37th, VibeVoice, Kimi Audio and MiMo-Audio 38th to 40th; F5-TTS needs gradio>=6, see LAB-49).
	if uv != 40 || audio != len(audiocpp.Models) || onnx != 1 || host+uv+own+audio+onnx != len(g.Entries) || len(g.Entries) < 66 {
		t.Errorf("uv=%d host=%d audio=%d of %d", uv, host, audio, len(g.Entries))
	}
}

func TestExplicitRequiresReplacesTheDerivedOne(t *testing.T) {
	s, root, _ := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("a", "A", `"requires":["b"]`)+`,`+entry("b", "B")+`]}`)
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.Entries["a"].Requires, ","); got != "b" {
		t.Errorf("a requires %q", got)
	}
	if got := strings.Join(g.Entries["b"].Requires, ","); got != "gradio-host" {
		t.Errorf("b requires %q", got)
	}
}

func TestBadRequiresIsALoadErrorNamingEntryAndName(t *testing.T) {
	s, root, _ := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("a", "A", `"requires":["a b"]`)+`]}`)
	if _, err := s.RequiresGraph(context.Background()); err == nil || !strings.Contains(err.Error(), `a: requires: "a b"`) {
		t.Fatalf("got %v", err)
	}
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("uv", "Fake uv")+`]}`)
	if _, err := s.RequiresGraph(context.Background()); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved: %v", err)
	}
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("a", "A", `"requires":["nowhere"]`)+`]}`)
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Check("a"); err == nil || !strings.Contains(err.Error(), `a: requires "nowhere"`) {
		t.Fatalf("unknown: %v", err)
	}
}

// An entry that requires another entry installs it first, in order; removing
// the required one is refused while the other is installed.
func TestInstallsRequiredEntriesFirstAndRefusesRemovingThem(t *testing.T) {
	s, root, log := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("top", "Top", `"requires":["base"]`)+`,`+entry("base", "Base")+`]}`)
	code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"top"}`, "tok")
	if code != 200 {
		t.Fatalf("install: %d %s", code, body)
	}
	for _, want := range []string{"Installing base (step 1 of 2)", "Installing top (step 2 of 2)"} {
		if !strings.Contains(body, want) {
			t.Errorf("stream lacks %q:\n%s", want, body)
		}
	}
	l := readLog(t, log)
	if len(l) != 2 || !strings.Contains(l[0], "example.com/base@main") || !strings.Contains(l[1], "example.com/top@main") {
		t.Fatalf("commands in order: %q", l)
	}

	// Removal: both own-venv entries, installed (their venv directories exist).
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("top", "Top", `"proxy":"native","requires":["base"]`)+`,`+entry("base", "Base", `"proxy":"native"`)+`]}`)
	s.opts.UV = uvStandIn(t, root)
	markUVInstalled(t, root, "CPU")
	for _, d := range []string{"top", "base"} {
		finishedVenv(t, root, d)
	}
	code, body = call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"base"}`, "tok")
	if code != 400 || !strings.Contains(body, "cannot remove base: required by top") {
		t.Fatalf("uninstall base: %d %s", code, body)
	}
	if code, body = call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"top"}`, "tok"); code != 200 {
		t.Fatalf("uninstall top: %d %s", code, body)
	}
}

func TestPastedEntryRequiresAreChecked(t *testing.T) {
	s, _, _ := installStore(t)
	bad := strings.Replace(goodEntry, `"package_name"`, `"requires": ["x y"], "package_name"`, 1)
	code, body := call(t, s.Handler(), "POST", Prefix+"install-entry", `{"entry":`+bad+`}`, "tok")
	if code != 400 || !strings.Contains(body, "requires") {
		t.Fatalf("bad requires: %d %s", code, body)
	}
}
