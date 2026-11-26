package onnx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// MaxText is the longest input one generation takes (the text goes on the
// program's command line).
const MaxText = 4000

// Request is one generation.
type Request struct {
	Package string  // the entry
	Text    string  // the text to speak
	Voice   int     // speaker id 0..Voices-1
	Speed   float64 // 1 = normal; 0 = normal
}

// ErrInvalid marks a request the caller got wrong (HTTP 400).
var ErrInvalid = errors.New("invalid request")

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

func (p *Platform) check(r Request) (Model, error) {
	m, ok := p.model(r.Package)
	if !ok {
		return m, invalidf("%s is not an onnx entry", r.Package)
	}
	t := strings.TrimSpace(r.Text)
	switch {
	case t == "":
		return m, invalidf("the text is empty")
	case !utf8.ValidString(t) || strings.ContainsRune(t, 0):
		return m, invalidf("the text is not valid UTF-8 text")
	case utf8.RuneCountInString(t) > MaxText:
		return m, invalidf("the text is longer than %d characters", MaxText)
	case strings.HasPrefix(t, "-"):
		// The text is a positional argument of the program; one that starts with
		// a dash would be read as an option.
		return m, invalidf("the text may not start with a dash")
	case r.Voice < 0 || r.Voice >= m.Voices:
		return m, invalidf("voice must be 0 to %d", m.Voices-1)
	case r.Speed != 0 && (r.Speed < 0.25 || r.Speed > 4):
		return m, invalidf("speed must be between 0.25 and 4")
	}
	return m, nil
}

// Generate runs the runtime once and returns the WAV file it wrote. Cancelling
// ctx kills the program; the temporary folder is removed in every case, so no
// clip is left behind.
func (p *Platform) Generate(ctx context.Context, r Request) ([]byte, error) {
	m, err := p.check(r)
	if err != nil {
		return nil, err
	}
	if st := p.Status(ctx); !st.Ready {
		return nil, errors.New(st.Detail)
	}
	if es := p.EntryStatus(ctx, platform.Entry{Package: r.Package}); !es.Installed {
		return nil, fmt.Errorf("%s is not installed", m.Title)
	}
	speed := r.Speed
	if speed == 0 {
		speed = 1
	}
	tmpRoot := filepath.Join(p.Dir(), "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(tmpRoot, "gen-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.wav")
	e := platform.Entry{Package: r.Package}
	c, err := p.Command(&e, []string{"offline-tts",
		fmt.Sprintf("--sid=%d", r.Voice),
		fmt.Sprintf("--kitten-length-scale=%g", 1/speed),
		"--output-filename=" + out,
		strings.TrimSpace(r.Text)})
	if err != nil {
		return nil, err
	}
	var tail []string
	runErr := platform.Run(ctx, c, func(l string) { tail = append(tail, l) })
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if runErr != nil {
		return nil, fmt.Errorf("the onnx runtime failed: %v: %s", runErr, trimTail(tail, 3))
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("the onnx runtime wrote no clip: %s", trimTail(tail, 3))
	}
	return b, nil
}
