package audiocpp

import "strings"

// File is one file of a model package: where it comes from, what it must hash to
// and where it goes (relative to the model's folder).
type File struct {
	Rel    string
	URL    string
	SHA256 string
	Size   int64
}

// Model is one audio.cpp model the catalog offers. Its catalog entry names it by
// package_name and requires the audiocpp platform.
type Model struct {
	Package      string // package_name of the catalog entry
	ID           string // the id the server config gives it ("model" in a request)
	Name         string
	Family       string // audio.cpp family
	Dir          string // models/<Dir>/
	GGUF         string // the GGUF's file name in Dir
	LoadOptions  map[string]any
	SampleRate   int // of the PCM the server streams (16-bit mono)
	Voices       []string
	DefaultVoice string
	License      string
	Files        []File
}

// hf is the Hugging Face repository the packages come from, at a fixed revision
// (read 2026-11-25): a file there can change only by a reviewed edit of this table.
const hf = "https://huggingface.co/audio-cpp/audio.cpp-gguf/resolve/047ec541cc3f7e50c583acd56e0177ae286c4078/"

// Models is the pinned list. The sizes and sha256 are the ones Hugging Face lists for
// each file (the LFS object id is the sha256); the three files of the first
// model were downloaded and hashed to the same values.
var Models = []Model{
	{
		Package:      "audiocpp_pocket_tts_en",
		ID:           "pocket-tts-en",
		Name:         "Pocket TTS (English, audio.cpp)",
		Family:       "pocket_tts",
		Dir:          "pocket-en-q8",
		GGUF:         "pocket-tts-english-q8_0.gguf",
		LoadOptions:  map[string]any{"language": "english"},
		SampleRate:   24000,
		Voices:       []string{"alba", "cosette", "marius"},
		DefaultVoice: "alba",
		License:      "CC-BY-4.0",
		Files: []File{
			{"pocket-tts-english-q8_0.gguf", hf + "PocketTTS-GGUF/english/pocket-tts-english-q8_0.gguf",
				"3f71cf6a51f93c2f2e554e98970a3f49fb9a98d0c64bed5a815e65be3d2ff588", 127867968},
			{"embeddings/alba.safetensors", hf + "PocketTTS-GGUF/english/embeddings/alba.safetensors",
				"69c32db63ca56843d994f81f343f62e0bf2d73f7e4c9bc73e44bb1110b1d8845", 6194424},
			{"embeddings/cosette.safetensors", hf + "PocketTTS-GGUF/english/embeddings/cosette.safetensors",
				"c4fdc15f5a3a20c44dd0064a37e87d15d25562936e8dbad7e07b9832015a545d", 6194424},
			{"embeddings/marius.safetensors", hf + "PocketTTS-GGUF/english/embeddings/marius.safetensors",
				"04f84efcb77a0547ba582c058db496f7ff4920891d49d37b9950d128422582a8", 6194424},
		},
	},
}

// ModelFor is the model of a catalog package name.
func ModelFor(pkg string) (Model, bool) {
	for _, m := range Models {
		if m.Package == pkg {
			return m, true
		}
	}
	return Model{}, false
}

// ModelByID is the model with a server id.
func ModelByID(id string) (Model, bool) {
	for _, m := range Models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// SizeMB is what the model's files add up to.
func (m Model) SizeMB() int {
	var n int64
	for _, f := range m.Files {
		n += f.Size
	}
	return int(n >> 20)
}

// HasVoice reports whether v is one of the model's voices.
func (m Model) HasVoice(v string) bool {
	for _, x := range m.Voices {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}
