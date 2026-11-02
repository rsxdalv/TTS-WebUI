package gpu_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

func TestTorchFor_PinnedVersions(t *testing.T) {
	cases := []struct {
		label    gpu.Label
		wantPkg  string // substring that must appear in at least one Pkg
		wantIdx  string // substring of IndexURL (or "" for PyPI)
		wantXFrm string // XFormers specifier or ""
	}{
		{gpu.NVIDIA, "torch==2.11.0+cu128", "cu128", "xformers==0.0.35"},
		{gpu.AMD, "torch==2.11.0", "rocm7.2", "xformers"},
		{gpu.Apple, "torch==2.11.0", "", ""},
		{gpu.Intel, "torch==2.11.0", "test/xpu", ""},
		{gpu.CPU, "torch==2.11.0+cpu", "cpu", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.label), func(t *testing.T) {
			spec, err := gpu.TorchFor(tc.label)
			if err != nil {
				t.Fatalf("TorchFor(%q) unexpected error: %v", tc.label, err)
			}
			found := false
			for _, p := range spec.Pkgs {
				if strings.Contains(p, tc.wantPkg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no pkg contains %q in %v", tc.wantPkg, spec.Pkgs)
			}
			if tc.wantIdx != "" && !strings.Contains(spec.IndexURL, tc.wantIdx) {
				t.Errorf("IndexURL %q does not contain %q", spec.IndexURL, tc.wantIdx)
			}
			if tc.wantIdx == "" && spec.IndexURL != "" {
				t.Errorf("IndexURL should be empty for %q, got %q", tc.label, spec.IndexURL)
			}
			if tc.wantXFrm != "" && !strings.Contains(spec.XFormers, tc.wantXFrm) {
				t.Errorf("XFormers %q does not contain %q", spec.XFormers, tc.wantXFrm)
			}
			if tc.wantXFrm == "" && spec.XFormers != "" {
				t.Errorf("XFormers should be empty for %q, got %q", tc.label, spec.XFormers)
			}
		})
	}
}

func TestTorchFor_Custom_ReturnsErrCustomManaged(t *testing.T) {
	_, err := gpu.TorchFor(gpu.Custom)
	if err == nil {
		t.Fatal("expected ErrCustomManaged, got nil")
	}
	if !errors.Is(err, gpu.ErrCustomManaged) {
		t.Errorf("error should be ErrCustomManaged, got: %v", err)
	}
}

func TestTorchFor_Unknown_ReturnsError(t *testing.T) {
	_, err := gpu.TorchFor("some-unknown-label")
	if err == nil {
		t.Fatal("expected error for unknown label, got nil")
	}
	if errors.Is(err, gpu.ErrCustomManaged) {
		t.Error("unknown label should not return ErrCustomManaged")
	}
}
