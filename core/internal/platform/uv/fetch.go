package uv

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
			return rerr
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

// extractMember writes the file named want (a base name, at any depth) of a zip
// or tar.gz archive to dest.
func extractMember(archive, want, dest string) error {
	if strings.HasSuffix(archive, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if !f.FileInfo().IsDir() && path.Base(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				return writeExecutable(dest, rc)
			}
		}
		return fmt.Errorf("%s is not in %s", want, filepath.Base(archive))
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s is not in %s", want, filepath.Base(archive))
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == want {
			return writeExecutable(dest, tr)
		}
	}
}

// writeExecutable writes r to dest through a temporary name, so a running copy
// is never half replaced.
func writeExecutable(dest string, r io.Reader) error {
	tmp := dest + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Remove(dest)
	return os.Rename(tmp, dest)
}
