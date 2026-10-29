// Package audiocpp provides the engine manifest for the audio.cpp native TTS engine.
//
// Sourced from 0xShug0/audio.cpp v0.9.3 (2026-10-29).
// See documentation/core-rewrite/notes/C8.0.md for full research note.
package audiocpp

import (
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// urlTemplate is the GitHub Releases asset URL pattern for audio.cpp v0.9.3.
// Source: https://api.github.com/repos/0xShug0/audio.cpp/releases/latest
// {os} is "windows"/"ubuntu"/"macos"; {arch} is "x64"/"arm64"; {gpu} is the backend token.
// Linux NVIDIA uses "cuda12.8-colab" (NOT "cuda12.8").
const urlTemplate = "https://github.com/0xShug0/audio.cpp/releases/download/v{version}/audio-v{version}-bin-{os}-{arch}-{gpu}.{ext}"

// checksums maps asset filename → SHA256 for v0.9.3.
// Source: https://api.github.com/repos/0xShug0/audio.cpp/releases/latest
var checksums = map[string]string{
	// Windows
	"audio-v0.9.3-bin-windows-x64-cpu.zip":             "3ac66f46395b4e92e4f3bb68b200f0032310bd9922f12d97f10ccdadb02dee4b",
	"audio-v0.9.3-bin-windows-x64-cuda12.4.zip":        "976b1fe4329407be45b272fc656d91006f081d22706f453f8879bc69dbb4a9b3",
	"audio-v0.9.3-bin-windows-x64-cuda13.3.zip":        "0bd3988be10c31ec078c40df2bd0d4266e660804f6d5083aeb05d1309d091e43",
	"audio-v0.9.3-bin-windows-x64-rocm10.0.0-test.zip": "ca07ba49b7e63d9fc37eb911b451fc3d97956f09441cb08cac3ad6b16f7627df",
	"audio-v0.9.3-bin-windows-x64-vulkan-portable.zip": "51f8d8ce6cc7789bc9dc3d15f35e05f143aaf2ce1cbecc525a4104bcdde82a42",
	// Linux (ubuntu)
	"audio-v0.9.3-bin-ubuntu-x64-cpu.tar.gz":             "98d06ba2a5e522f06aec7397c926691c4698137e7649c2436f16f3232320d79b",
	"audio-v0.9.3-bin-ubuntu-x64-cuda12.8-colab.tar.gz":  "5be3e8d515fb244387de87958668ebb729d9442de6a53df12e4c718ea918419d",
	"audio-v0.9.3-bin-ubuntu-x64-rocm10.0.0-test.tar.gz": "4d0688267b2b8c421238994031c720d52316cacc24150ea613ee6bb2e5b13dfc",
	// macOS
	"audio-v0.9.3-bin-macos-arm64-metal.tar.gz": "70a84c2b61ce5ef99547a12affab9b0d40daf08666baf7da4f940d5c7b722819",
	"audio-v0.9.3-bin-macos-x64-metal.tar.gz":   "81238e3957bbf840d15094c34262fa93609caa0233c3dccc1e0f64fb47f5e51b",
}

// osToken maps runtime.GOOS to the asset OS token used by 0xShug0/audio.cpp.
func osToken() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	default:
		return "ubuntu"
	}
}

// archToken maps runtime.GOARCH to the asset arch token used by 0xShug0/audio.cpp.
func archToken() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x64"
}

// AudioCPPManifest returns the engine manifest for the audio.cpp native engine.
//
// gpuLabel is the PyTorchChoice label (e.g. "NVIDIA GPU"); it is mapped to the
// URL-safe backend token by native.gpuVariant at resolve time.
// The Linux NVIDIA asset uses "cuda12.8-colab" — the URL template uses the
// gpuVariant output, but callers must pass "NVIDIA GPU" for correct resolution.
//
// Cancel mode is "disconnect": an SSE client that disconnects stops the run.
// Source: app/server/api.md — "An SSE client that disconnects stops the run
// within the next event or two."
func AudioCPPManifest(gpuLabel, version string) manifest.Manifest {
	_ = gpuLabel // resolved to URL token by native.Resolve via gpuVariant
	return manifest.Manifest{
		PackageName: "audio.cpp",
		Name:        "audio.cpp",
		Engine: manifest.Engine{
			Kind: manifest.KindNative,
			NativeAsset: &manifest.NativeAsset{
				URLTemplate: urlTemplate,
				Version:     version,
				BinaryPath:  audioCPPBinaryPath(),
				Checksums:   checksums,
			},
			Launch: &manifest.Launch{
				Cmd:     []string{"./audiocpp_server", "--host", "127.0.0.1"},
				PortArg: "--port",
			},
			Health: &manifest.Health{
				Path:      "/health",
				IntervalS: 2,
				TimeoutS:  30,
			},
			Cancel: &manifest.Cancel{
				Mode: manifest.CancelModeDisconnect,
			},
			KeepAlive: 300,
		},
	}
}

// audioCPPBinaryPath returns the expected binary name inside the release archive.
// Source: section 2 of C8.0.md — executable is `audiocpp_server`.
func audioCPPBinaryPath() string {
	if runtime.GOOS == "windows" {
		return "audiocpp_server.exe"
	}
	return "audiocpp_server"
}
