package history

import (
	"encoding/json"
	"errors"
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

// Select describes what the History and Gallery History tabs show when the
// user picks an item: one marker per output component, and "folder_root" as a
// plain value. p is the item's .png or .wav path.
//
// A generation that is not complete is still answered: a folder without a .wav
// gets the controls (the audio keeps the path it would have), a folder
// without a .json gets an empty JSON component, and a folder without a .png
// gets no image. The old Python tab showed the same, so a broken folder can
// still be selected and deleted. A path that leaves the collection is an
// error, like everywhere else.
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
	stem := p[:i]
	base := pathpkg.Base(stem)
	// The folder has to exist and to be inside a collection: that is where the
	// path rules are checked.
	_, segs, top, err := s.resolve(pathpkg.Dir(stem))
	if err != nil {
		return nil, err
	}
	if len(segs) < top+1 {
		return nil, invalidf("%q is not inside a generation directory", p)
	}
	folder := strings.Join(segs, "/")
	item := folder + "/" + base
	wav := item + ".wav"
	// The .wav may be missing (a broken generation): that is not an error,
	// but a file that leaves the collection is still refused.
	if _, _, _, err := s.resolve(wav); err != nil && !errors.Is(err, errNotFound) {
		return nil, err
	}
	meta, err := s.ItemJSON(wav)
	if err != nil {
		return nil, err
	}
	// The image is only shown when there is one: the History tab lists every
	// generation folder, not only the ones with a picture.
	image := mk("Image", map[string]any{"value": nil})
	if _, _, _, err := s.resolve(item + ".png"); err == nil {
		image = mk("Image", map[string]any{"value": item + ".png"})
	}
	return map[string]any{
		"bundle_name":       mk("Markdown", map[string]any{"value": folder}),
		"folder_root":       folder,
		"audio":             mk("Audio", map[string]any{"value": wav, "label": wav}),
		"image":             image,
		"json":              mk("JSON", map[string]any{"value": json.RawMessage(meta)}),
		"npz":               mk("Textbox", map[string]any{"value": item + ".npz"}),
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
