package react_ui_backend

// Port of react-ui/src/app/api/upload/route.ts.
//
// Kept from the original:
//   - the whole body is read before the answer, so a bad type, a size over the
//     limit and a damaged body are decided in the original's order: no "file"
//     entry 400, a "file" entry that is not a file 500, bad extension 400,
//     size over 256 MiB 413, then the write;
//   - the name is the last path component with leading dots removed, and its
//     extension must be on the allow-list (compared in lower case);
//   - the file is written to react-ui/public/file-input-cache, replacing a file
//     of the same name; the directory is not created (a missing one is a 500);
//   - the answer is {"success":true,"name":...}.
//
// Different, on purpose: the body is streamed to a temporary file in the cache
// directory and renamed at the end, so memory stays flat for large uploads and
// a failed upload never leaves a half-written file under its real name. The
// original holds the whole file in memory (twice). On Windows a name with ":"
// (an NTFS alternate data stream) or a reserved device name such as "con.wav"
// is answered with the "Unsupported file type" 400.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// uploadMaxBytes is MAX_BYTES in the original.
const uploadMaxBytes = 256 * 1024 * 1024

var uploadExtensions = map[string]bool{
	".wav": true, ".mp3": true, ".ogg": true, ".flac": true, ".m4a": true, ".opus": true,
	".webm": true, ".npz": true, ".npy": true, ".png": true, ".jpg": true, ".jpeg": true,
}

func init() {
	RegisterClaim(Claim{
		Switch:   "upload",
		Patterns: []string{"POST /api/upload"},
		Handler:  newUpload,
	})
}

// uploadTempPrefix names the temporary files of uploads in progress.
const uploadTempPrefix = ".upload-"

func newUpload(deps Deps) http.Handler {
	cacheDir := filepath.Join(deps.Root, "react-ui", "public", "file-input-cache")
	removeStaleUploads(cacheDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up, err := readUpload(r, cacheDir)
		if err != nil {
			up.discard()
			uploadFailed(w)
			return
		}
		defer up.discard()

		switch {
		case !up.present:
			uploadJSON(w, http.StatusBadRequest, map[string]any{"success": false})
			return
		case !up.isFile:
			uploadFailed(w) // file.name is undefined, path.basename throws
			return
		}
		safeName := safeUploadName(up.filename)
		if invalidFileNameChars(safeName) && uploadExtensions[strings.ToLower(filepath.Ext(safeName))] {
			uploadFailed(w) // writeFile fails on Windows
			return
		}
		if safeName == "" || !uploadExtensions[strings.ToLower(filepath.Ext(safeName))] || badWindowsName(safeName) {
			uploadJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Unsupported file type"})
			return
		}
		if up.size > uploadMaxBytes {
			uploadJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"success": false, "error": "File too large"})
			return
		}
		if err := up.commit(filepath.Join(cacheDir, safeName)); err != nil {
			uploadFailed(w)
			return
		}
		uploadJSON(w, http.StatusOK, map[string]any{"success": true, "name": safeName})
	})
}

// removeStaleUploads deletes temporary files an earlier run left behind, for
// instance when the launcher was killed during an upload. It runs when the
// claim starts, before any upload of this run exists.
func removeStaleUploads(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), uploadTempPrefix) {
			_ = os.Remove(filepath.Join(cacheDir, e.Name()))
		}
	}
}

// uploadNameKept reports whether a file with this name gets past the type
// check, so its content is worth keeping while the body is read.
func uploadNameKept(filename string) bool {
	safe := safeUploadName(filename)
	return safe != "" && uploadExtensions[strings.ToLower(filepath.Ext(safe))] && !badWindowsName(safe)
}

// upload is what the body held for the first "file" entry.
type upload struct {
	present  bool   // an entry named "file" exists and is not an empty string
	isFile   bool   // it has a filename, so it is a File and not a string
	filename string // the raw filename
	size     int64
	tmp      string // temporary file holding the content, if isFile
}

func (u *upload) discard() {
	if u != nil && u.tmp != "" {
		_ = os.Remove(u.tmp)
		u.tmp = ""
	}
}

func (u *upload) commit(target string) error {
	if err := os.Rename(u.tmp, target); err != nil {
		return err
	}
	u.tmp = ""
	return nil
}

