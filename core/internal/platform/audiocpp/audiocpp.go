// Package audiocpp is the audiocpp platform (P7, D2, D23): the audio.cpp server
// binary, pinned and checksummed, and its models as entries. Nothing here is
// Python. Everything lives in the install folder and goes away by deleting it:
//
//	platforms/audiocpp/bin/            the release's files (audiocpp_server, its DLLs)
//	platforms/audiocpp/platform.json   written last: no file, not installed
//	platforms/audiocpp/models/<dir>/   one folder per model (model.json written last)
//	platforms/audiocpp/server/         the server's config and log
package audiocpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Name is the platform's name.
const Name = "audiocpp"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// Row is the build to install (RowCPU or RowVulkan). Once installed it is read
	// from platform.json.
	Row string
	// Env (KEY=VALUE) is added to the server's environment.
	Env []string
	// Archive replaces the pinned archive and Models the pinned model list (tests
	// serve fake ones).
	Archive *Archive
	Models  []Model
	Client  *http.Client
	// GOOS and GOARCH default to the running machine.
	GOOS, GOARCH string
}

// Platform is the audiocpp platform.
type Platform struct {
	root         paths.Root
	row          string
	env          []string
	archive      *Archive
	models       []Model
	client       *http.Client
	goos, goarch string
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, row: o.Row, env: append([]string(nil), o.Env...), archive: o.Archive,
		models: o.Models, client: o.Client, goos: o.GOOS, goarch: o.GOARCH}
	if p.models == nil {
		p.models = Models
	}
	if p.goos == "" {
		p.goos = runtime.GOOS
	}
	if p.goarch == "" {
		p.goarch = runtime.GOARCH
	}
	return p
}

// Supported reports whether this OS and architecture have a pinned build to install.
func (p *Platform) Supported() bool {
	if p.archive != nil {
		return true
	}
	_, err := ArchiveFor(p.goos, p.goarch, RowCPU)
	return err == nil
}

// WithRow returns a copy that installs this row.
func (p *Platform) WithRow(row string) *Platform { q := *p; q.row = row; return &q }

// WithEnv returns a copy whose commands also carry env.
func (p *Platform) WithEnv(env []string) platform.Platform {
	q := *p
	q.env = append(append([]string(nil), p.env...), env...)
	return &q
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{Name} }

// Dir is platforms/audiocpp.
func (p *Platform) Dir() string { return filepath.Join(p.root.Dir(), "platforms", Name) }

// BinDir holds the release's files.
func (p *Platform) BinDir() string { return filepath.Join(p.Dir(), "bin") }

// ModelsDir holds one folder per model.
func (p *Platform) ModelsDir() string { return filepath.Join(p.Dir(), "models") }

// ServerDir holds the generated config and the log.
func (p *Platform) ServerDir() string { return filepath.Join(p.Dir(), "server") }

// ServerPath is the audio.cpp server program.
func (p *Platform) ServerPath() string {
	name := "audiocpp_server"
	if p.goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.BinDir(), name)
}

func (p *Platform) stateFile() string { return filepath.Join(p.Dir(), "platform.json") }

// State is platforms/audiocpp/platform.json.
type State struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Row         string `json:"row"`
	SHA256      string `json:"sha256"`
	InstalledAt string `json:"installed_at"`
}

// Info reads platform.json; ok is false when the platform is not installed.
func (p *Platform) Info() (State, bool) {
	b, err := os.ReadFile(p.stateFile())
	if err != nil {
		return State{}, false
	}
	var s State
	if json.Unmarshal(b, &s) != nil || s.Name != Name {
		return State{}, false
	}
	return s, true
}

// Row is the installed build, or the one this value was made with while it is
// not installed yet.
func (p *Platform) RowName() string {
	if s, ok := p.Info(); ok {
		return s.Row
	}
	return p.row
}

