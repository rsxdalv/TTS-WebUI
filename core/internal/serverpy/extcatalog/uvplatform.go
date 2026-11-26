package extcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

// uvWould is what the Extensions page says installing the uv platform does.
const uvWould = "install the uv platform: uv " + uv.Version + ", Python " + uv.PythonVersion + " and torch for the GPU you choose"

// gpuOption is one GPU choice the user can make for this OS.
type gpuOption struct {
	Label    string `json:"label"`
	Row      string `json:"row"`
	ApproxMB int    `json:"approx_mb"`
}

// needsGPUError is the answer to an install that needs the uv platform before
// the user has said which GPU it is for (D29): nothing is installed, the
// proposal and the choices are returned so the page can ask.
type needsGPUError struct {
	Proposed string      `json:"proposed"`
	Reason   string      `json:"reason"`
	Choices  []gpuOption `json:"choices"`
}

func (e *needsGPUError) Error() string {
	return "choose the GPU for the uv platform before it is installed (proposed: " + e.Proposed + ")"
}

func gpuOptions() []gpuOption {
	var out []gpuOption
	for _, l := range torchrows.Choices(runtime.GOOS) {
		if r, err := torchrows.Default(l, runtime.GOOS); err == nil {
			out = append(out, gpuOption{Label: string(l), Row: r.Name, ApproxMB: r.ApproxMB})
		}
	}
	return out
}

// proposeGPU is what the page offers the user: the choice an earlier install
// saved in installer_scripts/.gpu when there is one that still has a torch build
// here, otherwise what the machine looks like.
func (s *Store) proposeGPU(ctx context.Context) (gpu.Label, string) {
	if b, err := os.ReadFile(filepath.Join(s.opts.Root, "installer_scripts", ".gpu")); err == nil {
		if l, err := gpu.Parse(string(b)); err == nil {
			if _, err := torchrows.Default(l, runtime.GOOS); err == nil {
				return l, "your earlier choice in installer_scripts/.gpu"
			}
		}
	}
	d := torchrows.DetectHere(ctx)
	return d.Label, d.Reason
}

// chooseGPU sets pl.GPU when the plan installs the uv platform. A choice that
// was not given is not guessed: the install is refused with the proposal.
func (s *Store) chooseGPU(pl *plan, given string) error {
	need := false
	for _, p := range pl.Platforms {
		need = need || p == "uv"
	}
	if !need {
		return nil
	}
	if given == "" {
		l, why := s.proposeGPU(context.Background())
		return &needsGPUError{Proposed: string(l), Reason: why, Choices: gpuOptions()}
	}
	l, err := gpu.Parse(given)
	if err != nil {
		return invalidf("%v", err)
	}
	if _, err := torchrows.Default(l, runtime.GOOS); err != nil {
		return invalidf("%v", err)
	}
	pl.GPU = l
	return nil
}

// failInstall answers a refused install; a missing GPU choice carries what the
// page needs to ask for it.
func (s *Store) failInstall(w http.ResponseWriter, err error) {
	var need *needsGPUError
	if errors.As(err, &need) {
		writeStd(w, http.StatusConflict, map[string]any{
			"error": need.Error(), "needs": "gpu_choice",
			"proposed": need.Proposed, "reason": need.Reason, "choices": need.Choices,
		})
		return
	}
	s.fail(w, err)
}

func writeStd(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// installPlatform installs a platform of a plan.
func (s *Store) installPlatform(ctx context.Context, name string, l gpu.Label, emit func(string)) bool {
	if name == "onnx" {
		return s.installONNX(ctx, emit)
	}
	if name != "uv" || s.opts.UV == nil {
		emit("Failed: the " + name + " platform cannot be installed here")
		return false
	}
	var log []string
	rec := func(line string) { log = append(log, line); emit(line) }
	err := s.opts.UV.WithGPU(l).Install(ctx, rec)
	if err != nil {
		if ctx.Err() != nil {
			rec("Cancelled: the uv platform was not finished (install again to continue; downloads are kept)")
		} else {
			rec("Failed to install the uv platform: " + err.Error())
		}
	}
	s.writeLog("uv-platform-install", "uv", log)
	return err == nil
}

// serveUVPlatform answers what the page needs to show the uv platform and to
// ask for the GPU choice.
func (s *Store) serveUVPlatform(w http.ResponseWriter, r *http.Request) {
	if s.opts.UV == nil {
		writeStd(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	st := s.opts.UV.Status(r.Context())
	ans := map[string]any{"available": true, "installed": st.Ready, "detail": st.Detail,
		"uv_version": uv.Version, "python": uv.PythonVersion, "choices": gpuOptions(),
		"cache_dir": s.opts.UV.CacheDir()}
	if info, ok := s.opts.UV.Info(); ok {
		ans["gpu"], ans["installed_at"], ans["python"] = info.GPU, info.InstalledAt, info.Python
	}
	l, why := s.proposeGPU(r.Context())
	ans["proposed"], ans["reason"] = string(l), why
	writeStd(w, http.StatusOK, ans)
}
