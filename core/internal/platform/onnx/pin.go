package onnx

import "fmt"

// RuntimeVersion is the sherpa-onnx release this platform installs. It is
// pinned: another release is a reviewed change of this file, never "latest".
// v2.x is not used: it dropped espeak-ng, and the Kitten and Piper packages of
// the model list fail there ("No lexicon available"), run on 2026-11-25.
const RuntimeVersion = "v1.13.8"

// Archive is a downloadable archive and its checksum.
type Archive struct {
	URL    string
	SHA256 string
	Size   int64 // bytes, for the disk check and the message; 0 = unknown
}

// runtimes: the checksum is the digest GitHub lists for the release asset
// (api.github.com/repos/k2-fsa/sherpa-onnx/releases), read on 2026-11-25, and a
// download hashed to the same value. Windows x64 only in this version; any other
// OS or architecture is refused with a message (ArchiveFor).
var runtimes = map[string]Archive{
	"windows/amd64": {
		URL:    "https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.13.8/sherpa-onnx-v1.13.8-win-x64-shared-MT-Release.tar.bz2",
		SHA256: "6dffdc715a4465b989446a6105265d2cb345e7101591a17d35534b6758f6e8df",
		Size:   24805859,
	},
}

// ArchiveFor is the pinned runtime for an OS and architecture.
func ArchiveFor(goos, goarch string) (Archive, error) {
	if a, ok := runtimes[goos+"/"+goarch]; ok {
		return a, nil
	}
	return Archive{}, fmt.Errorf("the onnx platform is not available for %s/%s in this version (Windows x64 only)", goos, goarch)
}

// Model is one engine the platform can run: a pinned model archive and how the
// runtime is told to use it.
type Model struct {
	Package string // the catalog entry's package_name
	Dir     string // folder name under platforms/onnx/models (short, Windows paths)
	Title   string
	Kind    string // "kitten": which runtime options name the files
	Archive Archive
	// Files are paths inside the extracted folder that must exist (the install
	// is refused otherwise).
	Files    []string
	Voices   int // speaker ids 0..Voices-1
	Rate     int // sample rate of the clip, for the message only
	License  string
	Language string
}

// models: the checksum and size are the ones GitHub lists for the asset of the
// k2-fsa/sherpa-onnx `tts-models` release, read on 2026-11-25; a download hashed to
// the same value.
var models = []Model{{
	Package: "tts_webui_onnx.kitten_nano",
	Dir:     "kitten-nano",
	Title:   "Kitten TTS nano 0.8 (ONNX)",
	Kind:    "kitten",
	Archive: Archive{
		URL:    "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/kitten-nano-en-v0_8-int8.tar.bz2",
		SHA256: "6fa5be852612ce761094ba74ee6123b4fc4acfefa79bf64dc63acae4a83af2fd",
		Size:   31220690,
	},
	Files:    []string{"model.int8.onnx", "voices.bin", "tokens.txt", "espeak-ng-data"},
	Voices:   8,
	License:  "Apache-2.0 (model and runtime)",
	Language: "en",
}}

// ModelFor finds a pinned model by the catalog entry's package name.
func ModelFor(pkg string) (Model, bool) {
	for _, m := range models {
		if m.Package == pkg {
			return m, true
		}
	}
	return Model{}, false
}

// Models lists the pinned models.
func Models() []Model { return append([]Model(nil), models...) }
