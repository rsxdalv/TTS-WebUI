package openai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The layout is the one the Python generators write (decorator_add_base_filename,
// decorator_save_wav, decorator_save_metadata): outputs/<name>/<name>.wav, .json
// and .png, with <name> = <date>__<model>__<title>. History lists a folder for
// its .wav and the PNG, and the Outputs page reads the .json for its "date".

// promptToTitle is tts_webui/utils/prompt_to_title.py, character for character:
// the replacements in that order, then the first 15 characters.
func promptToTitle(s string) string {
	for _, c := range []string{" ", ":", "'", "\"", "\\", ".", ",", "(", ")", "?", "!", "/", "\n"} {
		s = strings.ReplaceAll(s, c, "_")
	}
	s = strings.ReplaceAll(s, "__", "_")
	if r := []rune(s); len(r) > 15 {
		s = string(r[:15])
	}
	return s
}

// saved is where a clip went, relative to the install folder (with /).
type saved struct {
	Folder string // outputs/<name>
	Wav    string // outputs/<name>/<name>.wav
	PNG    string
	JSON   string
}

type meta struct {
	Text           string
	Endpoint       string
	Model          string
	Voice          string
	Speed          float64
	ResponseFormat string
	// Label replaces "openai_<endpoint>" in the folder name when another platform
	// saves a clip (SaveClip).
	Label string
}

// saveGeneration writes the clip, its metadata and its waveform into
// <root>/outputs. The folder is built under a hidden name and renamed when it
// is complete, so History never lists half a generation.
func saveGeneration(root string, now time.Time, m meta, wavBytes []byte, w wavInfo) (saved, error) {
	png, err := w.waveformPNG()
	if err != nil {
		return saved{}, err
	}
	label := "openai_" + m.Endpoint
	if m.Label != "" {
		label = m.Label
	}
	base := fmt.Sprintf("%s__%s__%s", now.Format("2006-01-02_15-04-05"),
		promptToTitle(label), promptToTitle(m.Text))
	outputs := filepath.Join(root, "outputs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		return saved{}, err
	}
	name := base
	for n := 2; ; n++ {
		if _, err := os.Lstat(filepath.Join(outputs, name)); os.IsNotExist(err) {
			break
		} else if err != nil {
			return saved{}, err
		}
		name = fmt.Sprintf("%s_%d", base, n)
	}
	sum := sha256.Sum256(w.data)
	// The fields, in the order of the legacy file: version, hash version, the
	// request, outputs, date, hash (decorator_save_metadata._add_metadata).
	doc := orderedJSON{
		{"_version", "0.0.1"}, {"_hash_version", "0.0.2"},
		{"text", m.Text}, {"endpoint", m.Endpoint}, {"model", m.Model}, {"voice", m.Voice},
		{"speed", m.Speed}, {"response_format", m.ResponseFormat},
		{"outputs", nil},
		{"date", now.Format("2006-01-02 15:04:05.000000")},
		{"hash", hex.EncodeToString(sum[:])},
	}
	metaBytes, err := doc.marshal()
	if err != nil {
		return saved{}, err
	}
	tmp, err := os.MkdirTemp(outputs, ".incoming-")
	if err != nil {
		return saved{}, err
	}
	defer os.RemoveAll(tmp)
	for ext, b := range map[string][]byte{".wav": wavBytes, ".json": metaBytes, ".png": png} {
		if err := os.WriteFile(filepath.Join(tmp, name+ext), b, 0o644); err != nil {
			return saved{}, err
		}
	}
	if err := os.Rename(tmp, filepath.Join(outputs, name)); err != nil {
		return saved{}, err
	}
	rel := "outputs/" + name
	return saved{Folder: rel, Wav: rel + "/" + name + ".wav", PNG: rel + "/" + name + ".png", JSON: rel + "/" + name + ".json"}, nil
}

// orderedJSON is an object that keeps its field order, indented by two spaces
// like json.dump(indent=2) in the legacy decorator.
type orderedJSON []struct {
	K string
	V any
}

func (o orderedJSON) marshal() ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, kv := range o {
		k, err := json.Marshal(kv.K)
		if err != nil {
			return nil, err
		}
		v, err := marshalNoEscape(kv.V)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&sb, "  %s: %s", k, v)
		if i < len(o)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return []byte(sb.String()), nil
}

func marshalNoEscape(v any) (string, error) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
