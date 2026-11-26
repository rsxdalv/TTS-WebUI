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

func TestOlderThanTuring(t *testing.T) {
	for cap, want := range map[string]bool{"6.1": true, "5.2": true, "7.0": true, "7.5": false, "8.9": false, "12.0": false, "": false, "x": false} {
		if gpu.ComputeCapBelowCU128(cap) != want {
			t.Errorf("cap %q", cap)
		}
	}
	for name, want := range map[string]bool{"NVIDIA GeForce GTX 1070": true, "NVIDIA GeForce GTX 980 Ti": true, "Tesla P100-PCIE": true,
		"NVIDIA GeForce GTX 1660 Ti": false, "NVIDIA GeForce RTX 3060": false, "NVIDIA GeForce GTX 1650": false} {
		if gpu.NameBelowCU128(name) != want {
			t.Errorf("name %q", name)
		}
	}
	if _, err := gpu.Parse(string(gpu.NVIDIALegacy)); err != nil {
		t.Error(err)
	}
}
