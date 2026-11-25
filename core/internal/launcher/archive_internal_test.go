package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, body)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The running launcher binary is never overwritten: its entry is written as
// <exe>.new and everything else lands in place.
func TestExtractWritesTheRunningBinaryBesideItself(t *testing.T) {
	dest := t.TempDir()
	exe := filepath.Join(dest, "tts-webui-test.exe")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := runningExe
	runningExe = func() string { return exe }
	defer func() { runningExe = old }()

	z := zipOf(t, map[string]string{"tts-webui-test.exe": "new", "react-ui/out/index.html": "<html>"})
	if err := extractZip(context.Background(), z, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Errorf("the running binary was overwritten: %q", b)
	}
	if b, _ := os.ReadFile(exe + pendingSuffix); string(b) != "new" {
		t.Errorf("%s = %q, want the new binary", pendingSuffix, b)
	}
	if _, err := os.Stat(filepath.Join(dest, "react-ui", "out", "index.html")); err != nil {
		t.Errorf("the other files were not extracted: %v", err)
	}
}

// SwapPendingBinary replaces the binary and runs the new one: here a copy of the
// test binary itself, told to run no tests, so the run ends with exit code 0.
func TestSwapPendingBinaryReplacesAndRuns(t *testing.T) {
	self, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Skip("cannot read the test binary")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "tts-webui-test.exe")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe+pendingSuffix, self, 0o755); err != nil {
		t.Fatal(err)
	}
	old := runningExe
	runningExe = func() string { return exe }
	defer func() { runningExe = old }()

	handled, code, err := SwapPendingBinary([]string{"-test.run=^$"})
	if err != nil || !handled || code != 0 {
		t.Fatalf("handled=%v code=%d err=%v, want handled, exit 0", handled, code, err)
	}
	if b, _ := os.ReadFile(exe); !bytes.Equal(b, self) {
		t.Error("the launcher was not replaced by the .new binary")
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old binary" {
		t.Errorf(".old = %q, want the replaced binary", b)
	}
	if _, err := os.Stat(exe + pendingSuffix); err == nil {
		t.Error("the .new file is still there")
	}
	// Nothing pending: not handled.
	if handled, _, _ := SwapPendingBinary(nil); handled {
		t.Error("handled with nothing pending")
	}
}
