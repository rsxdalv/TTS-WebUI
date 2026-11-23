package appenv

import (
	"path/filepath"
	"runtime"
)

const envHeader = `
# This file gets updated automatically from the UI

# If you wish to manually specify any ENV variables, please do so in the .env.user file
# The variables in .env.user will take PRIORITY!

# Defaults
USE_TF=0 # Disable Tensorflow for transformers

# RVC Defaults
weight_root=data/models/rvc/checkpoints # Root directory for RVC model weights
weight_uvr5_root=data/models/rvc/uvr5_weights # Root directory for RVC model weights (UVR5)
index_root=data/models/rvc/checkpoints # Root directory for RVC model indices
outside_index_root=data/models/rvc/checkpoints # Root directory for RVC model indices (outside)
rmvpe_root=data/models/rvc/rmvpe # Root directory for RVC model RMVPE

`

func envLine(name, value, comment string) string {
	prefix := ""
	if value == "" {
		prefix = "# "
	}
	return "# " + comment + "\n" + prefix + name + "=" + value + "\n"
}

// GenerateEnv returns the text writer.py generate_env() produces, with "\n"
// newlines. env is the process environment (name=value) it reads the model
// location variables from.
func GenerateEnv(env []string) string {
	fold := runtime.GOOS == "windows"
	return generateEnv(env, fold)
}

func generateEnv(env []string, fold bool) string {
	get := func(k string) string { v, _ := lookupEnv(env, k, fold); return v }
	s := envHeader
	s += envLine("HUGGINGFACE_HUB_CACHE", get("HUGGINGFACE_HUB_CACHE"), "Environment variable for HuggingFace model location")
	s += envLine("HF_HOME", get("HF_HOME"), "Environment variable for HuggingFace model location (alternative)")
	s += envLine("TORCH_HOME", get("TORCH_HOME"), "Default location for Torch Hub models")
	s += envLine("XDG_CACHE_HOME", get("XDG_CACHE_HOME"), "Default location for Torch Hub models (alternative)")
	return s + "\n"
}

// EnsureEnv writes root/.env if it does not exist (init.py writes only when
// os.path.exists(".env") is false). Python writes in text mode, so on Windows
// the file gets CRLF line endings; this does the same. It reports whether the
// file was created. The content is always UTF-8.
func EnsureEnv(root string, env []string) (bool, error) {
	return ensureEnv(root, env, runtime.GOOS == "windows")
}

// windows selects Windows behaviour: env names fold case and the file gets
// CRLF. Tests pass true on every OS because the golden file was made there.
func ensureEnv(root string, env []string, windows bool) (bool, error) {
	return writeEnvFile(filepath.Join(root, ".env"), envText(generateEnv(env, windows), windows), false)
}