// Status is Ready when platform.json is there (it is written last), says the
// pinned version, and the server program is still where it was left.
func (p *Platform) Status(context.Context) platform.Status {
	s, ok := p.Info()
	if !ok {
		if _, err := os.Stat(p.Dir()); err == nil {
			return platform.Status{Detail: "the audiocpp platform install did not finish: install it again"}
		}
		return platform.Status{Detail: "the audiocpp platform is not installed"}
	}
	if s.Version != Version && p.archive == nil {
		return platform.Status{Detail: fmt.Sprintf("the audiocpp platform is audio.cpp %s and this version of the app pins %s (repair it)", s.Version, Version)}
	}
	if _, err := os.Stat(p.ServerPath()); err != nil {
		return platform.Status{Detail: "the audiocpp platform is damaged: " + p.ServerPath() + " is missing (repair it)"}
	}
	return platform.Status{Ready: true}
}

// Install downloads the pinned build, checks its sha256, unpacks it beside bin/
// and swaps it in, then writes platform.json. It can be run again after a cancel
// or a failure. The row must be set first.
func (p *Platform) Install(ctx context.Context, emit platform.Emit) error {
	row := p.row
	if row == "" {
		row = RowCPU
	}
	a := p.archive
	if a == nil {
		pinned, err := ArchiveFor(p.goos, p.goarch, row)
		if err != nil {
			return err
		}
		a = &pinned
	}
	if err := os.MkdirAll(p.Dir(), 0o755); err != nil {
		return err
	}
	if s, ok := p.Info(); ok && s.Version == Version && s.Row == row && s.SHA256 == a.SHA256 {
		if _, err := os.Stat(p.ServerPath()); err == nil {
			emit("audio.cpp " + Version + " (" + row + ") is already installed")
			return nil
		}
	}
	_ = os.Remove(p.stateFile()) // a repair is "not installed" until it is done
	dest := filepath.Join(p.Dir(), "audio.zip")
	emit(fmt.Sprintf("Downloading audio.cpp %s (%s build) from %s", Version, row, a.URL))
	last := -1
	err := download(ctx, p.client, a.URL, a.SHA256, dest, func(done, total int64) {
		if pc := int(done * 100 / total); pc != last {
			last = pc
			emit(fmt.Sprintf("audio.cpp: %d%% of %d MB", pc, total>>20))
		}
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return fmt.Errorf("cancelled: the audiocpp platform was not finished (install again to continue)")
		}
		return err
	}
	defer os.Remove(dest)
	emit("Checksum matches " + a.SHA256)
	tmp := p.BinDir() + ".new"
	_ = os.RemoveAll(tmp)
	if err := extractZip(dest, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if _, err := os.Stat(filepath.Join(tmp, filepath.Base(p.ServerPath()))); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("the archive has no %s", filepath.Base(p.ServerPath()))
	}
	ServerFor(p.root).Stop()
	if err := os.RemoveAll(p.BinDir()); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("cannot replace %s (is the audio.cpp server running? stop it first): %w", p.BinDir(), err)
	}
	if err := os.Rename(tmp, p.BinDir()); err != nil {
		return err
	}
	st := State{Name: Name, Version: Version, Row: row, SHA256: a.SHA256, InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(p.stateFile(), b, 0o644); err != nil {
		return err
	}
	emit("The audiocpp platform is installed")
	return nil
}

// Repair is Install again.
func (p *Platform) Repair(ctx context.Context, emit platform.Emit) error {
	if s, ok := p.Info(); ok && p.row == "" {
		p = p.WithRow(s.Row)
	}
	return p.Install(ctx, emit)
}

// Uninstall removes platforms/audiocpp, models included. The caller refuses
// while a model entry needs it and stops the server first.
func (p *Platform) Uninstall(_ context.Context, emit platform.Emit) error {
	ServerFor(p.root).Stop()
	if err := os.RemoveAll(p.Dir()); err != nil {
		return err
	}
	emit("The audiocpp platform is removed")
	return nil
}

// Command resolves a logical program: "server" is the audio.cpp server. A model
// is not a process of its own: the one server serves every installed model.
func (p *Platform) Command(_ *platform.Entry, argv []string) (platform.Command, error) {
	if len(argv) == 0 {
		return platform.Command{}, errors.New("empty command")
	}
	if argv[0] != "server" {
		return platform.Command{}, fmt.Errorf("the audiocpp platform does not provide %q", argv[0])
	}
	return platform.Command{Program: p.ServerPath(), Args: append([]string(nil), argv[1:]...),
		Env: append(os.Environ(), p.env...), Dir: p.BinDir()}, nil
}
