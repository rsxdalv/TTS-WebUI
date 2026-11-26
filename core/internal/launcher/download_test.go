package launcher

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, path string, names []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if n[len(n)-1] != '/' {
			_, _ = w.Write([]byte("x"))
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Shaped like the real release asset: no wrapper folder, .git/ first.
func TestExtractZip_NoWrapper_RealAssetLayout(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "a.zip")
	writeZip(t, zp, []string{".git/", ".git/HEAD", ".git/config", "installer_scripts/", "installer_scripts/init_app.js", "README.md"})
	dest := filepath.Join(dir, "out")
	if err := extractZip(context.Background(), zp, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".git/HEAD", ".git/config", "installer_scripts/init_app.js", "README.md"} {
		if !exists(filepath.Join(dest, filepath.FromSlash(p))) {
			t.Errorf("missing %s", p)
		}
	}
	if exists(filepath.Join(dest, "HEAD")) {
		t.Error(".git contents leaked into root")
	}
}

func TestExtractZip_NoWrapper_ManyTopLevelFolders(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "a.zip")
	writeZip(t, zp, []string{".git/HEAD", "installer_scripts/x.js"})
	dest := filepath.Join(dir, "out")
	if err := extractZip(context.Background(), zp, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, ".git", "HEAD")) || !exists(filepath.Join(dest, "installer_scripts", "x.js")) {
		t.Error("layout not preserved")
	}
}

func TestExtractZip_SingleWrapperStripped(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "a.zip")
	writeZip(t, zp, []string{"wrap/", "wrap/installer_scripts/", "wrap/installer_scripts/x.js", "wrap/README.md"})
	dest := filepath.Join(dir, "out")
	if err := extractZip(context.Background(), zp, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "installer_scripts", "x.js")) || exists(filepath.Join(dest, "wrap")) {
		t.Error("wrapper not stripped")
	}
}

func TestDownloadAndExtract_IntoRootRemovesZip(t *testing.T) {
	src := t.TempDir()
	zp := filepath.Join(src, "r.zip")
	writeZip(t, zp, []string{".git/HEAD", "installer_scripts/x.js"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, zp)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "root")
	if err := downloadAndExtract(context.Background(), srv.URL, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "installer_scripts", "x.js")) {
		t.Error("not extracted into dest")
	}
	if exists(filepath.Join(dest, "release.zip")) {
		t.Error("release.zip not removed")
	}
}

// archive.json names the commit the files belong to, so it is written after every
// other file: an extraction that fails half way must not leave the new commit over old files.
func TestExtractZip_ArchiveMarkerIsWrittenLast(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "a.zip")
	writeZip(t, zp, []string{"archive.json", "README.md", "../escape.txt", "later.txt"})
	dest := filepath.Join(dir, "out")
	if err := extractZip(context.Background(), zp, dest, newLog()); err == nil {
		t.Fatal("the zip-slip entry was accepted")
	}
	if exists(filepath.Join(dest, "archive.json")) {
		t.Error("archive.json was written before the extraction failed")
	}
	if !exists(filepath.Join(dest, "README.md")) {
		t.Error("the entries before the bad one should be extracted")
	}
	// A complete extraction writes it.
	good := filepath.Join(dir, "b.zip")
	writeZip(t, good, []string{"archive.json", "README.md"})
	if err := extractZip(context.Background(), good, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "archive.json")) {
		t.Error("archive.json missing after a complete extraction")
	}
}

// A download that ends early (the connection drops) leaves no half archive in the install folder.
func TestDownloadAndExtract_CutDownloadLeavesNoZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write([]byte("PK-not-the-whole-file"))
		// Returning with less than Content-Length makes the server drop the connection.
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "root")
	if err := downloadAndExtract(context.Background(), srv.URL, dest, newLog()); err == nil {
		t.Fatal("a cut download was accepted")
	}
	if exists(filepath.Join(dest, "release.zip")) {
		t.Error("release.zip left behind")
	}
}
