package audiocpp

import (
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

func TestAudioCPPManifest_Kind(t *testing.T) {
	m := AudioCPPManifest("cpu", "1.0.0")
	if m.Engine.Kind != manifest.KindNative {
		t.Errorf("kind = %q, want %q", m.Engine.Kind, manifest.KindNative)
	}
}

func TestAudioCPPManifest_URLTemplate_NoRawOSArch(t *testing.T) {
	// The URL template must have {os}/{arch} pre-resolved; only {version} and {ext} remain.
	m := AudioCPPManifest("NVIDIA GPU", "0.9.3")
	tmpl := m.Engine.NativeAsset.URLTemplate
	if strings.Contains(tmpl, "{os}") {
		t.Errorf("URLTemplate still contains {os}: %s", tmpl)
	}
	if strings.Contains(tmpl, "{arch}") {
		t.Errorf("URLTemplate still contains {arch}: %s", tmpl)
	}
	if strings.Contains(tmpl, "{gpu}") {
		t.Errorf("URLTemplate still contains {gpu}: %s", tmpl)
	}
	// Must still have {version} and {ext} for native.Resolve.
	if !strings.Contains(tmpl, "{version}") {
		t.Errorf("URLTemplate missing {version}: %s", tmpl)
	}
	if !strings.Contains(tmpl, "{ext}") {
		t.Errorf("URLTemplate missing {ext}: %s", tmpl)
	}
}

func TestAudioCPPManifest_URLContainsCorrectOSArch(t *testing.T) {
	m := AudioCPPManifest("NVIDIA GPU", "0.9.3")
	tmpl := m.Engine.NativeAsset.URLTemplate
	wantOS := osToken()
	wantArch := archToken()
	if !strings.Contains(tmpl, "-"+wantOS+"-") {
		t.Errorf("URL does not contain OS token %q: %s", wantOS, tmpl)
	}
	if !strings.Contains(tmpl, "-"+wantArch+"-") {
		t.Errorf("URL does not contain arch token %q: %s", wantArch, tmpl)
	}
}

func TestAudioCPPManifest_LinuxNvidiaCuda128Colab(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific GPU variant test")
	}
	m := AudioCPPManifest("NVIDIA GPU", "0.9.3")
	tmpl := m.Engine.NativeAsset.URLTemplate
	if !strings.Contains(tmpl, "cuda12.8-colab") {
		t.Errorf("Linux NVIDIA URL should use cuda12.8-colab, got: %s", tmpl)
	}
}

func TestAudioCPPManifest_Version(t *testing.T) {
	m := AudioCPPManifest("cpu", "3.0.0")
	if m.Engine.NativeAsset.Version != "3.0.0" {
		t.Errorf("NativeAsset.Version = %q, want 3.0.0", m.Engine.NativeAsset.Version)
	}
}

func TestAudioCPPManifest_CancelMode(t *testing.T) {
	m := AudioCPPManifest("cpu", "1.0.0")
	if m.Engine.Cancel == nil {
		t.Fatal("Cancel is nil")
	}
	if m.Engine.Cancel.Mode != manifest.CancelModeDisconnect {
		t.Errorf("cancel mode = %q, want %q", m.Engine.Cancel.Mode, manifest.CancelModeDisconnect)
	}
}

func TestAudioCPPManifest_KeepAlive(t *testing.T) {
	m := AudioCPPManifest("cpu", "1.0.0")
	if m.Engine.KeepAlive != 300 {
		t.Errorf("KeepAlive = %d, want 300", m.Engine.KeepAlive)
	}
}

func TestAudioCPPManifest_HealthPath(t *testing.T) {
	m := AudioCPPManifest("cpu", "1.0.0")
	if m.Engine.Health == nil {
		t.Fatal("Health is nil")
	}
	if m.Engine.Health.Path != "/health" {
		t.Errorf("health path = %q, want /health", m.Engine.Health.Path)
	}
}

func TestAudioCPPManifest_PackageName(t *testing.T) {
	m := AudioCPPManifest("NVIDIA GPU", "1.0.0")
	if m.PackageName != "audio.cpp" {
		t.Errorf("PackageName = %q, want audio.cpp", m.PackageName)
	}
}

func TestAudioCPPManifest_ChecksumsNonEmpty(t *testing.T) {
	m := AudioCPPManifest("CPU", "0.9.3")
	if len(m.Engine.NativeAsset.Checksums) == 0 {
		t.Error("Checksums map is empty; all 10 v0.9.3 assets must be listed")
	}
	for name, sum := range m.Engine.NativeAsset.Checksums {
		if len(sum) != 64 {
			t.Errorf("checksum for %q has length %d, want 64", name, len(sum))
		}
	}
}

func TestAudioCPPManifest_BinaryName(t *testing.T) {
	m := AudioCPPManifest("CPU", "0.9.3")
	bp := m.Engine.NativeAsset.BinaryPath
	if bp != "audiocpp_server" && bp != "audiocpp_server.exe" {
		t.Errorf("BinaryPath = %q, want audiocpp_server[.exe]", bp)
	}
}

func TestAudioCPPManifest_UsesConfigTemplate(t *testing.T) {
	m := AudioCPPManifest("CPU", "0.9.3")
	if m.Engine.Launch == nil {
		t.Fatal("Launch is nil")
	}
	if m.Engine.Launch.ConfigTemplate == "" {
		t.Error("ConfigTemplate is empty; server requires --config, not --host/--port")
	}
	if !strings.Contains(m.Engine.Launch.ConfigTemplate, "{port}") {
		t.Errorf("ConfigTemplate missing {port}: %s", m.Engine.Launch.ConfigTemplate)
	}
	if m.Engine.Launch.PortArg != "" {
		t.Errorf("PortArg should be empty when ConfigTemplate is set, got %q", m.Engine.Launch.PortArg)
	}
}

// TestAudioCPPURLsExist sends a HEAD request to each generated URL for each
// platform/GPU combination. Skipped in short mode (go test -short).
// Source: https://github.com/BtbN/FFmpeg-Builds/releases and
//
//	https://api.github.com/repos/0xShug0/audio.cpp/releases/latest (read 2026-11-02)
func TestAudioCPPURLsExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network URL check in short mode")
	}

	// Known good URLs for v0.9.3 — verified against checksums at 2026-11-02.
	knownURLs := []string{
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-windows-x64-cpu.zip",
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-windows-x64-cuda12.4.zip",
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-ubuntu-x64-cuda12.8-colab.tar.gz",
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-ubuntu-x64-cpu.tar.gz",
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-macos-arm64-metal.tar.gz",
		"https://github.com/0xShug0/audio.cpp/releases/download/v0.9.3/audio-v0.9.3-bin-macos-x64-metal.tar.gz",
	}
	for _, u := range knownURLs {
		resp, err := http.Head(u)
		if err != nil {
			t.Errorf("HEAD %s: %v", u, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("HEAD %s: status %d", u, resp.StatusCode)
		}
	}
}
