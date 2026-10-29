// Package manifest defines and loads the engine manifest schema that extends
// extensions.json entries with engine-specific configuration (D5).
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
)

// Kind is the engine runtime kind.
type Kind string

const (
	KindPythonVenv Kind = "python-venv"
	KindNative     Kind = "native"
	KindRemote     Kind = "remote"
)

// Install describes how to create the engine's Python venv.
type Install struct {
	Requirements []string `json:"requirements,omitempty"` // pip-compatible specifiers
	PipPackages  []string `json:"pip_packages,omitempty"`
}

// Launch describes how to start the engine process.
type Launch struct {
	Cmd     []string          `json:"cmd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	PortArg string            `json:"port_arg,omitempty"` // e.g. "--port"
}

// Health describes how to probe the engine's readiness.
type Health struct {
	Path      string `json:"path,omitempty"`       // HTTP path, e.g. "/health"
	IntervalS int    `json:"interval_s,omitempty"` // probe interval in seconds
	TimeoutS  int    `json:"timeout_s,omitempty"`  // per-probe timeout in seconds
}

// API describes the engine's HTTP API surface.
type API struct {
	OpenAISpeechPath string         `json:"openai_speech_path,omitempty"`
	ParamsSchema     map[string]any `json:"params_schema,omitempty"`
}

// UI describes how the engine's UI is served.
type UI struct {
	GradioIframe bool `json:"gradio_iframe,omitempty"`
}

// CancelMode describes how to cancel an in-flight generation.
type CancelMode string

const (
	CancelModeDisconnect CancelMode = "disconnect"
	CancelModeEndpoint   CancelMode = "endpoint"
)

// Cancel describes how to cancel a running generation.
type Cancel struct {
	Mode CancelMode `json:"mode,omitempty"`
	Path string     `json:"path,omitempty"` // endpoint path when Mode == endpoint
}

// NativeAsset describes the downloadable release asset for a native engine.
type NativeAsset struct {
	// URLTemplate is the download URL with {version}, {os}, {arch}, {gpu}, {ext} placeholders.
	// Example: https://github.com/owner/repo/releases/download/v{version}/{name}-{os}-{arch}-{gpu}.{ext}
	URLTemplate string `json:"url_template,omitempty"`
	// Version is the pinned release version (without leading "v").
	Version string `json:"version,omitempty"`
	// Checksums maps asset filename to expected SHA-256 hex digest.
	Checksums map[string]string `json:"checksums,omitempty"`
	// BinaryPath is the relative path to the main binary inside the extracted archive.
	// Defaults to PackageName + ".exe" on Windows, PackageName on Unix.
	BinaryPath string `json:"binary_path,omitempty"`
}

// Engine holds the engine-specific block for a manifest entry.
type Engine struct {
	Kind        Kind         `json:"kind"`
	Install     *Install     `json:"install,omitempty"`
	Launch      *Launch      `json:"launch,omitempty"`
	Health      *Health      `json:"health,omitempty"`
	API         *API         `json:"api,omitempty"`
	UI          *UI          `json:"ui,omitempty"`
	Cancel      *Cancel      `json:"cancel,omitempty"`
	NativeAsset *NativeAsset `json:"native_asset,omitempty"`
	KeepAlive   int          `json:"keep_alive,omitempty"` // idle timeout in seconds
}

// Manifest is a fully resolved engine entry derived from extensions.json.
type Manifest struct {
	PackageName string `json:"package_name"`
	Name        string `json:"name"`
	Engine      Engine `json:"engine"`
}

// extensionEntry is the raw structure of a single extensions.json entry.
type extensionEntry struct {
	PackageName string  `json:"package_name"`
	Name        string  `json:"name"`
	Proxy       string  `json:"proxy,omitempty"`
	Engine      *Engine `json:"engine,omitempty"`
	// remaining fields ignored
}

// extensionsFile is the top-level structure of extensions.json.
type extensionsFile struct {
	// extensions.json uses named groups; we need to collect all entries.
	TabsInGroups *struct {
		Extensions []extensionEntry `json:"extensions"`
	} `json:"tabsInGroups,omitempty"`
	Tabs []extensionEntry `json:"tabs,omitempty"`
}

// Load reads extensions.json and (if present) extensions.external.json from the
// directory dir, merges the entries, and returns Manifests for all entries that
// either have an explicit "engine" block or carry "proxy": "native"
// (which maps to kind: python-venv).
func Load(dir string) ([]Manifest, error) {
	mainPath := fmt.Sprintf("%s/extensions.json", dir)
	externalPath := fmt.Sprintf("%s/extensions.external.json", dir)

	entries, err := loadFile(mainPath)
	if err != nil {
		return nil, fmt.Errorf("loading extensions.json: %w", err)
	}
	if ext, err := loadFile(externalPath); err == nil {
		entries = append(entries, ext...)
	} // external file is optional — ignore not-found errors

	var manifests []Manifest
	for _, e := range entries {
		m, ok := toManifest(e)
		if !ok {
			continue
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// loadFile reads one extensions JSON file and returns all extension entries.
func loadFile(path string) ([]extensionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Try the "tabs" top-level array (extensions.external.json shape).
	var withTabs struct {
		Tabs []extensionEntry `json:"tabs"`
	}
	if json.Unmarshal(data, &withTabs) == nil && len(withTabs.Tabs) > 0 {
		return withTabs.Tabs, nil
	}
	// Try the "tabsInGroups.extensions" shape (extensions.json).
	var withGroups struct {
		TabsInGroups struct {
			Extensions []extensionEntry `json:"extensions"`
		} `json:"tabsInGroups"`
	}
	if json.Unmarshal(data, &withGroups) == nil {
		return withGroups.TabsInGroups.Extensions, nil
	}
	return nil, fmt.Errorf("unrecognised extensions file shape")
}

// toManifest converts a raw entry into a Manifest, returning false if the
// entry is not engine-managed (no "engine" block and not "proxy": "native").
func toManifest(e extensionEntry) (Manifest, bool) {
	if e.Engine != nil {
		return Manifest{PackageName: e.PackageName, Name: e.Name, Engine: *e.Engine}, true
	}
	if e.Proxy == "native" {
		return Manifest{
			PackageName: e.PackageName,
			Name:        e.Name,
			Engine: Engine{
				Kind: KindPythonVenv,
				Health: &Health{
					Path:      "/health",
					IntervalS: 2,
					TimeoutS:  10,
				},
				UI:        &UI{GradioIframe: true},
				KeepAlive: 300,
			},
		}, true
	}
	return Manifest{}, false
}
