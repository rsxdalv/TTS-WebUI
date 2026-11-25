package torchrows

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

// The arguments the three old copies produced for each GPU choice (extcatalog's
// torch table, as written on 2026-11-24), kept literally so that a change to the
// table shows up here.
func TestDefaultRowsMatchTheOldTable(t *testing.T) {
	cases := []struct {
		l    gpu.Label
		goos string
		want string
	}{
		{gpu.NVIDIA, "linux", "torch==2.11.0+cu128 torchvision torchaudio==2.11.0+cu128 --index-url https://download.pytorch.org/whl/cu128"},
		{gpu.NVIDIA, "windows", "torch==2.11.0+cu128 torchvision torchaudio==2.11.0+cu128 --index-url https://download.pytorch.org/whl/cu128"},
		{gpu.CPU, "windows", "torch==2.11.0+cpu torchvision torchaudio==2.11.0 --index-url https://download.pytorch.org/whl/cpu"},
		{gpu.AMD, "linux", "torch==2.11.0 torchvision torchaudio==2.11.0 --index-url https://download.pytorch.org/whl/rocm7.2"},
		{gpu.Intel, "linux", "torch==2.11.0 torchvision torchaudio==2.11.0 --index-url https://download.pytorch.org/whl/test/xpu"},
		{gpu.Apple, "darwin", "torch==2.11.0 torchvision torchaudio==2.11.0"},
		{gpu.Custom, "linux", "torch==2.11.0 torchvision torchaudio==2.11.0"},
	}
	for _, c := range cases {
		r, err := Default(c.l, c.goos)
		if err != nil {
			t.Fatalf("%s on %s: %v", c.l, c.goos, err)
		}
		if got := strings.Join(r.InstallArgs(), " "); got != c.want {
			t.Errorf("%s on %s:\n got %s\nwant %s", c.l, c.goos, got, c.want)
		}
	}
}

func TestOnlyNVIDIAGetsXformers(t *testing.T) {
	for _, r := range Rows("linux") {
		if (r.Xformers != "") != (r.Name == "cu128") {
			t.Errorf("row %s xformers %q", r.Name, r.Xformers)
		}
	}
}

func TestChoicesPerOS(t *testing.T) {
	if got := Choices("windows"); !reflect.DeepEqual(got, []gpu.Label{gpu.NVIDIA, gpu.Apple, gpu.Intel, gpu.CPU, gpu.Custom}) {
		// Apple's default row is pypi, which exists on every OS: the choice is
		// offered (and the detector only proposes it on Apple silicon).
		t.Errorf("windows: %v", got)
	}
	if _, err := Default(gpu.AMD, "windows"); err == nil {
		t.Error("AMD on Windows has no row and must say so")
	}
	if _, err := Default(gpu.CPU, "darwin"); err != nil {
		t.Errorf("CPU on macOS uses the PyPI row: %v", err)
	}
}

func TestUnknownRowIsAnErrorListingTheRows(t *testing.T) {
	_, err := ByName("cu999", "linux")
	if err == nil || !strings.Contains(err.Error(), "cu128") {
		t.Fatalf("got %v", err)
	}
}

func TestDetect(t *testing.T) {
	none := func(string) (string, error) { return "", errors.New("not found") }
	if d := Detect(context.Background(), "windows", "amd64", none); d.Label != gpu.CPU {
		t.Errorf("no tools: %v", d)
	}
	if d := Detect(context.Background(), "darwin", "arm64", none); d.Label != gpu.Apple {
		t.Errorf("apple: %v", d)
	}
	rocm := func(n string) (string, error) {
		if n == "rocm-smi" {
			return "rocm-smi", nil
		}
		return "", errors.New("not found")
	}
	if d := Detect(context.Background(), "linux", "amd64", rocm); d.Label != gpu.AMD {
		t.Errorf("rocm: %v", d)
	}
}

func TestCu126RowIsSelectableByNameAndNeverADefault(t *testing.T) {
	r, err := ByName("cu126", "windows")
	if err != nil || r.Xformers != "" || !strings.Contains(r.Packages[0], "+cu126") {
		t.Fatalf("cu126: %+v, %v", r, err)
	}
	for _, l := range []gpu.Label{gpu.NVIDIA, gpu.CPU, gpu.AMD, gpu.Intel} {
		if d, err := Default(l, "linux"); err == nil && d.Name == "cu126" {
			t.Fatalf("%s defaults to cu126", l)
		}
	}
}
