package appenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// envWrite serialises every write of root/.env in this process. writeEnvFile
// is the only function that writes it: EnsureEnv (create when missing) and
// SaveModelLocations (replace) both go through it.
var envWrite sync.Mutex

// ModelLocations are the four cache variables the Model Location Settings tab
// saves into .env. An empty value is written commented out ("# NAME=").
type ModelLocations struct {
	HuggingFaceHubCache string
	HFHome              string
	TorchHome           string
	XDGCacheHome        string
}

// env is the name=value form generateEnv reads the variables from.
func (m ModelLocations) env() []string {
	return []string{
		"HUGGINGFACE_HUB_CACHE=" + m.HuggingFaceHubCache,
		"HF_HOME=" + m.HFHome,
		"TORCH_HOME=" + m.TorchHome,
		"XDG_CACHE_HOME=" + m.XDGCacheHome,
	}
}

// Validate refuses what would not stay one value on one line of .env. The old
// Python wrote the text as given, so a value with a line break added a variable
// to the file that every child process then loaded.
func (m ModelLocations) Validate() error {
	for _, f := range []struct{ name, value string }{
		{"huggingface_hub_cache", m.HuggingFaceHubCache},
		{"hf_home", m.HFHome},
		{"torch_home", m.TorchHome},
		{"xdg_cache_home", m.XDGCacheHome},
	} {
		if len(f.value) > maxValue {
			return invalidf("%s is longer than %d bytes", f.name, maxValue)
		}
		for _, r := range f.value {
			if r < 0x20 || r == 0x7f {
				return invalidf("%s contains a control character", f.name)
			}
		}
	}
	return nil
}

const maxValue = 4096

// errInvalid marks input the API refuses (HTTP 400), as opposed to a failure to write.
var errInvalid = errors.New("invalid")

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errInvalid, fmt.Sprintf(format, a...))
}

// SaveModelLocations rewrites root/.env with the default text and these four
// values, as write_env(generate_env(...)) did: the file is replaced as a whole,
// so what a user added by hand to .env is not kept (it belongs in .env.user).
func SaveModelLocations(root string, m ModelLocations) error {
	return saveModelLocations(root, m, runtime.GOOS == "windows")
}

func saveModelLocations(root string, m ModelLocations, windows bool) error {
	if err := m.Validate(); err != nil {
		return err
	}
	_, err := writeEnvFile(filepath.Join(root, ".env"), envText(generateEnv(m.env(), windows), windows), true)
	return err
}

// envText is what Python's text mode writes: CRLF on Windows.
func envText(text string, windows bool) string {
	if windows {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// writeEnvFile writes text to path. With replace it swaps the file in whole
// (temp file and rename, so a reader never sees half a file); without it, it
// creates the file only when it does not exist and reports whether it did.
func writeEnvFile(path, text string, replace bool) (bool, error) {
	envWrite.Lock()
	defer envWrite.Unlock()
	if !replace {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return false, nil
			}
			return false, err
		}
		if _, err := f.WriteString(text); err != nil {
			f.Close()
			return false, err
		}
		return true, f.Close()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".env.*.tmp")
	if err != nil {
		return false, err
	}
	_, err = tmp.WriteString(text)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return false, err
	}
	return true, nil
}
