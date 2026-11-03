package launcher

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// canonicalDownloadURL is the default release zip URL.
const canonicalDownloadURL = "https://github.com/rsxdalv/TTS-WebUI/releases/download/v0.0.0/tts-webui-installer.zip"

// downloadAndExtract downloads the zip at url and extracts it into destDir.
// Progress lines are written to lg.
func downloadAndExtract(ctx context.Context, url, destDir string, lg *Log) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	// --- Download ---
	lg.AddLine(fmt.Sprintf("Downloading %s", url))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	// Use the default transport (TLS-verified, no InsecureSkipVerify).
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	zipPath := filepath.Join(destDir, "release.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("creating zip file: %w", err)
	}

	total := resp.ContentLength
	written, err := copyWithProgress(ctx, f, resp.Body, total, lg)
	f.Close()
	if err != nil {
		return fmt.Errorf("writing zip: %w", err)
	}
	lg.AddLine(fmt.Sprintf("Download complete (%d bytes)", written))

	// --- Extract ---
	lg.AddLine("Extracting archive...")
	if err := extractZip(ctx, zipPath, destDir, lg); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}
	lg.AddLine("Extraction complete.")
	return nil
}

// copyWithProgress copies from src to dst, reporting progress every 5 MB.
func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, lg *Log) (int64, error) {
	const chunkSize = 5 << 20 // 5 MB
	buf := make([]byte, 32*1024)
	var written int64
	var nextReport int64 = chunkSize
	for {
		if ctx.Err() != nil {
			return written, ctx.Err()
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			nw, werr := dst.Write(buf[:n])
			written += int64(nw)
			if werr != nil {
				return written, werr
			}
			if written >= nextReport {
				if total > 0 {
					pct := written * 100 / total
					lg.AddLine(fmt.Sprintf("  %d%% (%d / %d bytes)", pct, written, total))
				} else {
					lg.AddLine(fmt.Sprintf("  %d bytes downloaded", written))
				}
				nextReport = written + chunkSize
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return written, rerr
		}
	}
	return written, nil
}

// extractZip extracts a zip archive into destDir, stripping one top-level directory.
func extractZip(ctx context.Context, zipPath, destDir string, lg *Log) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	// Detect the common prefix (strip the top-level zip directory).
	var prefix string
	if len(r.File) > 0 {
		parts := strings.SplitN(r.File[0].Name, "/", 2)
		if len(parts) == 2 {
			prefix = parts[0] + "/"
		}
	}

	for _, f := range r.File {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" {
			continue
		}
		// Guard against zip-slip.
		target := filepath.Join(destDir, filepath.FromSlash(name))
		if !strings.HasPrefix(target+string(os.PathSeparator), destDir+string(os.PathSeparator)) {
			return fmt.Errorf("zip-slip attempt: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(f, target); err != nil {
			return fmt.Errorf("extracting %q: %w", f.Name, err)
		}
	}
	lg.AddLine(fmt.Sprintf("Extracted %d entries", len(r.File)))
	return nil
}

// extractFile writes one zip file entry to disk.
func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
