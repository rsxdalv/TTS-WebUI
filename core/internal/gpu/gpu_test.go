package gpu_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

// fakeRunner returns results from a pre-seeded map of command→(output, code).
// An unknown command returns an error simulating "command not found".
type fakeRunner struct {
	cmds map[string]struct {
		out  string
		code int
	}
}

func (f *fakeRunner) run(ctx context.Context, name string, args ...string) (string, int, error) {
	if e, ok := f.cmds[name]; ok {
		return e.out, e.code, nil
	}
	return "", -1, fmt.Errorf("running %q: %w", name, errors.New("executable file not found"))
}

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("loading fixture %q: %v", name, err)
	}
	return string(data)
}

func TestDetect_Table(t *testing.T) {
	nvidiaOut := loadFixture(t, "nvidia-smi-found.txt")
	rocmOut := loadFixture(t, "rocm-smi-found.txt")

	cases := []struct {
		name string
		cmds map[string]struct {
			out  string
			code int
		}
		goos   string
		goarch string
		want   gpu.Label
	}{
		{
			name: "nvidia wins on any platform",
			cmds: map[string]struct {
				out  string
				code int
			}{
				"nvidia-smi": {out: nvidiaOut, code: 0},
			},
			goos: "linux", goarch: "amd64",
			want: gpu.NVIDIA,
		},
		{
			name: "apple arm64 darwin",
			cmds: map[string]struct {
				out  string
				code int
			}{},
			goos: "darwin", goarch: "arm64",
			want: gpu.Apple,
		},
		{
			name: "amd via rocm-smi on linux",
			cmds: map[string]struct {
				out  string
				code int
			}{
				"rocm-smi": {out: rocmOut, code: 0},
			},
			goos: "linux", goarch: "amd64",
			want: gpu.AMD,
		},
		{
			name: "intel via xpu-smi",
			cmds: map[string]struct {
				out  string
				code int
			}{
				"xpu-smi": {out: "Intel XPU", code: 0},
			},
			goos: "linux", goarch: "amd64",
			want: gpu.Intel,
		},
		{
			name: "cpu fallback",
			cmds: map[string]struct {
				out  string
				code int
			}{},
			goos: "linux", goarch: "amd64",
			want: gpu.CPU,
		},
		{
			name: "cpu fallback on windows",
			cmds: map[string]struct {
				out  string
				code int
			}{},
			goos: "windows", goarch: "amd64",
			want: gpu.CPU,
		},
		{
			name: "cpu fallback on darwin intel",
			cmds: map[string]struct {
				out  string
				code int
			}{},
			goos: "darwin", goarch: "amd64",
			want: gpu.CPU,
		},
		{
			name: "nvidia beats apple on darwin arm64",
			cmds: map[string]struct {
				out  string
				code int
			}{
				"nvidia-smi": {out: nvidiaOut, code: 0},
			},
			goos: "darwin", goarch: "arm64",
			want: gpu.NVIDIA,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRunner{cmds: tc.cmds}
			res, err := gpu.DetectWith(context.Background(), fr.run, tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("DetectWith: %v", err)
			}
			if res.Label != tc.want {
				t.Errorf("Label = %q, want %q (note: %q)", res.Label, tc.want, res.Note)
			}
		})
	}
}

func TestParse(t *testing.T) {
	valid := []struct {
		input string
		want  gpu.Label
	}{
		{"NVIDIA GPU", gpu.NVIDIA},
		{"AMD GPU (ROCM, Linux only)", gpu.AMD},
		{"Apple M Series Chip", gpu.Apple},
		{"Intel GPU (XPU)", gpu.Intel},
		{"CPU", gpu.CPU},
		{"Custom (Manual torch install)", gpu.Custom},
	}
	for _, tc := range valid {
		l, err := gpu.Parse(tc.input)
		if err != nil {
			t.Errorf("Parse(%q) error = %v, want nil", tc.input, err)
		}
		if l != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.input, l, tc.want)
		}
	}

	unknown := []string{"nvidia", "amd", "cpu", "intel_extension_for_pytorch", "unknown", ""}
	for _, s := range unknown {
		_, err := gpu.Parse(s)
		if err == nil {
			t.Errorf("Parse(%q) expected error, got nil", s)
		}
	}
}

func TestParseRealGPUFile(t *testing.T) {
	content := loadFixture(t, "gpu_real.txt")
	label, err := gpu.Parse(content)
	if err != nil {
		t.Fatalf("Parse of real .gpu file content %q: %v", content, err)
	}
	if label != gpu.NVIDIA {
		t.Errorf("label = %q, want %q", label, gpu.NVIDIA)
	}
}
