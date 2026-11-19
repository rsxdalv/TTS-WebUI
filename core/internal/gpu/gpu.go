// Package gpu holds the GPU labels the installer understands.
package gpu

import (
	"fmt"
	"strings"
)

// Label is the GPU kind string understood by the TTS WebUI installer.
// These must match the PyTorchChoice values written to installer_scripts/.gpu.
type Label string

const (
	NVIDIA Label = "NVIDIA GPU"
	AMD    Label = "AMD GPU (ROCM, Linux only)"
	Apple  Label = "Apple M Series Chip"
	Intel  Label = "Intel GPU (XPU)"
	CPU    Label = "CPU"
	Custom Label = "Custom (Manual torch install)"
)

var allLabels = []Label{NVIDIA, AMD, Apple, Intel, CPU, Custom}

// Parse returns the Label for s, or an error if s is not a valid label.
// An unknown label is never silently accepted.
func Parse(s string) (Label, error) {
	s = strings.TrimSpace(s)
	for _, l := range allLabels {
		if string(l) == s {
			return l, nil
		}
	}
	return "", fmt.Errorf("unknown GPU label %q: valid labels are NVIDIA GPU, AMD GPU (ROCM, Linux only), Apple M Series Chip, Intel GPU (XPU), CPU, Custom (Manual torch install)", s)
}
