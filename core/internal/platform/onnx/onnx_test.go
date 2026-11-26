package onnx

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// The test binary is the stand-in for the runtime's program: with ONNX_FAKE set it
// writes a WAV to --output-filename, or sleeps when the text is "slow".
func TestMain(m *testing.M) {
	if os.Getenv("ONNX_FAKE") == "1" {
		var out, text string
		for _, a := range os.Args[1:] {
			if strings.HasPrefix(a, "--output-filename=") {
				out = strings.TrimPrefix(a, "--output-filename=")
			} else if !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "-test") {
				text = a
			}
		}
		if text == "slow" {
			_ = os.WriteFile(out+".partial", []byte("x"), 0o644)
			time.Sleep(time.Minute)
		}
		_ = os.WriteFile(out, fakeWav(), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeWav() []byte {
	var b bytes.Buffer
	data := make([]byte, 3200)
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

func archive(top string, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: top + "/", Typeflag: tar.TypeDir, Mode: 0o755})
	for name, body := range files {
		_ = tw.WriteHeader(&tar.Header{Name: top + "/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		_, _ = tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type fixture struct {
	p      *Platform
	rt, md []byte
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{}
	f.rt = archive("rt", map[string]string{"bin/sherpa-onnx-offline-tts.exe": "x", "bin/sherpa-onnx-offline-tts": "x"})
	f.md = archive("m", map[string]string{"model.int8.onnx": "m", "voices.bin": "v", "tokens.txt": "t", "espeak-ng-data/en_dict": "d"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rt":
			_, _ = w.Write(f.rt)
		case "/model":
			_, _ = w.Write(f.md)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	exe, _ := os.Executable()
	t.Setenv("ONNX_FAKE", "1")
	root, _ := paths.New(t.TempDir())
	f.p = New(Options{Root: root,
		Runtime: &Archive{URL: srv.URL + "/rt", SHA256: sum(f.rt)},
		Models: []Model{{Package: "tts.fake", Dir: "fake", Title: "Fake", Kind: "kitten", Voices: 4,
			Archive: Archive{URL: srv.URL + "/model", SHA256: sum(f.md)},
			Files:   []string{"model.int8.onnx", "voices.bin", "tokens.txt", "espeak-ng-data"}}},
		Program: []string{exe}})
	return f
}

func collect() (platform.Emit, *[]string) {
	var l []string
	return func(s string) { l = append(l, s) }, &l
}

func TestInstallEntryGenerateRemove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if f.p.Status(ctx).Ready {
		t.Fatal("ready before install")
	}
	emit, log := collect()
	if err := f.p.Install(ctx, emit); err != nil {
		t.Fatal(err, *log)
	}
	if !f.p.Status(ctx).Ready {
		t.Fatal("not ready after install")
	}
	e := platform.Entry{Package: "tts.fake"}
	if f.p.EntryStatus(ctx, e).Installed {
		t.Fatal("entry installed before create")
	}
	if !f.p.Create(ctx, e, emit) {
		t.Fatal(*log)
	}
	if !f.p.EntryStatus(ctx, e).Installed {
		t.Fatal("entry not installed")
	}
	wav, err := f.p.Generate(ctx, Request{Package: "tts.fake", Text: "hello", Voice: 1, Speed: 1.5})
	if err != nil || string(wav[:4]) != "RIFF" {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(filepath.Join(f.p.Dir(), "tmp")); len(left) != 0 {
		t.Fatalf("temporary clip left: %v", left)
	}
	if !f.p.Remove(ctx, e, emit) || f.p.EntryStatus(ctx, e).Installed || !f.p.Status(ctx).Ready {
		t.Fatal("remove should drop the entry and keep the platform")
	}
	if _, err := os.Stat(f.p.ModelDir(f.p.models[0])); err == nil {
		t.Fatal("model folder still there")
	}
}

func TestBadChecksumLeavesNothing(t *testing.T) {
	f := newFixture(t)
	f.p.runtime.SHA256 = strings.Repeat("0", 64)
	emit, _ := collect()
	if err := f.p.Install(context.Background(), emit); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	if _, err := os.Stat(f.p.RuntimeDir()); err == nil {
		t.Fatal("runtime folder exists after a bad checksum")
	}
	if _, ok := f.p.Info(); ok || f.p.Status(context.Background()).Ready {
		t.Fatal("platform marked installed")
	}
	ents, _ := os.ReadDir(f.p.Dir())
	for _, e := range ents {
		if strings.Contains(e.Name(), "download") || strings.Contains(e.Name(), ".new") || strings.Contains(e.Name(), ".part") {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}

func TestBadModelChecksumLeavesNoEntry(t *testing.T) {
	f := newFixture(t)
	emit, log := collect()
	if err := f.p.Install(context.Background(), emit); err != nil {
		t.Fatal(err)
	}
	f.p.models[0].Archive.SHA256 = strings.Repeat("1", 64)
	if f.p.Create(context.Background(), platform.Entry{Package: "tts.fake"}, emit) {
		t.Fatal("create succeeded")
	}
	if st := f.p.EntryStatus(context.Background(), platform.Entry{Package: "tts.fake"}); st.Installed || st.Partial {
		t.Fatalf("status %+v", st)
	}
	if !strings.Contains(strings.Join(*log, "\n"), "checksum mismatch") {
		t.Fatal(*log)
	}
}

func TestCancelDuringDownloadThenAgain(t *testing.T) {
	f := newFixture(t)
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write(f.rt[:10])
		w.(http.Flusher).Flush()
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(stop)
	good := f.p.runtime.URL
	f.p.runtime.URL = srv.URL
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	emit, _ := collect()
	if err := f.p.Install(ctx, emit); err == nil {
		t.Fatal("cancelled install returned nil")
	}
	if f.p.Status(context.Background()).Ready {
		t.Fatal("ready after a cancel")
	}
	if _, err := os.Stat(f.p.RuntimeDir()); err == nil {
		t.Fatal("runtime folder after a cancel")
	}
	f.p.runtime.URL = good
	if err := f.p.Install(context.Background(), emit); err != nil || !f.p.Status(context.Background()).Ready {
		t.Fatalf("second install: %v", err)
	}
}

func TestCancelGenerationKillsProcessAndLeavesNoFile(t *testing.T) {
	f := newFixture(t)
	emit, _ := collect()
	ctx := context.Background()
	if err := f.p.Install(ctx, emit); err != nil || !f.p.Create(ctx, platform.Entry{Package: "tts.fake"}, emit) {
		t.Fatal("setup")
	}
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := f.p.Generate(cctx, Request{Package: "tts.fake", Text: "slow"}); done <- err }()
	time.Sleep(700 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled generation returned a clip")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("generation did not stop after cancel")
	}
	if left, _ := os.ReadDir(filepath.Join(f.p.Dir(), "tmp")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestRequestValidation(t *testing.T) {
	f := newFixture(t)
	for name, r := range map[string]Request{
		"empty":   {Package: "tts.fake", Text: "  "},
		"dash":    {Package: "tts.fake", Text: "--help"},
		"voice":   {Package: "tts.fake", Text: "x", Voice: 9},
		"speed":   {Package: "tts.fake", Text: "x", Speed: 99},
		"unknown": {Package: "nope", Text: "x"},
		"long":    {Package: "tts.fake", Text: strings.Repeat("a", MaxText+1)},
	} {
		if _, err := f.p.Generate(context.Background(), r); err == nil || !strings.Contains(err.Error(), ErrInvalid.Error()) {
			t.Errorf("%s: want invalid request, got %v", name, err)
		}
	}
}

func TestArchiveRefusesPathEscape(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "top/../../evil", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
	_, _ = tw.Write([]byte("x"))
	tw.Close()
	gz.Close()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tar.gz")
	_ = os.WriteFile(a, buf.Bytes(), 0o644)
	out := filepath.Join(dir, "out")
	if err := extractTarBz2(context.Background(), a, out); err == nil {
		t.Fatal("path escape accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Fatal("file written outside")
	}
}

func TestOtherOSRefused(t *testing.T) {
	if _, err := ArchiveFor("linux", "amd64"); err == nil || !strings.Contains(err.Error(), "Windows x64 only") {
		t.Fatal(err)
	}
}
