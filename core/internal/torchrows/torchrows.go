// Package torchrows is the torch table (D29): the one place that says which torch,
// CUDA and xformers versions an install uses. A row is a torch build (a CUDA
// version, ROCm, CPU, ...). Every GPU choice has one default row per OS; an
// entry may name another row, and every extra row is several GB that venvs on
// other rows cannot share by hardlink, so the table says how big a row is.
package torchrows

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

// The versions. installer_scripts (the legacy-env installer) reads these too.
const (
	TorchVersion    = "2.11.0"
	CUDAVersion     = "12.8"
	CUDATag         = "cu128"
	ROCmTag         = "rocm7.2" // ROCM_VERSION_TAG["2.11.0"]
	XformersVersion = "0.0.35"  // NVIDIA only

	// TorchcodecIndex is where torchcodec comes from on every row.
	TorchcodecIndex = "https://download.pytorch.org/whl/cpu"
)

// Row is one torch build.
type Row struct {
	Name     string   // "cu128", "cpu", "rocm7.2", "xpu", "pypi"
	Packages []string // torch, torchvision, torchaudio with their pins
	IndexURL string   // "" means the default index
	// Xformers is the requirement installed after torch ("" = none).
	Xformers string
	// OS lists the GOOS values the row exists for.
	OS []string
	// ApproxMB is a rough download size, shown so that picking a second row is a
	// visible cost. It is an estimate, not a measurement.
	ApproxMB int
}

var rows = []Row{
	{
		Name: CUDATag,
		Packages: []string{"torch==" + TorchVersion + "+" + CUDATag, "torchvision",
			"torchaudio==" + TorchVersion + "+" + CUDATag},
		IndexURL: "https://download.pytorch.org/whl/" + CUDATag,
		Xformers: "xformers==" + XformersVersion,
		OS:       []string{"windows", "linux"},
		ApproxMB: 3500,
	},
	{
		Name:     "cpu",
		Packages: []string{"torch==" + TorchVersion + "+cpu", "torchvision", "torchaudio==" + TorchVersion},
		IndexURL: "https://download.pytorch.org/whl/cpu",
		OS:       []string{"windows", "linux"},
		ApproxMB: 400,
	},
	{
		Name:     "rocm" + strings.TrimPrefix(ROCmTag, "rocm"),
		Packages: []string{"torch==" + TorchVersion, "torchvision", "torchaudio==" + TorchVersion},
		IndexURL: "https://download.pytorch.org/whl/" + ROCmTag,
		OS:       []string{"linux"},
		ApproxMB: 6000,
	},
	{
		Name:     "xpu",
		Packages: []string{"torch==" + TorchVersion, "torchvision", "torchaudio==" + TorchVersion},
		IndexURL: "https://download.pytorch.org/whl/test/xpu",
		OS:       []string{"windows", "linux"},
		ApproxMB: 2500,
	},
	{
		Name:     "pypi",
		Packages: []string{"torch==" + TorchVersion, "torchvision", "torchaudio==" + TorchVersion},
		OS:       []string{"windows", "linux", "darwin"},
		ApproxMB: 900,
	},
}

// Rows are the table's rows for an OS, in table order.
func Rows(goos string) []Row {
	var out []Row
	for _, r := range rows {
		if r.has(goos) {
			out = append(out, r)
		}
	}
	return out
}

func (r Row) has(goos string) bool {
	for _, o := range r.OS {
		if o == goos {
			return true
		}
	}
	return false
}

// ByName is the row with that name on that OS. An unknown name is an error that
// lists the rows, never a fallback.
func ByName(name, goos string) (Row, error) {
	for _, r := range Rows(goos) {
		if r.Name == name {
			return r, nil
		}
	}
	var names []string
	for _, r := range Rows(goos) {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return Row{}, fmt.Errorf("unknown torch row %q on %s (rows: %s)", name, goos, strings.Join(names, ", "))
}

// Default is the row a GPU choice installs on an OS. A choice that has no build
// for the OS is an error (AMD on Windows, Apple off macOS).
func Default(l gpu.Label, goos string) (Row, error) {
	var name string
	switch l {
	case gpu.NVIDIA:
		name = CUDATag
	case gpu.CPU:
		name = "cpu"
		if goos == "darwin" {
			name = "pypi"
		}
	case gpu.AMD:
		name = "rocm" + strings.TrimPrefix(ROCmTag, "rocm")
	case gpu.Intel:
		name = "xpu"
	case gpu.Apple, gpu.Custom:
		name = "pypi"
	default:
		return Row{}, fmt.Errorf("unsupported GPU choice %q", l)
	}
	r, err := ByName(name, goos)
	if err != nil {
		return Row{}, fmt.Errorf("%s has no torch build for %s: %w", l, goos, err)
	}
	return r, nil
}

// Choices are the GPU choices that have a default row on an OS.
func Choices(goos string) []gpu.Label {
	var out []gpu.Label
	for _, l := range []gpu.Label{gpu.NVIDIA, gpu.AMD, gpu.Apple, gpu.Intel, gpu.CPU, gpu.Custom} {
		if _, err := Default(l, goos); err == nil {
			out = append(out, l)
		}
	}
	return out
}

// InstallArgs is what follows `uv pip install` to install the row's torch.
func (r Row) InstallArgs() []string {
	args := append([]string(nil), r.Packages...)
	if r.IndexURL != "" {
		args = append(args, "--index-url", r.IndexURL)
	}
	return args
}

// Detected is what Detect found.
type Detected struct {
	Label  gpu.Label
	Reason string // what was seen, shown to the user next to the proposal
}

// Detect proposes a GPU choice for this machine. It looks for the vendor tools
// that ship with the drivers; it never installs anything. With nothing found the
// proposal is CPU, and the user is asked all the same.
func Detect(ctx context.Context, goos, goarch string, look func(string) (string, error)) Detected {
	if look == nil {
		look = exec.LookPath
	}
	if goos == "darwin" && goarch == "arm64" {
		return Detected{gpu.Apple, "Apple silicon"}
	}
	if p, err := look("nvidia-smi"); err == nil {
		if out, err := exec.CommandContext(ctx, p, "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			if name := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]); name != "" {
				return Detected{gpu.NVIDIA, name}
			}
		}
	}
	if goos == "linux" {
		if _, err := look("rocm-smi"); err == nil {
			return Detected{gpu.AMD, "rocm-smi found"}
		}
	}
	return Detected{gpu.CPU, "no supported GPU found"}
}

// DetectHere is Detect for this machine.
func DetectHere(ctx context.Context) Detected {
	return Detect(ctx, runtime.GOOS, runtime.GOARCH, nil)
}
