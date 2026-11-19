//go:build windows

package react_ui_backend

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// shortName returns the 8.3 alias of path's last part, or "" when the volume
// keeps no 8.3 names.
func shortName(t *testing.T, path string) string {
	t.Helper()
	in, _ := windows.UTF16PtrFromString(path)
	buf := make([]uint16, 520)
	n, err := windows.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return ""
	}
	short := filepath.Base(windows.UTF16ToString(buf[:n]))
	if strings.EqualFold(short, filepath.Base(path)) {
		return ""
	}
	return short
}

func TestDotfilesStayHiddenBehind83Names(t *testing.T) {
	root := exportFixture(t)
	hidden := filepath.Join(root, "react-ui", "public", ".hidden")
	if err := os.WriteFile(hidden, []byte("SECRET-DOTFILE"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(root, "outputs", ".secret-review")
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("SECRET-OUTPUT"), 0o644); err != nil {
		t.Fatal(err)
	}
	publicShort, outputShort := shortName(t, hidden), shortName(t, secret)
	if publicShort == "" || outputShort == "" {
		t.Skip("this volume keeps no 8.3 names")
	}

	h, err := NewHandler(Config{Root: root, Port: testPort, Static: true, Routes: routes(t, "webui-generations")})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	defer front.Close()

	for _, p := range []string{
		"/" + publicShort,
		"/" + strings.ToLower(publicShort),
		"/api/webui-generations/outputs/" + outputShort,
		"/api/webui-generations/outputs/" + strings.ToLower(outputShort),
	} {
		resp := request(t, "GET", front.URL+p, nil, nil)
		if got := body(t, resp); resp.StatusCode != 404 || strings.Contains(got, "SECRET") {
			t.Errorf("%s: %d %q; a dotfile was served through its 8.3 name", p, resp.StatusCode, got)
		}
	}

	// An ordinary file keeps working through its alias-free name.
	if resp := request(t, "GET", front.URL+"/robots.txt", nil, nil); resp.StatusCode != 200 {
		t.Fatalf("robots.txt: %d", resp.StatusCode)
	}
}
