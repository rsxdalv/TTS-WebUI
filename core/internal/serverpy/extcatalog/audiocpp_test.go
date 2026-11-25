package extcatalog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
)

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// audioStore is a Store whose audiocpp platform downloads a fake audio.cpp
// archive and one fake model from a local TLS server.
func audioStore(t *testing.T, badArchiveSum bool) (*Store, string, *audiocpp.Platform) {
	t.Helper()
	s, root, _ := installStore(t)
	name := "audiocpp_server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, _ := zw.Create(name)
	w.Write([]byte("not a real server"))
	zw.Close()
	gguf := []byte(strings.Repeat("g", 4000))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			w.Write(zb.Bytes())
			return
		}
		w.Write(gguf)
	}))
	t.Cleanup(srv.Close)
	archSum := sha(zb.Bytes())
	if badArchiveSum {
		archSum = strings.Repeat("0", 64)
	}
	pr, _ := paths.New(root)
	p := audiocpp.New(audiocpp.Options{Root: pr, Client: srv.Client(),
		Archive: &audiocpp.Archive{URL: srv.URL + "/audio.zip", SHA256: archSum},
		Models: []audiocpp.Model{{Package: "audiocpp_fake", ID: "fake", Dir: "fake", GGUF: "m.gguf",
			Files: []audiocpp.File{{Rel: "m.gguf", URL: srv.URL + "/m.gguf", SHA256: sha(gguf), Size: int64(len(gguf))}}}}})
	s.opts.AudioCpp = p
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("audiocpp_fake", "Fake Model", `"requires":["audiocpp"]`)+`]}`)
	return s, root, p
}

func TestAudioModelInstallInstallsThePlatformFirstWithTheChosenBuild(t *testing.T) {
	s, root, p := audioStore(t, false)
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st := g.State("audiocpp_fake"); len(st) != 1 || st[0].State != "missing" || !st[0].Installable || !strings.Contains(st[0].Would, "sha256") {
		t.Fatalf("requirement state = %+v", st)
	}
	code, out, raw := stream(t, s, `install {"package_name":"audiocpp_fake","audiocpp_row":"vulkan"}`)
	if code != 200 || !out.ok {
		t.Fatalf("install: %d %s", code, raw)
	}
	log := strings.Join(out.lines, "\n")
	for _, want := range []string{"Installing the audiocpp platform (step 1 of 2)", "Installing the vulkan build", "Checksum matches", "Installing audiocpp_fake (step 2 of 2)", "is installed"} {
		if !strings.Contains(log, want) {
			t.Errorf("no %q in:\n%s", want, log)
		}
	}
	if row := p.RowName(); row != "vulkan" {
		t.Errorf("row = %q", row)
	}
	g, _ = s.RequiresGraph(context.Background())
	if !g.Entries["audiocpp_fake"].Installed {
		t.Error("the model is not installed")
	}
	if st := g.State("audiocpp_fake"); st[0].State != "ready" || st[0].Via != "audiocpp" {
		t.Errorf("requirement state = %+v", st)
	}
	for _, f := range []string{"audiocpp-platform-install-audiocpp.log", "audiocpp-model-install-fake_Model.log"} {
		if _, err := os.Stat(filepath.Join(root, "installer_scripts", "logs", strings.ReplaceAll(f, "fake_Model", "Fake Model"))); err != nil {
			t.Errorf("log %s: %v", f, err)
		}
	}
}

func TestAudioInstallRefusesABadRowAndARowThatIsNotUsed(t *testing.T) {
	s, _, p := audioStore(t, false)
	if code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"audiocpp_fake","audiocpp_row":"cuda"}`, "tok"); code != 400 || !strings.Contains(body, "cpu, vulkan") {
		t.Errorf("bad row: %d %s", code, body)
	}
	if _, err := os.Stat(p.Dir()); err == nil {
		t.Error("something was made for a refused install")
	}
	if code, out, _ := stream(t, s, `install {"package_name":"audiocpp_fake"}`); code != 200 || !out.ok {
		t.Fatal("install without a row (the CPU build)")
	}
	if p.RowName() != "cpu" || !strings.Contains(readAll(t, p.Dir()), "cpu") {
		t.Errorf("row = %q", p.RowName())
	}
	if code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"audiocpp_fake","audiocpp_row":"vulkan"}`, "tok"); code != 400 {
		t.Errorf("a row with no platform step: %d %s", code, body)
	}
}

func readAll(t *testing.T, dir string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(dir, "platform.json"))
	return string(b)
}

func TestAudioInstallWithAWrongChecksumInstallsNothing(t *testing.T) {
	s, _, p := audioStore(t, true)
	code, out, raw := stream(t, s, `install {"package_name":"audiocpp_fake"}`)
	if code != 200 || out.ok {
		t.Fatalf("a wrong checksum was accepted: %d %s", code, raw)
	}
	log := strings.Join(out.lines, "\n")
	if !strings.Contains(log, "checksum mismatch") || !strings.Contains(log, "Stopped: step 1 of 2") {
		t.Errorf("log:\n%s", log)
	}
	if p.Status(context.Background()).Ready {
		t.Error("platform ready")
	}
	if _, err := os.Stat(filepath.Join(p.Dir(), "bin")); err == nil {
		t.Error("bin/ exists")
	}
	g, _ := s.RequiresGraph(context.Background())
	if g.Entries["audiocpp_fake"].Installed {
		t.Error("the model counts as installed")
	}
}

func TestAudioRemovalOrderPlatformIsRefusedWhileAModelNeedsIt(t *testing.T) {
	s, _, p := audioStore(t, false)
	if code, out, _ := stream(t, s, `install {"package_name":"audiocpp_fake"}`); code != 200 || !out.ok {
		t.Fatal("install")
	}
	code, body := call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"audiocpp"}`, "tok")
	if code != 400 || !strings.Contains(body, "required by audiocpp_fake") {
		t.Fatalf("platform removal with a model: %d %s", code, body)
	}
	if code, out, _ := stream(t, s, `uninstall {"package_name":"audiocpp_fake"}`); code != 200 || !out.ok {
		t.Fatal("model uninstall")
	}
	if g, _ := s.RequiresGraph(context.Background()); g.Entries["audiocpp_fake"].Installed {
		t.Error("the model is still installed")
	}
	if !p.Status(context.Background()).Ready {
		t.Error("removing the model removed the platform")
	}
	if code, out, raw := stream(t, s, `uninstall {"package_name":"audiocpp"}`); code != 200 || !out.ok {
		t.Fatalf("platform uninstall: %d %s", code, raw)
	}
	if _, err := os.Stat(p.Dir()); err == nil {
		t.Error("platforms/audiocpp is still there")
	}
}

func TestAudioPlatformHasNoCheckAndAnUnsupportedOSIsNotOffered(t *testing.T) {
	s, root, _ := audioStore(t, false)
	if code, _ := call(t, s.Handler(), "POST", Prefix+"check-dependencies", `{"package_name":"audiocpp"}`, "tok"); code != 400 {
		t.Errorf("check-dependencies: %d", code)
	}
	// A platform with no archive for this OS (no override): the requirement is
	// blocked with the reserved text, not offered.
	pr, _ := paths.New(root)
	s.opts.AudioCpp = audiocpp.New(audiocpp.Options{Root: pr, GOOS: "plan9", GOARCH: "mips"})
	code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"audiocpp_fake"}`, "tok")
	if code != 400 || !strings.Contains(body, "not available here") {
		t.Errorf("unsupported: %d %s", code, body)
	}
}
