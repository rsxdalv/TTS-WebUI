package uv

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Prefetching the requirements' wheels while torch downloads.
//
// torch is one wheel of several GB from one server and takes most of an
// install; the other packages come from PyPI. uv installs torch first because
// the requirements must be resolved with it in place, but their wheels do not
// care: this puts them into the wheel cache at the same time, so the real
// requirements step finds them there. It is only an optimisation. Anything that
// goes wrong in it is a line in the log, and the requirements step downloads
// what is missing as it always did.
//
// How: `uv pip compile` pins the whole requirement set, the torch family is cut
// out of the pins (their wheels come from the torch row's index, and a
// PyPI one would be the wrong build), and `uv pip install --no-deps --target`
// into a scratch folder downloads and unpacks the rest into the cache.

// prefetchSkip matches the pins that are not prefetched: torch and what ships
// with it, which come from the row's own index.
var prefetchSkip = regexp.MustCompile(`(?i)^(torch|torchvision|torchaudio|torchcodec|triton|xformers|nvidia-[a-z0-9-]+)(\s|=|<|>|@|~|!|$)`)

// prefetchEnabled says whether the platform prefetches: not for a stand-in uv
// (the tests count its calls) unless asked, and not when TTS_PREFETCH_WHEELS=0.
func (p *Platform) prefetchEnabled() bool {
	if os.Getenv("TTS_PREFETCH_WHEELS") == "0" {
		return false
	}
	return (len(p.prefix) == 0 && p.archive == nil) || p.prefetch
}

// requirementLines reads the requirement tokens of an install (the words after
// `uv pip install`) as the lines of a requirements file: `-r file` is read in
// (relative to the install folder), every other word is a requirement. A token
// that is an option (an index, a constraint) makes the set one this does not
// handle, and ok is false.
func requirementLines(root string, toks []string) (lines []string, ok bool) {
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t == "-r" || t == "--requirement":
			if i+1 >= len(toks) {
				return nil, false
			}
			i++
			path := toks[i]
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			f, err := os.Open(path)
			if err != nil {
				return nil, false
			}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				l := strings.TrimSpace(sc.Text())
				if strings.HasPrefix(l, "-") { // an option or an include inside the file
					f.Close()
					return nil, false
				}
				lines = append(lines, l)
			}
			f.Close()
		case strings.HasPrefix(t, "-"):
			return nil, false
		default:
			lines = append(lines, t)
		}
	}
	return lines, true
}

// prefetchPins keeps the pins that are prefetched.
func prefetchPins(pinned string) []string {
	var out []string
	for _, l := range strings.Split(pinned, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || prefetchSkip.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// prefetchWheels fills the wheel cache with the requirements of an install. It
// reports the "wheels" step, never fails the install, and returns when done or
// when ctx ends.
func (p *Platform) prefetchWheels(ctx context.Context, id, pkg string, toks []string, pyExe string, emit platform.Emit) {
	skip := func(why string) {
		emit("Not prefetching the requirements' wheels: " + why)
		platform.Step(ctx, platform.StepEvent{ID: id, State: platform.StepSkipped, Detail: why})
	}
	lines, ok := requirementLines(p.root.Dir(), toks)
	if !ok {
		skip("the requirements use options it does not handle")
		return
	}
	tmp, err := os.MkdirTemp(p.CacheDir(), ".prefetch-")
	if err != nil {
		skip(err.Error())
		return
	}
	defer removeAll(tmp)
	in := filepath.Join(tmp, "requirements.in")
	if err := os.WriteFile(in, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		skip(err.Error())
		return
	}
	cache := []string{"--cache-dir", p.CacheDir(), "--no-config"}
	pinnedFile := filepath.Join(tmp, "pinned.txt")
	platform.Step(ctx, platform.StepEvent{ID: id, State: platform.StepRunning, Detail: "resolving the requirements"})
	quiet := func(string) {}
	compile := append(append([]string{"pip", "compile", "--python", pyExe}, cache...), "--no-header", "--no-annotate", in, "-o", pinnedFile)
	if err := p.run(ctx, quiet, compile...); err != nil {
		if ctx.Err() == nil {
			skip("could not resolve them: " + err.Error())
		}
		return
	}
	b, err := os.ReadFile(pinnedFile)
	if err != nil {
		skip(err.Error())
		return
	}
	pins := prefetchPins(string(b))
	if len(pins) == 0 {
		skip("nothing to fetch")
		return
	}
	list := filepath.Join(tmp, "prefetch.txt")
	if err := os.WriteFile(list, []byte(strings.Join(pins, "\n")+"\n"), 0o644); err != nil {
		skip(err.Error())
		return
	}
	emit(fmt.Sprintf("Fetching %d requirement wheels into the cache while torch downloads", len(pins)))
	obs := &uvLines{ctx: ctx, id: id}
	install := append(append([]string{"pip", "install", "--python", pyExe}, cache...), "--no-deps", "--target", filepath.Join(tmp, "target"), "-r", list)
	if err := p.run(ctx, func(l string) { obs.observe(l) }, install...); err != nil {
		if ctx.Err() == nil {
			skip("could not fetch them: " + err.Error())
		}
		return
	}
	platform.Step(ctx, platform.StepEvent{ID: id, State: platform.StepDone, Detail: fmt.Sprintf("%d wheels are in the cache", len(pins))})
}
