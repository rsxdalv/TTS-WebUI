// Package audiocpp provides the engine manifest for the audio.cpp native TTS engine.
//
// audio.cpp is a fork of llama.cpp adapted for text-to-speech inference.
// Details marked "needs human verification" must be confirmed once the
// rsxdalv/audio.cpp repository becomes publicly accessible.
package audiocpp

import (
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// urlTemplate is the GitHub Releases asset URL pattern for audio.cpp.
// needs human verification: confirm exact asset naming in rsxdalv/audio.cpp releases.
const urlTemplate = "https://github.com/rsxdalv/audio.cpp/releases/download/v{version}/audio-cpp-{os}-{arch}-{gpu}.{ext}"

// AudioCPPManifest returns the engine manifest for the audio.cpp native engine.
//
// gpuLabel selects the GPU variant (cpu, cuda, vulkan, metal); it is resolved at launch
// time by native.Resolve and is not embedded in the returned manifest.
//
// Cancel mode is "disconnect": the core router cancels the upstream request context.
// The llama.cpp HTTP server does not natively abort inference on disconnect; the
// generation loop may run to completion depending on the audio.cpp build.
// Sentence-chunked requests are the recommended fallback: send one sentence at a time
// and stop sending when the caller cancels.
// Last resort: lifecycle.Manager.Stop then Start to kill and restart the process.
func AudioCPPManifest(gpuLabel, version string) manifest.Manifest {
	_ = gpuLabel // passed to native.Resolve at launch time, not stored in the manifest
	return manifest.Manifest{
		PackageName: "audio.cpp",
		Name:        "audio.cpp",
		Engine: manifest.Engine{
			Kind: manifest.KindNative,
			NativeAsset: &manifest.NativeAsset{
				URLTemplate: urlTemplate,
				Version:     version,
				BinaryPath:  audioCPPBinaryPath(),
			},
			Launch: &manifest.Launch{
				// The leading path element is overridden by native.Launch at run time.
				// needs human verification: confirm actual CLI flags in rsxdalv/audio.cpp.
				Cmd:     []string{"./audio-cpp-server", "--host", "127.0.0.1"},
				PortArg: "--port",
			},
			Health: &manifest.Health{
				Path:      "/health", // standard llama.cpp health endpoint
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
// needs human verification: confirm actual binary name in rsxdalv/audio.cpp archives.
func audioCPPBinaryPath() string {
	if runtime.GOOS == "windows" {
		return "audio-cpp-server.exe"
	}
	return "audio-cpp-server"
}
