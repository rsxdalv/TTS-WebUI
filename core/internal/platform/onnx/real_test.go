package onnx

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// TestRealInstallAndGenerate downloads the pinned runtime and the Kitten model
// (about 56 MB) into ONNX_REAL_ROOT, a scratch folder, and generates a clip with
// the real program. Opt-in: it needs the network.
func TestRealInstallAndGenerate(t *testing.T) {
	dir := os.Getenv("ONNX_REAL_ROOT")
	if dir == "" {
		t.Skip("set ONNX_REAL_ROOT to a scratch folder to download and run the real runtime")
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := New(Options{Root: root})
	ctx := context.Background()
	emit := func(l string) { t.Log(l) }
	if err := p.Install(ctx, emit); err != nil {
		t.Fatal(err)
	}
	e := platform.Entry{Package: "tts_webui_onnx.kitten_nano"}
	if !p.Create(ctx, e, emit) {
		t.Fatal("create failed")
	}
	wav, err := p.Generate(ctx, Request{Package: e.Package, Text: "Hello from the ONNX platform.", Voice: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(wav[:4]) != "RIFF" || len(wav) < 20000 {
		t.Fatalf("not a real clip: %d bytes", len(wav))
	}
	// Not silence: some 16-bit sample is far from zero.
	loud := false
	for i := 44; i+1 < len(wav); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(wav[i:])); v > 2000 || v < -2000 {
			loud = true
			break
		}
	}
	if !loud {
		t.Fatal("the clip is silent")
	}
	// Cancel a long generation: the process stops and no file is left.
	long := ""
	for i := 0; i < 60; i++ {
		long += "This is a long sentence that takes a while to speak. "
	}
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := p.Generate(cctx, Request{Package: e.Package, Text: long}); done <- err }()
	time.Sleep(1500 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled generation returned a clip")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("generation did not stop")
	}
	if left, _ := os.ReadDir(filepath.Join(p.Dir(), "tmp")); len(left) != 0 {
		t.Fatalf("files left after cancel: %v", left)
	}
	if !p.Remove(ctx, e, emit) || p.EntryStatus(ctx, e).Installed {
		t.Fatal("remove failed")
	}
}
