// Package gpu detects the GPU type and returns a label that matches
// the installer's PyTorchChoice values.
package gpu

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Label is the GPU kind string understood by the TTS WebUI installer.
// These must match the PyTorchChoice values written by ignition to the .gpu file.
type Label string

const (
	NVIDIA Label = "NVIDIA GPU"
	AMD    Label = "AMD GPU (ROCM, Linux only)"
	Apple  Label = "Apple M Series Chip"
	Intel  Label = "Intel GPU (XPU)"
	CPU    Label = "CPU"
	Custom Label = "Custom (Manual torch install)" // user choice, not auto-detected
)

// allLabels is the full set of valid PyTorchChoice labels for Parse validation.
var allLabels = []Label{NVIDIA, AMD, Apple, Intel, CPU, Custom}

// Parse returns the Label for s, or an error if s is not a valid PyTorchChoice string.
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

// Result holds the detected label and an optional human-readable note.
type Result struct {
	Label Label
	Note  string
}

// commandRunner executes a command and returns (stdout, exitCode, error).
// A non-zero exit code is not an error; only execution failures (not-found) are.
type commandRunner func(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)

func defaultRunner(ctx context.Context, name string, args ...string) (string, int, error) {
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode(), nil
		}
		return "", -1, fmt.Errorf("running %q: %w", name, err)
	}
	return string(out), 0, nil
}

// Detect detects the GPU and returns the installer label.
// Detection failures fall back to Label "CPU" with a note; they are not errors.
func Detect(ctx context.Context) (Result, error) {
	return detect(ctx, defaultRunner, runtime.GOOS, runtime.GOARCH)
}

// DetectWith is the testable variant of Detect that accepts an injected command
// runner and explicit OS/arch strings.
func DetectWith(ctx context.Context, run commandRunner, goos, goarch string) (Result, error) {
	return detect(ctx, run, goos, goarch)
}

func detect(ctx context.Context, run commandRunner, goos, goarch string) (Result, error) {
	// 1. NVIDIA — any platform
	if _, code, err := run(ctx, "nvidia-smi"); err == nil && code == 0 {
		return Result{Label: NVIDIA}, nil
	}

	// 2. Apple Silicon — Darwin arm64
	if goos == "darwin" && goarch == "arm64" {
		return Result{Label: Apple}, nil
	}

	// 3. AMD ROCm — Linux only
	if goos == "linux" {
		if amd, note := detectAMD(ctx, run); amd {
			return Result{Label: AMD, Note: note}, nil
		}
	}

	// 4. Intel XPU — any platform
	if intel, note := detectIntel(ctx, run, goos); intel {
		return Result{Label: Intel, Note: note}, nil
	}

	// 5. Fallback
	return Result{Label: CPU, Note: "no discrete GPU detected"}, nil
}

func detectAMD(ctx context.Context, run commandRunner) (bool, string) {
	if _, code, err := run(ctx, "rocm-smi"); err == nil && code == 0 {
		return true, "rocm-smi found"
	}
	if dirExists("/opt/rocm") {
		return true, "/opt/rocm exists"
	}
	return false, ""
}

func detectIntel(ctx context.Context, run commandRunner, goos string) (bool, string) {
	if _, code, err := run(ctx, "xpu-smi"); err == nil && code == 0 {
		return true, "xpu-smi found"
	}
	if goos == "linux" && hasIntelDGSysfs() {
		return true, "Intel DG GPU in /sys/class/drm"
	}
	return false, ""
}

// hasIntelDGSysfs checks /sys/class/drm for an Intel (0x8086) DG-class device.
func hasIntelDGSysfs() bool {
	for i := 0; i < 8; i++ {
		vendor := fmt.Sprintf("/sys/class/drm/card%d/device/vendor", i)
		class := fmt.Sprintf("/sys/class/drm/card%d/device/class", i)
		v := strings.TrimSpace(readSysFile(vendor))
		c := strings.TrimSpace(readSysFile(class))
		if strings.EqualFold(v, "0x8086") && isDGClass(c) {
			return true
		}
	}
	return false
}
