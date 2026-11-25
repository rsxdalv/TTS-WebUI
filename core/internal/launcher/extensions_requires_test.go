package launcher_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

type reqRow struct {
	PackageName       string   `json:"package_name"`
	Requires          []string `json:"requires"`
	RequirementsState []struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		State string `json:"state"`
		Via   string `json:"via"`
		Would string `json:"would"`
	} `json:"requirements_state"`
}

type reqAnswer struct {
	Platform       string   `json:"platform"`
	PlatformReady  bool     `json:"platform_ready"`
	PlatformDetail string   `json:"platform_detail"`
	RequiresError  string   `json:"requires_error"`
	Extensions     []reqRow `json:"extensions"`
}

func listRequires(t *testing.T, root paths.Root, srv *server.Server, token string) map[string]reqRow {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root.Dir(), "extensions.json"), []byte(extList), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doRequest(t, srv, token, http.MethodGet, "/api/ext/list", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	var ans reqAnswer
	if err := json.NewDecoder(resp.Body).Decode(&ans); err != nil {
		t.Fatal(err)
	}
	if ans.RequiresError != "" {
		t.Fatalf("requires error: %s", ans.RequiresError)
	}
	rows := map[string]reqRow{}
	for _, e := range ans.Extensions {
		rows[e.PackageName] = e
	}
	t.Logf("platform=%s ready=%v detail=%q", ans.Platform, ans.PlatformReady, ans.PlatformDetail)
	if ans.Platform != "legacy-env" {
		t.Fatalf("platform = %q", ans.Platform)
	}
	return rows
}

// A new folder (no micromamba, no env): each entry shows what it requires, and
// says what would be installed.
func TestExtensionsListNewFolderSaysWhatWouldBeInstalled(t *testing.T) {
	root, srv, token := newTestInstall(t)
	rows := listRequires(t, root, srv, token)
	nat := rows["extension_native"]
	if len(nat.Requires) != 1 || nat.Requires[0] != "uv" || len(nat.RequirementsState) != 1 {
		t.Fatalf("native: %+v", nat)
	}
	if r := nat.RequirementsState[0]; r.State != "missing" || r.Kind != "platform" || !strings.Contains(r.Would, "uv platform") {
		t.Errorf("native uv: %+v", r)
	}
	tab := rows["extension_beta"]
	if len(tab.Requires) != 1 || tab.Requires[0] != "gradio-host" || tab.RequirementsState[0].State != "missing" {
		t.Errorf("tab: %+v", tab)
	}
}

// A legacy install (micromamba and the env are there): the same entries show
// what they require as met by legacy-env.
func TestExtensionsListLegacyInstallShowsRequirementsMet(t *testing.T) {
	root, srv, token := newTestInstall(t)
	mamba := filepath.Join(root.InstallerFiles(), "mamba", "micromamba")
	if runtime.GOOS == "windows" {
		mamba += ".exe"
	}
	for _, d := range []string{filepath.Dir(mamba), filepath.Join(root.InstallerFiles(), "env")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(mamba, []byte("not a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	rows := listRequires(t, root, srv, token)
	for name, want := range map[string]string{"extension_native": "uv", "extension_beta": "gradio-host", "extension_deco": "gradio-host"} {
		r := rows[name]
		if len(r.RequirementsState) != 1 || r.RequirementsState[0].Name != want ||
			r.RequirementsState[0].State != "ready" || r.RequirementsState[0].Via != "legacy-env" {
			t.Errorf("%s: %+v", name, r)
		}
	}
}
