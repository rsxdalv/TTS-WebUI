package gpu_test

import (
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

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
