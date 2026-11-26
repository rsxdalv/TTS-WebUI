package onnx

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// extractTarBz2 unpacks a .tar.bz2 into dest, dropping the one top-level folder
// the archives have. Only regular files and folders are written; a path that
// leaves dest is refused; links and special files are skipped with an error.
func extractTarBz2(ctx context.Context, archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	var src io.Reader
	// The release archives are .tar.bz2; the format is read from the first bytes
	// (tests serve .tar.gz, because Go cannot write bzip2).
	magic, _ := br.Peek(3)
	switch {
	case len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		gz, err := gzip.NewReader(br)
		if err != nil {
			return err
		}
		defer gz.Close()
		src = gz
	case string(magic) == "BZh":
		src = bzip2.NewReader(br)
	default:
		return errors.New("the archive is neither bzip2 nor gzip")
	}
	tr := tar.NewReader(src)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(filepath.ToSlash(h.Name), "./")
		i := strings.Index(name, "/")
		if i < 0 { // the top-level folder entry itself
			continue
		}
		rel := name[i+1:]
		if rel == "" {
			continue
		}
		target := filepath.Join(absDest, filepath.FromSlash(rel))
		if r, err := filepath.Rel(absDest, target); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path %q leaves the install folder", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive entry %q has an unsupported type", h.Name)
		}
	}
}
