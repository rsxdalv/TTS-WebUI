package platform_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The main env belongs to the legacy-env platform: no other shipped Go code
// may name micromamba, the installer_files folder or MAMBA_ROOT_PREFIX. The
// allow-list is the legacy-env package, the installer step (installer_scripts
// and the launcher's step files, which P4 removes), the paths package that
// names the folder, and two files that keep a direct reference until P3 and
// P4 delete what they serve (PLATFORM-DESIGN.md, section 6). Adding a file
// here is a reviewed change.
var allowed = []string{
	"internal/platform/legacyenv/",
	"installer_scripts/",
	"internal/launcher/steps.go",
	"internal/launcher/handlers.go",
	"internal/paths/paths.go",
	"cmd/tts-webui/cmd_launch.go",
	"react_ui_backend/go_ffmpeg_metadata.go",
	"internal/launcher/procscan_other.go",
	"internal/platform/guard_test.go",
}

var mainEnvWords = regexp.MustCompile(`(?i)micromamba|installer_files|MAMBA_ROOT_PREFIX`)

func TestMainEnvIsNamedOnlyByLegacyEnvAndTheInstallerStep(t *testing.T) {
	root := filepath.Join("..", "..") // core/
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "ui", "scripts", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, a := range allowed {
			if rel == a || (strings.HasSuffix(a, "/") && strings.HasPrefix(rel, a)) {
				return nil
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if m := mainEnvWords.FindString(line); m != "" {
				offenders = append(offenders, rel+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("the main env is named outside legacy-env and the installer step:\n%s", strings.Join(offenders, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}
