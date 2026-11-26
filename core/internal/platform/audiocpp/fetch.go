package audiocpp

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// download fetches url into dest. The bytes go to dest+".part" and are renamed
// only after the checksum matches, so a cancelled or corrupt download never
// leaves something that looks complete. progress is called about every 5%.
func download(ctx context.Context, client *http.Client, url, wantSHA, dest string, progress func(done, total int64)) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	part := dest + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	h := sha256.New()
	var done, last int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(part)
				return werr
			}
			done += int64(n)
			if progress != nil && resp.ContentLength > 0 && done-last >= resp.ContentLength/20 {
				last = done
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(part)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("download %s: the transfer broke after %s: %w", url, received(done, resp.ContentLength), rerr)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(part)
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, wantSHA) {
		os.Remove(part)
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantSHA)
	}
	return os.Rename(part, dest)
}

// received says how far a broken download got, for the message ("120 of 300 bytes").
func received(done, total int64) string {
	if total > 0 {
		return fmt.Sprintf("%d of %d bytes", done, total)
	}
	return fmt.Sprintf("%d bytes", done)
}

// fileMatches reports whether the file at path has the sha256 want.
func fileMatches(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want)
}

// extractZip unpacks a zip into dest. A member that would land outside dest, or
// a link, is an error.
func extractZip(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		for _, part := range strings.Split(name, "/") {
			if part == ".." {
				return fmt.Errorf("%q has a .. in it", f.Name)
			}
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return fmt.Errorf("%q would be written outside the install folder", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q is a link; the audio.cpp archive is expected to have none", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm()|0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
