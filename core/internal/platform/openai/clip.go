package openai

import (
	"errors"
	"time"
)

// Saved is where a clip went, relative to the install folder (with /).
type Saved = saved

// SaveClip saves a WAV that another platform generated, in the layout the History
// page reads (outputs/<date>__<label>__<title>/<same>.{wav,json,png}). label
// names the generator in the folder name and in the metadata's "endpoint".
// It returns where the clip went and its length in seconds.
func SaveClip(root string, now time.Time, label, text, model, voice string, wav []byte) (Saved, float64, error) {
	w, err := parseWav(wav)
	if err != nil {
		return Saved{}, 0, err
	}
	s, err := saveGeneration(root, now, meta{Text: text, Endpoint: label, Model: model, Voice: voice,
		Speed: 1, ResponseFormat: "wav", Label: label}, wav, w)
	if err != nil {
		return Saved{}, 0, err
	}
	return s, w.seconds(), nil
}

// Field is one metadata field of a saved clip.
type Field struct {
	Key   string
	Value any
}

// SaveClipFields is SaveClip for a generator whose metadata is not the openai
// one: the clip's JSON has "_version", "_hash_version", "text", then fields in
// order, then "outputs", "date" and "hash" like every legacy generation. version
// is the "_version" value ("" keeps 0.0.1). The label and the folder rules are
// SaveClip's.
func SaveClipFields(root string, now time.Time, label, text, version string, fields []Field, wav []byte) (Saved, float64, error) {
	w, err := parseWav(wav)
	if err != nil {
		return Saved{}, 0, err
	}
	of := make(orderedJSON, 0, len(fields))
	for _, f := range fields {
		of = append(of, kv{f.Key, f.Value})
	}
	s, err := saveGeneration(root, now, meta{Text: text, Label: label, Version: version, Fields: of}, wav, w)
	if err != nil {
		return Saved{}, 0, err
	}
	return s, w.seconds(), nil
}

// CheckWav reports the length in seconds of a WAV file the platform can read;
// the error says why not (ErrNotWav when it is not a WAV file at all).
func CheckWav(b []byte) (seconds float64, err error) {
	w, err := parseWav(b)
	if err != nil {
		return 0, err
	}
	if w.frames() == 0 {
		return 0, errors.New("the WAV file has no audio in it")
	}
	return w.seconds(), nil
}

// ErrNotWav is returned by CheckWav when the bytes are not a WAV file.
var ErrNotWav = errNotWav
