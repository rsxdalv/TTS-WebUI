package audiocpp

import (
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

func TestAudioCPPManifest_Kind(t *testing.T) {
	m := AudioCPPManifest("cpu", "1.0.0")
	if m.Engine.Kind != manifest.KindNative {
		t.Errorf("kind = %q, want %q", m.Engine.Kind, manifest.KindNative)
	}
}

func TestAudioCPPManifest_URLTemplate(t *testing.T) {
	m := AudioCPPManifest("NVIDIA GPU", "2.0.0")
	const want = "https://github.com/0xShug0/audio.cpp/releases/download/v{version}/audio-v{version}-bin-{os}-{arch}-{gpu}.{ext}"
	if m.Engine.NativeAsset.URLTemplate != want {
		t.Errorf("URLTemplate:\n got %q\nwant %q", m.Engine.NativeAsset.URLTemplate, want)
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
