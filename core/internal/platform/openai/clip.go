package openai

import "time"

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
