package react_ui_backend

// Huggingface Cache Manager (extension_huggingface_cache_manager) in Go, so the
// page works with no Python and no Gradio. The work is in internal/hfcache; this
// file is the HTTP side:
//
//	GET  /api/go-data/hf-cache/scan    -> {headers, rows, cache_dir, exists, size_on_disk, size_on_disk_str, warnings}
//	POST /api/go-data/hf-cache/delete  -> {deleted, not_found, expected_freed_size, expected_freed_size_str, removed, errors}
//
// scan answers the same {headers, rows} the Gradio endpoint scan_huggingface_cache_api
// answered (the keys of scan_cache.get_rows_json), plus what the page can show.
//
// delete takes {"revisions": [{"commit_hash", "repo_id"?, "repo_type"?}]} or,
// like the Gradio endpoint, {"commit_hash": "..."}. It never takes a path: the
// request picks revisions out of a fresh scan, the cache folder comes from the
// environment the app runs with (HF_HUB_CACHE, HUGGINGFACE_HUB_CACHE, HF_HOME,
// XDG_CACHE_HOME from the process and from .env, which is what the Model
// Location tab saves), and everything removed is checked to be inside it
// (hfcache's package comment lists the rules). A hash that exists in two repos
// is refused (409) unless repo_id names one.

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/hfcache"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "go-hf-cache",
		Patterns: []string{"GET /api/go-data/hf-cache/scan", "POST /api/go-data/hf-cache/delete"},
		Handler:  func(d Deps) http.Handler { return newHFCache(d.Root, appEnvironment) },
	})
}

// appEnvironment is the environment server.py has after .env is loaded: the
// process environment, then .env (never overriding it), then .env.user.
func appEnvironment(root string) []string {
	env, err := appenv.LoadEnv(root, os.Environ())
	if err != nil {
		return os.Environ()
	}
	return env
}

const maxHFCacheSelectors = 500

func newHFCache(root string, environment func(root string) []string) http.Handler {
	var mu sync.Mutex // a delete does not run beside another delete or a scan
	cacheDir := func() string { return hfcache.CacheDir(environment(root), root) }

	scan := func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		info, err := hfcache.Scan(cacheDir())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not scan the Hugging Face cache: "+err.Error())
			return
		}
		writeJSONStatus(w, http.StatusOK, map[string]any{
			"headers":          hfcache.Headers,
			"rows":             info.Rows(time.Now()),
			"cache_dir":        info.CacheDir,
			"exists":           info.Exists,
			"size_on_disk":     info.SizeOnDisk,
			"size_on_disk_str": hfcache.FormatSize(info.SizeOnDisk),
			"warnings":         info.Warnings,
		})
	}

	del := func(w http.ResponseWriter, r *http.Request) {
		if !requireJSON(w, r) {
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		var in struct {
			Revisions  []hfcache.Selector `json:"revisions"`
			CommitHash string             `json:"commit_hash"`
			RepoID     string             `json:"repo_id"`
			RepoType   string             `json:"repo_type"`
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			writeAPIError(w, http.StatusBadRequest, `body must be {"revisions": [{"commit_hash": "..."}]} or {"commit_hash": "..."}`)
			return
		}
		sel := in.Revisions
		if in.CommitHash != "" {
			sel = append(sel, hfcache.Selector{CommitHash: in.CommitHash, RepoID: in.RepoID, RepoType: in.RepoType})
		}
		if len(sel) == 0 || len(sel) > maxHFCacheSelectors {
			writeAPIError(w, http.StatusBadRequest, "name between 1 and 500 revisions")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		info, err := hfcache.Scan(cacheDir())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not scan the Hugging Face cache: "+err.Error())
			return
		}
		plan, err := hfcache.PlanDelete(info, sel)
		switch {
		case errors.Is(err, hfcache.ErrAmbiguous):
			writeAPIError(w, http.StatusConflict, err.Error())
			return
		case err != nil:
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		res := plan.Execute()
		writeJSONStatus(w, http.StatusOK, map[string]any{
			"deleted":                 orEmpty(plan.Deleted),
			"not_found":               orEmpty(plan.NotFound),
			"expected_freed_size":     plan.ExpectedFreed,
			"expected_freed_size_str": hfcache.FormatSize(plan.ExpectedFreed),
			"removed":                 len(res.Removed),
			"errors":                  orEmpty(res.Errors),
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/go-data/hf-cache/scan":
			scan(w, r)
		case "POST /api/go-data/hf-cache/delete":
			del(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
