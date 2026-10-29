package env

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func TestTorchIndexURL(t *testing.T) {
	tests := []struct {
		gpu     string
		wantURL string
		wantOK  bool
	}{
		{"nvidia", "https://download.pytorch.org/whl/cu126", true},
		{"amd", "https://download.pytorch.org/whl/rocm6.2", true},
		{"apple", "", true},
		{"intel_extension_for_pytorch", "", true},
		{"cpu", "https://download.pytorch.org/whl/cpu", true},
		{"unknown_gpu", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.gpu, func(t *testing.T) {
			got, ok := TorchIndexURL(tt.gpu)
			if ok != tt.wantOK {
				t.Errorf("TorchIndexURL(%q) ok=%v, want %v", tt.gpu, ok, tt.wantOK)
			}
			if got != tt.wantURL {
				t.Errorf("TorchIndexURL(%q) url=%q, want %q", tt.gpu, got, tt.wantURL)
			}
		})
	}
}

func makeRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCreateMain_CallSequence(t *testing.T) {
	var calls [][]string
	fakeRun := func(_ context.Context, bin string, args []string, _ []string) ([]byte, error) {
		calls = append(calls, append([]string{bin}, args...))
		return nil, nil
	}

	root := makeRoot(t)
	if err := createMain(context.Background(), root, "nvidia", fakeRun); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 uv calls, got %d: %v", len(calls), calls)
	}
	if calls[0][1] != "python" {
		t.Errorf("call[0]: expected 'python', got %q", calls[0][1])
	}
	if calls[1][1] != "venv" {
		t.Errorf("call[1]: expected 'venv', got %q", calls[1][1])
	}
	thirdCall := strings.Join(calls[2], " ")
	if !strings.Contains(thirdCall, "cu126") {
		t.Errorf("nvidia pip install call missing cu126: %v", calls[2])
	}
}

func TestCreateMain_CPUNoIndexURL(t *testing.T) {
	var calls [][]string
	fakeRun := func(_ context.Context, bin string, args []string, _ []string) ([]byte, error) {
		calls = append(calls, append([]string{bin}, args...))
		return nil, nil
	}

	root := makeRoot(t)
	if err := createMain(context.Background(), root, "cpu", fakeRun); err != nil {
		t.Fatal(err)
	}

	thirdCall := strings.Join(calls[2], " ")
	if !strings.Contains(thirdCall, "download.pytorch.org/whl/cpu") {
		t.Errorf("cpu pip install call missing whl/cpu: %v", calls[2])
	}
}

func TestCreateMain_AppleNoIndexURL(t *testing.T) {
	var calls [][]string
	fakeRun := func(_ context.Context, bin string, args []string, _ []string) ([]byte, error) {
		calls = append(calls, append([]string{bin}, args...))
		return nil, nil
	}

	root := makeRoot(t)
	if err := createMain(context.Background(), root, "apple", fakeRun); err != nil {
		t.Fatal(err)
	}

	thirdCall := strings.Join(calls[2], " ")
	if strings.Contains(thirdCall, "--index-url") {
		t.Errorf("apple should not have --index-url, got: %v", calls[2])
	}
}

func TestCreateMain_Intel(t *testing.T) {
	var calls [][]string
	fakeRun := func(_ context.Context, bin string, args []string, _ []string) ([]byte, error) {
		calls = append(calls, append([]string{bin}, args...))
		return nil, nil
	}

	root := makeRoot(t)
	if err := createMain(context.Background(), root, "intel_extension_for_pytorch", fakeRun); err != nil {
		t.Fatal(err)
	}

	thirdCall := strings.Join(calls[2], " ")
	if !strings.Contains(thirdCall, "intel-extension-for-pytorch") {
		t.Errorf("intel call missing intel-extension-for-pytorch: %v", calls[2])
	}
}

func TestCreateMain_UVCacheDir(t *testing.T) {
	root := makeRoot(t)
	wantCacheDir := filepath.Join(root.InstallerFiles(), "uvcache")

	fakeRun := func(_ context.Context, _ string, _ []string, env []string) ([]byte, error) {
		for _, kv := range env {
			if kv == "UV_CACHE_DIR="+wantCacheDir {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("UV_CACHE_DIR not set to %q in env", wantCacheDir)
	}

	if err := createMain(context.Background(), root, "cpu", fakeRun); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMain_CopyWarning(t *testing.T) {
	fakeRun := func(_ context.Context, _ string, _ []string, _ []string) ([]byte, error) {
		return []byte("Copying instead of linking"), nil
	}
	root := makeRoot(t)
	// Should not return error, only warn.
	if err := createMain(context.Background(), root, "cpu", fakeRun); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
