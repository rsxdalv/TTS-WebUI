package openai

import (
	"errors"
	"fmt"
	"time"
)

// ClipMeta is what a local engine records beside a clip.
type ClipMeta struct {
	Engine string // the name in the folder and in the metadata
	Model  string
	Voice  string
	Text   string
	Speed  float64
}

// SaveClip checks that wav is a WAV file with audio in it and saves it in
// <root>/outputs in the layout History reads (the same code the openai platform
// uses for a remote clip). Other engines that make WAV files in Go call it.
func SaveClip(root string, now time.Time, m ClipMeta, wav []byte) (Result, error) {
	w, err := parseWav(wav)
	if err != nil {
		return Result{}, fmt.Errorf("not a usable WAV file: %w", err)
	}
	if w.frames() == 0 {
		return Result{}, errors.New("the WAV file has no audio in it")
	}
	sv, err := saveGeneration(root, now, meta{Text: m.Text, Endpoint: m.Engine, Model: m.Model, Voice: m.Voice,
		Speed: m.Speed, ResponseFormat: "wav"}, wav, w)
	if err != nil {
		return Result{}, fmt.Errorf("saving the clip in outputs/: %w", err)
	}
	return Result{Folder: sv.Folder, Wav: sv.Wav, PNG: sv.PNG, JSON: sv.JSON, Seconds: w.seconds(), Bytes: len(wav)}, nil
}
