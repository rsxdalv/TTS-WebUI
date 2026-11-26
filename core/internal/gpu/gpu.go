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
	// NVIDIALegacy is an NVIDIA card older than Turing (compute capability below 7.5:
	// GTX 10xx Pascal, GTX 9xx Maxwell, Titan V Volta). Only the uv platform knows it:
	// it installs torch cu126, the last CUDA build with kernels for those cards. The
	// legacy installer (installer_scripts/.gpu) has no such choice.
	NVIDIALegacy Label = "NVIDIA GPU, older than Turing (GTX 10xx, 9xx, Volta)"
	AMD          Label = "AMD GPU (ROCM, Linux only)"
	Apple        Label = "Apple M Series Chip"
	Intel        Label = "Intel GPU (XPU)"
	CPU          Label = "CPU"
	Custom       Label = "Custom (Manual torch install)"
)

var allLabels = []Label{NVIDIA, NVIDIALegacy, AMD, Apple, Intel, CPU, Custom}

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

// MinCUDAComputeCap is the first compute capability the cu128 torch build has
// kernels for (Turing, sm_75). Below it the build still reports
// torch.cuda.is_available() == True, then fails on the first kernel.
const MinCUDAComputeCap = 7.5

// ComputeCapBelowCU128 reports whether a compute capability string such as "6.1"
// is older than cu128 supports. A string that is not a number is not "older".
func ComputeCapBelowCU128(cap string) bool {
	var major, minor int
	if n, err := fmt.Sscanf(strings.TrimSpace(cap), "%d.%d", &major, &minor); n != 2 || err != nil {
		return false
	}
	return float64(major)+float64(minor)/10 < MinCUDAComputeCap
}

// legacyNameMarks are fallbacks when nvidia-smi is too old to answer
// compute_cap: product-name fragments of cards below Turing.
var legacyNameMarks = []string{"GTX 10", "GTX 9", "GTX 7", "GTX TITAN", "TITAN X", "TITAN V", "TITAN XP", "GT 10", "GT 7",
	"QUADRO P", "QUADRO M", "QUADRO GP", "QUADRO GV", "TESLA P", "TESLA M", "TESLA V100", "TESLA K", "MX1", "MX2", "MX3"}

// NameBelowCU128 reports whether a product name is a card older than Turing. It is
// only the fallback; the compute capability is the measurement. GTX 16xx are Turing.
func NameBelowCU128(name string) bool {
	u := strings.ToUpper(name)
	for _, m := range legacyNameMarks {
		if strings.Contains(u, m) {
			return true
		}
	}
	return false
}

// Family names the architecture of a compute capability for the reason line.
func Family(cap string) string {
	switch {
	case strings.HasPrefix(cap, "6."):
		return "Pascal"
	case strings.HasPrefix(cap, "5."):
		return "Maxwell"
	case strings.HasPrefix(cap, "7.0"):
		return "Volta"
	case strings.HasPrefix(cap, "3."):
		return "Kepler"
	}
	return "older than Turing"
}
