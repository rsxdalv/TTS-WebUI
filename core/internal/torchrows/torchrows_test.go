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
	if got := Choices("windows"); !reflect.DeepEqual(got, []gpu.Label{gpu.NVIDIA, gpu.NVIDIALegacy, gpu.Apple, gpu.Intel, gpu.CPU, gpu.Custom}) {
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

// stubSMI stands in for nvidia-smi: answers maps the --query-gpu field list to output.
func stubSMI(t *testing.T, answers map[string]string) {
	t.Helper()
	old := smiQuery
	smiQuery = func(_ context.Context, _, fields string) (string, error) {
		if out, ok := answers[fields]; ok {
			return out, nil
		}
		return "", errors.New("Field \"" + fields + "\" is not a valid field")
	}
	t.Cleanup(func() { smiQuery = old })
}

func TestDetectProposesCu126BelowTuring(t *testing.T) {
	smi := func(string) (string, error) { return "nvidia-smi", nil }
	cases := []struct {
		name    string
		answers map[string]string
		want    gpu.Label
	}{
		{"pascal by compute_cap", map[string]string{"name,compute_cap": "NVIDIA GeForce GTX 1070, 6.1\n"}, gpu.NVIDIALegacy},
		{"volta", map[string]string{"name,compute_cap": "NVIDIA TITAN V, 7.0\n"}, gpu.NVIDIALegacy},
		{"turing", map[string]string{"name,compute_cap": "NVIDIA GeForce RTX 2070, 7.5\n"}, gpu.NVIDIA},
		{"GTX 1660 is Turing", map[string]string{"name,compute_cap": "NVIDIA GeForce GTX 1660, 7.5\n"}, gpu.NVIDIA},
		{"ampere", map[string]string{"name,compute_cap": "NVIDIA GeForce RTX 3090, 8.6\n"}, gpu.NVIDIA},
		{"old driver, name says Pascal", map[string]string{"name": "NVIDIA GeForce GTX 1070\n"}, gpu.NVIDIALegacy},
		{"old driver, name says Turing", map[string]string{"name": "NVIDIA GeForce GTX 1660\n"}, gpu.NVIDIA},
		{"old driver, RTX", map[string]string{"name": "NVIDIA GeForce RTX 4070\n"}, gpu.NVIDIA},
	}
	for _, c := range cases {
		stubSMI(t, c.answers)
		if d := Detect(context.Background(), "windows", "amd64", smi); d.Label != c.want {
			t.Errorf("%s: got %v", c.name, d)
		}
	}
	stubSMI(t, map[string]string{"name,compute_cap": "NVIDIA GeForce GTX 1070, 6.1\n"})
	d := Detect(context.Background(), "windows", "amd64", smi)
	if !strings.Contains(d.Reason, "GTX 1070 (Pascal): cu128 has no kernels") {
		t.Errorf("reason: %q", d.Reason)
	}
}

func TestNVIDIALegacyInstallsCu126WithoutXformers(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		r, err := Default(gpu.NVIDIALegacy, goos)
		if err != nil || r.Name != "cu126" || r.Xformers != "" {
			t.Fatalf("%s: %+v, %v", goos, r, err)
		}
		if d, _ := Default(gpu.NVIDIA, goos); d.Name != "cu128" {
			t.Fatalf("NVIDIA is %s", d.Name)
		}
	}
	if _, err := Default(gpu.NVIDIALegacy, "darwin"); err == nil {
		t.Error("no cu126 on darwin")
	}
	found := 0
	for _, l := range Choices("windows") {
		if l == gpu.NVIDIA || l == gpu.NVIDIALegacy {
			found++
		}
	}
	if found != 2 {
		t.Errorf("choices %v", Choices("windows"))
	}
}
