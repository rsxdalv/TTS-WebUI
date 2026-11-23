package appenv

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetModelLocations(t *testing.T) {
	h, root, _ := envAPI(t)
	get := func(token string) (int, string) {
		r := httptest.NewRequest("GET", APIPrefix+"model-locations", nil)
		r.RemoteAddr = "127.0.0.1:4000"
		if token != "" {
			r.Header.Set(TokenHeader, token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	if code, _ := get(""); code != 403 {
		t.Fatalf("no token: %d", code)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("HF_HOME=/data/hf\n# TORCH_HOME=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, body := get("tok")
	if code != 200 || !strings.Contains(body, `"hf_home":"/data/hf"`) || !strings.Contains(body, `"torch_home":""`) {
		t.Fatalf("%d %s", code, body)
	}
}
