package history

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// mk builds a Gradio object marker; the Python bridge turns it into the
// component (tts_webui/core_bridge/gradio_resolve.py).
func mk(kind string, props map[string]any) map[string]any {
	return map[string]any{"__gradio__": kind, "props": props}
}

// Select describes what the Gallery History tab shows when the user picks an
// item: one marker per output component, and "folder_root" as a plain value.
// p is the item's .png or .wav path.
func (s *Store) Select(p string) (map[string]any, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	i := strings.LastIndexByte(p, '.')
	if i < 0 {
		return nil, invalidf("expected a .wav or .png path")
	}
	switch strings.ToLower(p[i:]) {
	case ".wav", ".png":
	default:
		return nil, invalidf("expected a .wav or .png path")
	}
	_, segs, top, err := s.resolve(p[:i] + ".wav")
	if err != nil {
		return nil, err
	}
	if len(segs) < top+2 {
		return nil, invalidf("%q is not inside a generation directory", p)
	}
	wav := strings.Join(segs, "/")
	meta, err := s.ItemJSON(wav)
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(wav, ".wav")
	folder := pathpkg.Dir(wav)
	// The image is only shown when there is one: the History tab lists every
	// generation folder, not only the ones with a picture.
	image := mk("Image", map[string]any{"value": nil})
	if _, _, _, err := s.resolve(stem + ".png"); err == nil {
		image = mk("Image", map[string]any{"value": stem + ".png"})
	}
	return map[string]any{
		"bundle_name":       mk("Markdown", map[string]any{"value": folder}),
		"folder_root":       folder,
		"audio":             mk("Audio", map[string]any{"value": wav, "label": wav}),
		"image":             image,
		"json":              mk("JSON", map[string]any{"value": json.RawMessage(meta)}),
		"npz":               mk("Textbox", map[string]any{"value": stem + ".npz"}),
		"delete":            mk("Button", map[string]any{"visible": true}),
		"save_to_favorites": mk("Button", map[string]any{"visible": segs[0] != "favorites", "value": "Save to favorites"}),
		"save_to_voices":    mk("Button", map[string]any{"visible": true, "value": "Save to voices"}),
		"open_folder":       mk("Button", map[string]any{"visible": true}),
	}, nil
}

// SaveToVoices copies an item's .npz into voices/, replacing a file of the
// same name (shutil.copy did). The source must be a regular .npz inside a
// generation directory.
func (s *Store) SaveToVoices(npz string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, segs, top, err := s.resolve(npz)
	if err != nil {
		return err
	}
	if len(segs) < top+2 || !strings.EqualFold(pathpkg.Ext(segs[len(segs)-1]), ".npz") {
		return invalidf("%q is not an .npz file inside a generation directory", npz)
	}
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errNotFound, npz)
	}
	rootReal, err := filepath.EvalSymlinks(s.opts.Root)
	if err != nil {
		return err
	}
	vdir := filepath.Join(rootReal, "voices")
	if vi, err := os.Lstat(vdir); err == nil && vi.Mode()&os.ModeSymlink != 0 {
		return invalidf("voices/ is a link")
	}
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(vdir, ".save-*.tmp")
	if err != nil {
		return err
	}
	if _, err = io.Copy(tmp, in); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(vdir, segs[len(segs)-1]))
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
