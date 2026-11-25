package audiocpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// ModelFile is written last in a model's folder: no file, not installed.
const ModelFile = "model.json"

type modelState struct {
	Package     string `json:"package"`
	ID          string `json:"id"`
	Platform    string `json:"audio_cpp"`
	InstalledAt string `json:"installed_at"`
}

// ModelDir is models/<dir> of a model.
func (p *Platform) ModelDir(m Model) string { return filepath.Join(p.ModelsDir(), m.Dir) }

// ModelList is the pinned models.
func (p *Platform) ModelList() []Model { return p.models }

func (p *Platform) modelFor(pkg string) (Model, bool) {
	for _, m := range p.models {
		if m.Package == pkg {
			return m, true
		}
	}
	return Model{}, false
}

// Create downloads a model's files, each checked against its sha256, then writes
// model.json. Files that are already there with the right size are kept (a file
// gets its final name only after its checksum matched), so a cancelled install
// continues where it stopped.
func (p *Platform) Create(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	fail := func(format string, a ...any) bool { emit("Failed: " + fmt.Sprintf(format, a...)); return false }
	m, ok := p.modelFor(e.Package)
	if !ok {
		return fail("%s is not an audio.cpp model this version knows", e.Label())
	}
	if st := p.Status(ctx); !st.Ready {
		return fail("%s requires audiocpp (%s)", e.Label(), st.Detail)
	}
	dir := p.ModelDir(m)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail("%v", err)
	}
	_ = os.Remove(filepath.Join(dir, ModelFile))
	for i, f := range m.Files {
		dest := filepath.Join(dir, filepath.FromSlash(f.Rel))
		if st, err := os.Stat(dest); err == nil && st.Size() == f.Size {
			emit(fmt.Sprintf("%s: already there (file %d of %d)", f.Rel, i+1, len(m.Files)))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fail("%v", err)
		}
		emit(fmt.Sprintf("Downloading %s (file %d of %d, %d MB)", f.Rel, i+1, len(m.Files), f.Size>>20))
		last := -1
		err := download(ctx, p.client, f.URL, f.SHA256, dest, func(done, total int64) {
			if pc := int(done * 100 / total); pc != last {
				last = pc
				emit(fmt.Sprintf("%s: %d%% of %d MB", f.Rel, pc, total>>20))
			}
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				emit(fmt.Sprintf("Cancelled: %s was not finished (install it again to continue; finished files are kept)", e.Label()))
				return false
			}
			return fail("%v", err)
		}
		emit("Checksum matches " + f.SHA256)
	}
	b, _ := json.MarshalIndent(modelState{Package: m.Package, ID: m.ID, Platform: Version,
		InstalledAt: time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, ModelFile), b, 0o644); err != nil {
		return fail("%v", err)
	}
	emit(e.Label() + " is installed")
	return true
}

// Check says what Create would do.
func (p *Platform) Check(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	m, ok := p.modelFor(e.Package)
	if !ok {
		emit(e.Label() + " is not an audio.cpp model this version knows")
		return false
	}
	emit(fmt.Sprintf("%s: %d files, %d MB, from Hugging Face audio-cpp/audio.cpp-gguf", e.Label(), len(m.Files), m.SizeMB()))
	return true
}

// EntryStatus is installed when model.json is there and the GGUF is too; a
// folder without them is partial.
func (p *Platform) EntryStatus(_ context.Context, e platform.Entry) platform.EntryStatus {
	m, ok := p.modelFor(e.Package)
	if !ok {
		return platform.EntryStatus{}
	}
	dir := p.ModelDir(m)
	if _, err := os.Stat(dir); err != nil {
		return platform.EntryStatus{}
	}
	if _, err := os.Stat(filepath.Join(dir, ModelFile)); err == nil {
		if _, err := os.Stat(filepath.Join(dir, m.GGUF)); err == nil {
			return platform.EntryStatus{Installed: true}
		}
	}
	return platform.EntryStatus{Partial: true}
}

// Remove deletes the model's folder. The server is stopped first by the caller
// (a running server holds the files open).
func (p *Platform) Remove(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	m, ok := p.modelFor(e.Package)
	if !ok {
		emit("Failed: " + e.Label() + " is not an audio.cpp model this version knows")
		return false
	}
	ServerFor(p.root).Stop() // a running server holds the files open
	if err := os.RemoveAll(p.ModelDir(m)); err != nil {
		emit("Failed: " + err.Error())
		return false
	}
	emit(e.Label() + " is removed")
	return true
}
