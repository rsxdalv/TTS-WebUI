package react_ui_backend

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseGPUCSV(t *testing.T) {
	got := parseGPUCSV("0, NVIDIA GeForce GTX 1070, 8192, 457, 27, 8.29, 1, 6.1\n1, Tesla T4, 15360, [N/A], N/A, [N/A], 0, 7.5\nbroken line\n", "12.4")
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
	g := got[0]
	if g["name"] != "NVIDIA GeForce GTX 1070" || g["vram"] != 8192.0 || g["used_vram_total"] != 457.0 || g["power_draw"] != 8.29 || g["cuda_version"] != "12.4" {
		t.Fatalf("%v", g)
	}
	if c := g["cuda_capabilities"].([]float64); c[0] != 6 || c[1] != 1 {
		t.Fatalf("capability %v", c)
	}
	if got[1]["used_vram_total"] != 0.0 || got[1]["temperature"] != 0.0 {
		t.Fatalf("N/A must be 0: %v", got[1])
	}
	// The keys of get_gpu_info, all present.
	for _, k := range []string{"torch_version", "cuda_version", "vram", "name", "cuda_capabilities", "used_vram", "used_vram_total", "cached_vram", "idx", "multi_processor_count", "temperature", "power_draw", "utilization"} {
		if _, ok := g[k]; !ok {
			t.Errorf("missing key %s", k)
		}
	}
}

func TestGPUInfoWithoutNvidiaSMI(t *testing.T) {
	h := newGPUInfo(func(context.Context, ...string) ([]byte, error) { return nil, errors.New("not found") })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("%d %q", w.Code, w.Body)
	}
}

func TestGPUInfoCalls(t *testing.T) {
	calls := 0
	h := newGPUInfo(func(_ context.Context, args ...string) ([]byte, error) {
		calls++
		if len(args) == 0 {
			return []byte("| NVIDIA-SMI 582.66   Driver Version: 582.66   CUDA Version: 13.0 |"), nil
		}
		return []byte("0, X, 100, 10, 30, 5, 2, 8.6\n"), nil
	})
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
		if !strings.Contains(w.Body.String(), `"cuda_version":"13.0"`) {
			t.Fatalf("%s", w.Body)
		}
	}
	if calls != 2 {
		t.Fatalf("a burst of requests must reuse the answer, calls=%d", calls)
	}
}