// readUpload reads the whole body like request.formData() and keeps the first
// "file" entry. It always returns a non-nil *upload.
func readUpload(r *http.Request, cacheDir string) (*upload, error) {
	up := &upload{}
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return up, err
	}
	switch mediaType {
	case "application/x-www-form-urlencoded":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return up, err
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return up, err
		}
		if v := values["file"]; len(v) > 0 && v[0] != "" {
			up.present = true // a string, not a file
		}
		return up, nil
	case "multipart/form-data":
	default:
		return up, http.ErrNotMultipart
	}
	if params["boundary"] == "" {
		return up, http.ErrMissingBoundary
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	seenFile := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return up, nil
		}
		if err != nil {
			return up, err
		}
		_, disp, derr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		name, hasName := disp["name"]
		filename, isFile := disp["filename"]
		if derr != nil || !hasName || name != "file" || seenFile {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return up, err
			}
			continue
		}
		seenFile = true
		if !isFile {
			b, err := io.ReadAll(part)
			if err != nil {
				return up, err
			}
			up.present = len(b) > 0
			continue
		}
		filename = undiciFilename(filename)
		if !uploadNameKept(filename) {
			// Rejected as "Unsupported file type" whatever the size: read it,
			// write nothing.
			n, err := io.Copy(io.Discard, part)
			if err != nil {
				return up, err
			}
			up.present, up.isFile, up.filename, up.size = true, true, filename, n
			continue
		}
		f, err := os.CreateTemp(cacheDir, uploadTempPrefix+"*")
		if err != nil {
			_, _ = io.Copy(io.Discard, part)
			return up, err
		}
		up.tmp = f.Name()
		// Write at most the limit. Past it the answer is 413, so the temporary
		// file is removed and the rest is only counted, never stored.
		n, err := io.Copy(f, io.LimitReader(part, uploadMaxBytes+1))
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			return up, err
		}
		if n > uploadMaxBytes {
			up.discard()
			rest, err := io.Copy(io.Discard, part)
			if err != nil {
				return up, err
			}
			n += rest
		}
		up.present, up.isFile, up.filename, up.size = true, true, filename, n
	}
}

// undiciFilename undoes what browsers' multipart encoding does to a filename;
// Node's parser (undici) turns %22, %0A and %0D back into the characters.
func undiciFilename(name string) string {
	return strings.NewReplacer("%22", "\"", "%0A", string(rune(10)), "%0a", string(rune(10)), "%0D", string(rune(13)), "%0d", string(rune(13))).Replace(name)
}

// safeUploadName is path.basename(name).replace(/^\.+/, "") for the platform.
func safeUploadName(name string) string {
	if runtime.GOOS == "windows" {
		name = strings.ReplaceAll(name, "\\", "/")
		if len(name) >= 2 && name[1] == ':' {
			name = name[2:] // a drive letter, as path.win32.basename drops it
		}
	}
	name = strings.TrimRight(name, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimLeft(name, ".")
}

// badWindowsName reports names the original would turn into an alternate data
// stream or a device on Windows.
func badWindowsName(name string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if strings.Contains(name, ":") {
		return true
	}
	stem := strings.ToUpper(strings.TrimRight(strings.SplitN(name, ".", 2)[0], " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return true
	}
	return false
}

// invalidFileNameChars reports characters Windows cannot have in a file name.
func invalidFileNameChars(name string) bool {
	if runtime.GOOS != "windows" {
		return strings.ContainsRune(name, 0)
	}
	return strings.ContainsAny(name, "<>\"|?*") || strings.IndexFunc(name, func(r rune) bool { return r < 32 }) >= 0
}

// uploadFailed is what Next answers when the route handler throws: 500 with no body.
func uploadFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	flushNow(w) // no Content-Length, like Next's chunked answer
}

func flushNow(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// uploadJSON writes what NextResponse.json writes: compact JSON, keys in the
// order the original builds them, no HTML escaping.
func uploadJSON(w http.ResponseWriter, status int, v map[string]any) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for _, k := range []string{"success", "error", "name"} {
		val, ok := v[k]
		if !ok {
			continue
		}
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		var vb bytes.Buffer
		enc := json.NewEncoder(&vb)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(val)
		buf.Write(bytes.TrimRight(vb.Bytes(), "\n"))
	}
	buf.WriteByte('}')
	h := w.Header()
	h.Set("Content-Type", "application/json")
	w.WriteHeader(status)
	flushNow(w) // Next answers chunked, without Content-Length
	_, _ = w.Write(buf.Bytes())
}
