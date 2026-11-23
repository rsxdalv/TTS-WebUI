package extcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The torch table of tts_webui/utils/get_torch_command.py, copied as is. It is
// frozen during the migration: change both or neither.
const (
	torchVersion   = "2.11.0"
	cudaVersionTag = "cu128"
	rocmVersionTag = "rocm7.2" // ROCM_VERSION_TAG["2.11.0"]
)

type torchSource struct {
	pkg      string
	indexURL string
}

var torchChoices = map[string]torchSource{
	"NVIDIA GPU": {
		"torch==" + torchVersion + "+" + cudaVersionTag + " torchvision torchaudio==" + torchVersion + "+" + cudaVersionTag,
		"https://download.pytorch.org/whl/" + cudaVersionTag,
	},
	"Custom (Manual torch install)": {"torch==" + torchVersion + " torchvision torchaudio==" + torchVersion, ""},
	"Apple M Series Chip":           {"torch==" + torchVersion + " torchvision torchaudio==" + torchVersion, ""},
	"CPU": {
		"torch==" + torchVersion + "+cpu torchvision torchaudio==" + torchVersion,
		"https://download.pytorch.org/whl/cpu",
	},
	"AMD GPU (ROCM, Linux only)": {
		"torch==" + torchVersion + " torchvision torchaudio==" + torchVersion,
		"https://download.pytorch.org/whl/" + rocmVersionTag,
	},
	"Intel GPU (XPU)": {
		"torch==" + torchVersion + " torchvision torchaudio==" + torchVersion,
		"https://download.pytorch.org/whl/test/xpu",
	},
}

// torchArgs is get_torch_command() as install arguments, from the GPU choice
// the installer saved in installer_scripts/.gpu.
func (s *Store) torchArgs() ([]string, error) {
	file := filepath.Join(s.opts.Root, "installer_scripts", ".gpu")
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("GPU choice file not found in %s. Please run the installer to select your GPU", file)
		}
		return nil, err
	}
	choice := strings.TrimSpace(string(b))
	src, ok := torchChoices[choice]
	if !ok {
		return nil, fmt.Errorf("Unsupported GPU choice: %s", choice)
	}
	args := strings.Fields(src.pkg)
	if src.indexURL != "" {
		args = append(args, "--index-url", src.indexURL)
	}
	return args, nil
}
