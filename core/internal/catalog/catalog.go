// Package catalog fetches, caches and serves the TTS WebUI engine catalog.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// ErrUnknownSchema is returned when the catalog's schema field is not recognised.
// The returned manifests are from the cached copy (if available).
var ErrUnknownSchema = errors.New("catalog: unrecognised schema version")

// currentSchema is the only schema version this binary understands.
const currentSchema = 1

// catalogFile is the top-level catalog JSON structure.
type catalogFile struct {
	Schema  int                 `json:"schema"`
	Engines []manifest.Manifest `json:"engines"`
}

// cacheFile is the path to the cached catalog inside cacheDir.
func cacheFile(cacheDir string) string {
	return filepath.Join(cacheDir, "catalog.json")
}

// Fetch downloads the catalog from url, writes it atomically to
// cacheDir/catalog.json, and returns the manifests.
//
// If the schema field is unrecognised, ErrUnknownSchema is returned along with
// the cached copy (if available). If the network call fails and a cache exists,
// the cached copy is returned with a warning logged; error is nil in that case.
func Fetch(ctx context.Context, url, cacheDir string) ([]manifest.Manifest, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return loadCachedFallback(cacheDir, fmt.Errorf("building request: %w", err))
	}
	resp, err := client.Do(req)
	if err != nil {
		return loadCachedFallback(cacheDir, fmt.Errorf("fetching catalog: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return loadCachedFallback(cacheDir, fmt.Errorf("catalog server returned %d", resp.StatusCode))
	}

	var cf catalogFile
	if err := json.NewDecoder(resp.Body).Decode(&cf); err != nil {
		return loadCachedFallback(cacheDir, fmt.Errorf("decoding catalog: %w", err))
	}

	if cf.Schema != 0 && cf.Schema != currentSchema {
		// Unknown schema: return ErrUnknownSchema + cached copy.
		cached, _ := Load(cacheDir)
		return cached, fmt.Errorf("%w (got %d)", ErrUnknownSchema, cf.Schema)
	}

	// Write atomically.
	if err := writeCatalog(cacheDir, cf); err != nil {
		slog.Warn("catalog: could not write cache", "err", err)
	}

	return cf.Engines, nil
}

// Load reads the catalog from the cache at cacheDir/catalog.json.
func Load(cacheDir string) ([]manifest.Manifest, error) {
	data, err := os.ReadFile(cacheFile(cacheDir))
	if err != nil {
		return nil, fmt.Errorf("reading catalog cache: %w", err)
	}
	var cf catalogFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing catalog cache: %w", err)
	}
	return cf.Engines, nil
}

// writeCatalog atomically writes cf to cacheDir/catalog.json.
func writeCatalog(cacheDir string, cf catalogFile) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshalling catalog: %w", err)
	}
	tmp := cacheFile(cacheDir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	return os.Rename(tmp, cacheFile(cacheDir))
}

// loadCachedFallback logs fetchErr as a warning and returns the cached copy.
// If no cache exists, both the fetch error and a load error are returned.
func loadCachedFallback(cacheDir string, fetchErr error) ([]manifest.Manifest, error) {
	cached, loadErr := Load(cacheDir)
	if loadErr == nil {
		slog.Warn("catalog: fetch failed, serving cached copy", "err", fetchErr)
		return cached, nil
	}
	return nil, fmt.Errorf("%w; cache also unavailable: %v", fetchErr, loadErr)
}
