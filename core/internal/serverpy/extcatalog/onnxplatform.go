package extcatalog

import (
	"context"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/onnx"
)

// onnxWould is what the Extensions page says installing the onnx platform does.
const onnxWould = "install the onnx platform: the sherpa-onnx " + onnx.RuntimeVersion + " runtime (CPU, about 24 MB)"

// installONNX installs the onnx platform.
func (s *Store) installONNX(ctx context.Context, emit func(string)) bool {
	if s.opts.ONNX == nil {
		emit("Failed: the onnx platform cannot be installed here")
		return false
	}
	var log []string
	rec := func(line string) { log = append(log, line); emit(line) }
	err := s.opts.ONNX.Install(ctx, rec)
	if err != nil {
		if ctx.Err() != nil {
			rec("Cancelled: the onnx platform was not finished (install again to continue)")
		} else {
			rec("Failed to install the onnx platform: " + err.Error())
		}
	}
	s.writeLog("onnx-platform-install", "onnx", log)
	return err == nil
}

// isONNX reports whether the entry runs on the onnx platform.
func (s *Store) isONNX(pkg string) bool { return s.opts.ONNX != nil && s.opts.ONNX.IsOnnxEntry(pkg) }

// onnxCreate installs an entry on the onnx platform.
func (s *Store) onnxCreate(ctx context.Context, entry *Obj, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	e := platform.Entry{Package: entry.Str("package_name"), Name: entry.Str("name")}
	ok := s.opts.ONNX.Create(ctx, e, rec)
	s.writeLog("onnx-install", e.Package, log)
	return ok
}

// onnxRemove removes an entry from the onnx platform.
func (s *Store) onnxRemove(ctx context.Context, entry *Obj, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	e := platform.Entry{Package: entry.Str("package_name"), Name: entry.Str("name")}
	ok := s.opts.ONNX.Remove(ctx, e, rec)
	s.writeLog("onnx-uninstall", e.Package, log)
	return ok
}
