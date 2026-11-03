// Package api implements the core HTTP API endpoints that replace Next.js API routes.
package api

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

// uploadMaxBytes is the maximum allowed upload size (50 MB).
const uploadMaxBytes = 50 << 20

// Register adds the core API routes to srv.
//
//   - GET  /api/generations/{collection}/{name...}  — serve generated files
//   - POST /api/upload                              — accept multipart file uploads
//   - GET  /api/version                             — return build version
//
// All routes are automatically token-guarded by server.Handle.
func Register(srv *server.Server, root paths.Root) {
	srv.Handle("GET /api/generations/{collection}/{name...}", handleGenerations(root))
	srv.Handle("POST /api/upload", handleUpload(root))
	srv.Handle("GET /api/version", handleVersion())
}

// collectionRoot maps a URL collection name to the absolute directory on disk.
func collectionRoot(root paths.Root, collection string) (string, bool) {
	switch collection {
	case "outputs":
		return root.Outputs(), true
	case "favorites":
		return root.Favorites(), true
	case "voices":
		return root.Voices(), true
	default:
		return "", false
	}
}

// handleGenerations serves files from output directories.
// Replaces pages/api/webui-generations/ in Next.js.
func handleGenerations(root paths.Root) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := r.PathValue("collection")
		name := r.PathValue("name")

		collDir, ok := collectionRoot(root, collection)
		if !ok {
			http.Error(w, "unknown collection", http.StatusNotFound)
			return
		}

		// path.Clean rejects ".." traversal in the URL component.
		cleanName := path.Clean("/" + name)
		if cleanName == "/" || strings.Contains(cleanName, "..") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Resolve to an absolute path and verify it is under the collection root.
		abs := filepath.Join(collDir, filepath.FromSlash(cleanName))
		absResolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "forbidden", http.StatusForbidden)
			}
			return
		}
		collDirClean := filepath.Clean(collDir)
		if !strings.HasPrefix(absResolved, collDirClean+string(filepath.Separator)) &&
			absResolved != collDirClean {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Set Content-Type from file extension.
		ext := filepath.Ext(abs)
		if ct := mime.TypeByExtension(ext); ct != "" {
			w.Header().Set("Content-Type", ct)
		}

		http.ServeFile(w, r, absResolved)
	})
}

// uploadResponse is the JSON body returned by POST /api/upload.
type uploadResponse struct {
	Path string `json:"path"`
}

// handleUpload accepts a multipart/form-data upload with a "file" field.
// Replaces app/api/upload/route.ts in Next.js.
func handleUpload(root paths.Root) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, uploadMaxBytes)

		if err := r.ParseMultipartForm(uploadMaxBytes); err != nil {
			http.Error(w, "request too large or malformed", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Sanitize: take the base name only, reject empty or dot-only names.
		baseName := filepath.Base(filepath.FromSlash(header.Filename))
		if baseName == "" || baseName == "." || baseName == ".." {
			http.Error(w, "invalid filename", http.StatusBadRequest)
			return
		}

		destDir := filepath.Join(root.Outputs(), "uploads")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			http.Error(w, "could not create upload directory", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(destDir, baseName)
		// Verify the destination is still inside destDir after cleaning.
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)+string(filepath.Separator)) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		out, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "could not create file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			http.Error(w, "write failed", http.StatusInternalServerError)
			return
		}

		// Return relative path from the install root.
		rel, _ := filepath.Rel(root.Dir(), destPath)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(uploadResponse{Path: filepath.ToSlash(rel)})
	})
}

// versionResponse is the JSON body returned by GET /api/version.
type versionResponse struct {
	Version string `json:"version"`
}

// handleVersion returns build version information.
func handleVersion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(versionResponse{Version: version.String()})
	})
}
