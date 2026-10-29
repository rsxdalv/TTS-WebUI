// Package legacy provides the built-in engine manifest for the legacy Gradio
// server (server.py) and helpers to load native-proxy extension manifests.
//
// In --proxy=core mode (C5.5) the Go core starts server.py as the
// "legacy-gradio" engine and routes "/" through the router to it.
package legacy

import (
	"path/filepath"
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// LegacyGradioManifest returns a built-in manifest for `server.py --no-react`.
// The launch command uses the conda env Python; the lifecycle manager will
// assign a free port and pass it via the PortArg mechanism.
func LegacyGradioManifest(root paths.Root) manifest.Manifest {
	python := condaPython(root)
	serverPy := filepath.Join(root.Dir(), "server.py")

	return manifest.Manifest{
		PackageName: "legacy-gradio",
		Name:        "Legacy Gradio Server",
		Engine: manifest.Engine{
			Kind: manifest.KindPythonVenv,
			Launch: &manifest.Launch{
				Cmd: []string{python, serverPy, "--no-react"},
				Cwd: root.Dir(),
			},
			Health: &manifest.Health{
				Path:      "/",
				IntervalS: 2,
				TimeoutS:  60,
			},
			KeepAlive: 0, // always on
		},
	}
}

// condaPython returns the Python binary inside installer_files/env/.
func condaPython(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "env", "python.exe")
	}
	return filepath.Join(root.InstallerFiles(), "env", "bin", "python")
}

// LoadNativeExtensions reads extensions.json and extensions.external.json via
// manifest.Load, then returns only entries whose engine kind is python-venv
// (i.e. entries that were already proxy:native or had an explicit engine block).
// dir is the install root directory containing extensions.json.
func LoadNativeExtensions(dir string) ([]manifest.Manifest, error) {
	all, err := manifest.Load(dir)
	if err != nil {
		return nil, err
	}
	var native []manifest.Manifest
	for _, m := range all {
		if m.Engine.Kind == manifest.KindPythonVenv {
			native = append(native, m)
		}
	}
	return native, nil
}
