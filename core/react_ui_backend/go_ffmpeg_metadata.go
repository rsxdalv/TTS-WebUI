package react_ui_backend

// FFMPEG Metadata (the retired Gradio extension_ffmpeg_metadata): the JSON that a generation
// stored in the "comment" tag of an ogg or flac file, read with ffprobe.
//
// Same logic as load_ffmpeg_metadata had in the extension:
//   - ogg (extension .ogg or format_name "ogg"): the comment tag of stream 0;
//   - flac (extension .flac or format_name "flac"): the comment tag of the format;
//   - anything else: the whole ffprobe document.
// Differences: the tag name is matched without regard to case (ffprobe prints
// vorbis comments as they are written); a file without a usable comment tag
// answers with the whole ffprobe document, where the extension failed with an
// error.
//
// The file is named, never given as a path: POST {"name": "x.flac"} reads
// frontend/public/file-input-cache/x.flac, the directory /api/upload fills.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "go-ffmpeg-metadata",
		Patterns: []string{"POST /api/go-data/ffmpeg-metadata"},
		Handler:  newFFmpegMetadata,
	})
}

// ffprobeRun runs ffprobe on a file and returns its JSON. Tests replace it.
var ffprobeRun = func(ctx context.Context, root, file string) ([]byte, error) {
	exe := findFFprobe(root)
	if exe == "" {
		return nil, errNoFFprobe
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "-i", file)
	// The provider's Linux build is shared: ffprobe loads libav* from ../lib.
	if lib := filepath.Join(filepath.Dir(exe), "..", "lib"); runtime.GOOS == "linux" && strings.Contains(filepath.ToSlash(exe), "/tools/ffmpeg/") {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+lib)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var errNoFFprobe = errors.New("ffprobe was not found (install ffmpeg or add ffprobe to PATH; installing any engine from the launcher installs uv first, which fetches ffmpeg)")

// findFFprobe looks in the project environment first, then on PATH.
func findFFprobe(root string) string {
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	for _, dir := range []string{
		filepath.Join(root, "installer_files", "env", "Library", "bin"),
		filepath.Join(root, "installer_files", "env", "bin"),
		filepath.Join(root, "installer_files", "env", "Scripts"),
		// The uv platform's tools provider (D26): a new install has no env.
		filepath.Join(root, "platforms", "uv", "tools", "ffmpeg", "bin"),
	} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	if p, err := exec.LookPath("ffprobe"); err == nil {
		return p
	}
	return ""
}

func newFFmpegMetadata(deps Deps) http.Handler {
	cacheDir := filepath.Join(deps.Root, "frontend", "public", "file-input-cache")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireJSON(w, r) {
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		var in struct {
			Name string `json:"name"`
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || !validCacheName(in.Name) {
			writeAPIError(w, http.StatusBadRequest, "name must be the name of an uploaded audio file")
			return
		}
		path := filepath.Join(cacheDir, in.Name)
		if st, err := os.Lstat(path); err != nil || !st.Mode().IsRegular() {
			writeAPIError(w, http.StatusNotFound, "file not found")
			return
		}
		raw, err := ffprobeRun(r.Context(), deps.Root, path)
		if err != nil {
			if errors.Is(err, errNoFFprobe) {
				writeAPIError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			writeAPIError(w, http.StatusUnprocessableEntity, "ffprobe could not read the file")
			return
		}
		out, err := commentMetadata(raw, in.Name)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "ffprobe gave no JSON")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(out)
	})
}

// validCacheName: one path element with an extension /api/upload accepts.
func validCacheName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\x00:") {
		return false
	}
	return uploadExtensions[strings.ToLower(filepath.Ext(name))] && !badWindowsName(name)
}

// commentMetadata applies the tag rules to ffprobe's document.
func commentMetadata(raw []byte, name string) ([]byte, error) {
	var probe struct {
		Format struct {
			FormatName string            `json:"format_name"`
			Tags       map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Tags map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	var tags map[string]string
	switch {
	case strings.HasSuffix(lower, ".ogg") || probe.Format.FormatName == "ogg":
		if len(probe.Streams) > 0 {
			tags = probe.Streams[0].Tags
		}
	case strings.HasSuffix(lower, ".flac") || probe.Format.FormatName == "flac":
		tags = probe.Format.Tags
	}
	for k, v := range tags {
		if strings.EqualFold(k, "comment") {
			var js json.RawMessage
			if json.Unmarshal([]byte(v), &js) == nil {
				return js, nil
			}
		}
	}
	return raw, nil
}
