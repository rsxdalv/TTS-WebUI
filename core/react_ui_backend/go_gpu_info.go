package react_ui_backend

// GPU Info (extension_gpu_info) in Go, from nvidia-smi, so the page works with
// no Python and no Gradio. The answer is the array of get_gpu_info: one object
// per GPU with the same keys the page reads.
//
// What nvidia-smi cannot know is filled with neutral values: torch_version is
// "" (no torch in the Go core), used_vram and cached_vram are 0 (they were this
// Python process's torch allocations), multi_processor_count is 0.
// used_vram_total is the memory in use on the device, from nvidia-smi.
// No NVIDIA GPU or no nvidia-smi answers [] (what torch did without CUDA).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "go-gpu-info",
		Patterns: []string{"GET /api/go-data/gpu-info"},
		Handler:  func(Deps) http.Handler { return newGPUInfo(nvidiaSMI) },
	})
}

const gpuQuery = "index,name,memory.total,memory.used,temperature.gpu,power.draw,utilization.gpu,compute_cap"

// smiRunner runs nvidia-smi with args and returns stdout.
type smiRunner func(ctx context.Context, args ...string) ([]byte, error)

func findNvidiaSMI() string {
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p
	}
	for _, p := range []string{
		`C:\Windows\System32\nvidia-smi.exe`,
		`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		"/usr/bin/nvidia-smi",
		"/usr/local/nvidia/bin/nvidia-smi",
	} {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func nvidiaSMI(ctx context.Context, args ...string) ([]byte, error) {
	exe := findNvidiaSMI()
	if exe == "" {
		return nil, exec.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// gpuInfoCache keeps the last answer for a second: the page polls every 2 s
// and several tabs may poll at once.
type gpuInfoCache struct {
	mu      sync.Mutex
	at      time.Time
	value   []map[string]any
	cudaAt  time.Time
	cudaVer string
}

var cudaVersionRE = regexp.MustCompile(`CUDA Version:\s*([0-9.]+)`)

func (c *gpuInfoCache) get(ctx context.Context, run smiRunner) []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.value != nil && time.Since(c.at) < time.Second {
		return c.value
	}
	if time.Since(c.cudaAt) > time.Minute {
		c.cudaAt = time.Now()
		if out, err := run(ctx); err == nil {
			if m := cudaVersionRE.FindSubmatch(out); m != nil {
				c.cudaVer = string(m[1])
			}
		}
	}
	out, err := run(ctx, "--query-gpu="+gpuQuery, "--format=csv,noheader,nounits")
	c.value = []map[string]any{}
	if err == nil {
		c.value = parseGPUCSV(string(out), c.cudaVer)
	}
	c.at = time.Now()
	return c.value
}

// num reads a nvidia-smi number; "[N/A]", "N/A" and junk are 0.
func num(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// parseGPUCSV turns the csv of gpuQuery into the get_gpu_info objects.
func parseGPUCSV(csv, cudaVersion string) []map[string]any {
	out := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(csv), "\n") {
		f := strings.Split(line, ",")
		if len(f) != 8 {
			continue
		}
		capab := []float64{0, 0}
		if parts := strings.SplitN(strings.TrimSpace(f[7]), ".", 2); len(parts) == 2 {
			capab = []float64{num(parts[0]), num(parts[1])}
		}
		out = append(out, map[string]any{
			"idx":                   int(num(f[0])),
			"name":                  strings.TrimSpace(f[1]),
			"vram":                  num(f[2]),
			"used_vram_total":       num(f[3]),
			"used_vram":             0,
			"cached_vram":           0,
			"temperature":           num(f[4]),
			"power_draw":            num(f[5]),
			"utilization":           num(f[6]),
			"cuda_capabilities":     capab,
			"cuda_version":          cudaVersion,
			"torch_version":         "",
			"multi_processor_count": 0,
		})
	}
	return out
}

func newGPUInfo(run smiRunner) http.Handler {
	cache := &gpuInfoCache{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(cache.get(r.Context(), run))
	})
}
