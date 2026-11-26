package hfcache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CacheDir is where huggingface_hub would put the hub cache for this
// environment (env is NAME=value pairs, the last one wins), in its order:
// HF_HUB_CACHE, HUGGINGFACE_HUB_CACHE (the older name), then
// $HF_HOME/hub, where HF_HOME defaults to $XDG_CACHE_HOME/huggingface and then
// ~/.cache/huggingface. "~" and $VAR / ${VAR} are expanded; a relative path
// is relative to root, the folder the app runs in. An empty variable counts as
// not set (the .env the app writes comments an empty setting out).
func CacheDir(env []string, root string) string {
	vars := map[string]string{}
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			k = strings.ToUpper(k)
		}
		vars[k] = v
	}
	get := func(k string) string { return strings.TrimSpace(vars[k]) }
	home, _ := os.UserHomeDir()
	if h := get("HOME"); h != "" && runtime.GOOS != "windows" {
		home = h
	}
	expand := func(p string) string {
		if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
			p = filepath.Join(home, p[1:])
		}
		return os.Expand(p, func(name string) string {
			if v, ok := vars[name]; ok {
				return v
			}
			return "$" + name
		})
	}
	var dir string
	switch {
	case get("HF_HUB_CACHE") != "":
		dir = expand(get("HF_HUB_CACHE"))
	case get("HUGGINGFACE_HUB_CACHE") != "":
		dir = expand(get("HUGGINGFACE_HUB_CACHE"))
	default:
		hfHome := expand(get("HF_HOME"))
		if get("HF_HOME") == "" {
			base := filepath.Join(home, ".cache")
			if x := get("XDG_CACHE_HOME"); x != "" {
				base = x
			}
			hfHome = expand(filepath.Join(base, "huggingface"))
		}
		dir = filepath.Join(hfHome, "hub")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir)
}
