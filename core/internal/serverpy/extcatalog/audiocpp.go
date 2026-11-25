package extcatalog

import (
	"context"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
)

// The audiocpp platform (P7): the audio.cpp server and the models it serves. A
// model is a catalog entry that requires "audiocpp" and is named in
// audiocpp.Models; the platform is installed before the first model, from the
// same install call, like uv.

// audioWould is what the Extensions page says installing the platform does.
const audioWould = "install the audiocpp platform: audio.cpp " + audiocpp.Version +
	" (a build you choose: CPU 34 MB, Vulkan 84 MB), checked against its sha256"

// requiresAudio reports whether an entry is an audio.cpp model: it requires the
// audiocpp platform.
func requiresAudio(e *Obj) bool {
	req, err := entryRequires(e)
	if err != nil {
		return false
	}
	for _, n := range req {
		if n == audiocpp.Name {
			return true
		}
	}
	return false
}

// chooseAudioRow sets pl.AudioRow when the plan installs the audiocpp platform.
// No choice means the CPU build, which runs anywhere (the install line says so);
// a row that is given but not used is refused rather than ignored.
func (s *Store) chooseAudioRow(pl *plan, given string) error {
	need := false
	for _, p := range pl.Platforms {
		need = need || p == audiocpp.Name
	}
	if !need {
		if given != "" {
			return invalidf("audiocpp_row is only for an install that installs the audiocpp platform")
		}
		return nil
	}
	if given == "" {
		given = audiocpp.RowCPU
	}
	if !audiocpp.ValidRow(given) {
		return invalidf("audiocpp_row must be one of cpu, vulkan")
	}
	pl.AudioRow = given
	return nil
}

// installAudioCpp is the audiocpp platform step of a plan.
func (s *Store) installAudioCpp(ctx context.Context, row string, emit func(string)) bool {
	if s.opts.AudioCpp == nil {
		emit("Failed: the audiocpp platform cannot be installed here")
		return false
	}
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	rec("Installing the " + row + " build")
	err := s.opts.AudioCpp.WithRow(row).Install(ctx, rec)
	if err != nil {
		if ctx.Err() != nil {
			rec("Cancelled: the audiocpp platform was not finished (install again to continue)")
		} else {
			rec("Failed to install the audiocpp platform: " + err.Error())
		}
	}
	s.writeLog("audiocpp-platform-install", "audiocpp", log)
	return err == nil
}

// audioModelSetup installs one model's files on the platform.
func (s *Store) audioModelSetup(ctx context.Context, entry *Obj, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	if s.opts.AudioCpp == nil {
		rec("Failed: the audiocpp platform is not available")
		return false
	}
	e := s.platformEntry(entry, nil)
	ok := s.opts.AudioCpp.Create(ctx, e, rec)
	s.writeLog("audiocpp-model-install", e.Name, log)
	return ok
}

// audioModelRemove deletes one model's files (the server is stopped first).
func (s *Store) audioModelRemove(ctx context.Context, entry *Obj, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	if s.opts.AudioCpp == nil {
		rec("Failed: the audiocpp platform is not available")
		return false
	}
	e := s.platformEntry(entry, nil)
	ok := s.opts.AudioCpp.Remove(ctx, e, rec)
	s.writeLog("audiocpp-model-uninstall", e.Name, log)
	return ok
}

// serveAudioCpp is serveInstall for the platform itself: it is not in the
// catalog, so it has its own checks, then the same lock and stream as any entry.
func (s *Store) serveAudioCpp(w http.ResponseWriter, r *http.Request, op string, o *Obj) {
	if s.opts.AudioCpp == nil {
		s.fail(w, invalidf("the audiocpp platform is not available here"))
		return
	}
	if op != "install" && op != "uninstall" {
		s.fail(w, invalidf("the audiocpp platform has no %s", op))
		return
	}
	ready := s.opts.AudioCpp.Status(r.Context()).Ready
	var pl plan
	var err error
	if op == "install" {
		if ready {
			err = invalidf("the audiocpp platform is already installed")
		} else {
			pl = plan{Platforms: []string{audiocpp.Name}, Total: 1}
			err = s.chooseAudioRow(&pl, o.Str("audiocpp_row"))
		}
	} else {
		var g = s.opts.AudioCpp
		if _, installed := g.Info(); !installed {
			err = invalidf("the audiocpp platform is not installed")
		} else if gr, gerr := s.RequiresGraph(r.Context()); gerr != nil {
			err = gerr
		} else {
			err = refuseIfRequired(gr, audiocpp.Name)
		}
	}
	if err != nil {
		s.failInstall(w, err)
		return
	}
	if !s.install.TryLock() {
		s.fail(w, errBusy)
		return
	}
	defer s.install.Unlock()
	s.stream(w, r, func(ctx context.Context, emit func(string)) bool {
		if op == "uninstall" {
			var log []string
			rec := func(l string) { log = append(log, l); emit(l) }
			err := s.opts.AudioCpp.Uninstall(ctx, rec)
			if err != nil {
				rec("Failed to remove the audiocpp platform: " + err.Error())
			}
			s.writeLog("audiocpp-platform-uninstall", "audiocpp", log)
			return err == nil
		}
		return s.runSteps(ctx, pl, nil, emit)
	})
}

// audioEntryStatus is whether a model entry is installed on the platform.
func (s *Store) audioEntryStatus(ctx context.Context, pkg string) bool {
	if s.opts.AudioCpp == nil {
		return false
	}
	return s.opts.AudioCpp.EntryStatus(ctx, platform.Entry{Package: pkg}).Installed
}

// AudioCppSupported is whether this machine has a pinned audio.cpp build.
func (s *Store) AudioCppSupported() bool {
	return s.opts.AudioCpp != nil && s.opts.AudioCpp.Supported()
}

// AudioCppReady is whether the audiocpp platform is installed.
func (s *Store) AudioCppReady(ctx context.Context) bool {
	return s.opts.AudioCpp != nil && s.opts.AudioCpp.Status(ctx).Ready
}
