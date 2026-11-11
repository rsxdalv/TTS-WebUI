package gpu

import (
	"errors"
	"fmt"
)

// ErrCustomManaged is returned by TorchFor when the label is Custom.
// The caller should skip torch installation; it is not a failure.
var ErrCustomManaged = errors.New("Custom (Manual torch install): user manages torch")

// TorchSpec describes the pip packages needed to install torch for a GPU label.
// Matches the commands in installer_scripts/js/initializeApp.js exactly.
const torchVersion = "2.11.0"

// TorchSpec describes the pip packages and index URL for a torch install.
type TorchSpec struct {
	// Pkgs are pip-compatible package specifiers.
	Pkgs []string
	// IndexURL is the --index-url to pass, or "" for the default PyPI index.
	IndexURL string
	// XFormers is the xformers specifier to install (e.g. "xformers==0.0.35"),
	// or "" if xformers is not needed for this label.
	XFormers string
}

// TorchFor returns the TorchSpec for the given GPU label.
//
// Matches installer_scripts/js/initializeApp.js (torch 2.11.0, cu128 + xformers 0.0.35
// for NVIDIA, rocm7.2 + xformers for AMD, XPU test index for Intel, CPU index for CPU,
// default PyPI for Apple).
//
// Returns ErrCustomManaged when label is Custom (caller should skip torch install).
// Returns a non-nil error for any unrecognised label.
func TorchFor(label Label) (TorchSpec, error) {
	switch label {
	case NVIDIA:
		return TorchSpec{
			Pkgs:     []string{"torch==" + torchVersion + "+cu128", "torchvision", "torchaudio==" + torchVersion + "+cu128"},
			IndexURL: "https://download.pytorch.org/whl/cu128",
			XFormers: "xformers==0.0.35",
		}, nil
	case AMD:
		return TorchSpec{
			Pkgs:     []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion},
			IndexURL: "https://download.pytorch.org/whl/rocm7.2",
			XFormers: "xformers",
		}, nil
	case Apple:
		return TorchSpec{
			Pkgs: []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion},
		}, nil
	case Intel:
		return TorchSpec{
			Pkgs:     []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion},
			IndexURL: "https://download.pytorch.org/whl/test/xpu",
		}, nil
	case CPU:
		return TorchSpec{
			Pkgs:     []string{"torch==" + torchVersion + "+cpu", "torchvision", "torchaudio==" + torchVersion},
			IndexURL: "https://download.pytorch.org/whl/cpu",
		}, nil
	case Custom:
		return TorchSpec{}, ErrCustomManaged
	default:
		return TorchSpec{}, fmt.Errorf("unknown GPU label %q: cannot determine torch packages", label)
	}
}
