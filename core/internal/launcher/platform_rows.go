package launcher

// The Extensions page's storage rows: the uv platform (uv, Python, git, ffmpeg)
// and the wheel cache. Both can be removed to get the first-run state back: the
// platform only while no venv needs it, the cache whenever no install runs.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
)

type storageRow struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
}

// storageState is the answer of GET /api/ext/storage.
type storageState struct {
	UV    storageRow `json:"uv"`
	Cache storageRow `json:"cache"`
	// Venvs are the own-venv entries on disk; while there is one, the uv
	// platform cannot be removed (a venv breaks when its base Python goes).
	Venvs      []string `json:"venvs"`
	AppRunning bool     `json:"app_running"`
}

// storageAnswer is a result of one of the POST actions.
type storageAnswer struct {
	OK  bool     `json:"ok"`
	Log []string `json:"log"`
}

// dirSize sums the file sizes under dir (0 when it is not there). Hardlinks are
// counted each time, as everywhere on this page.
func dirSize(ctx context.Context, dir string) (int64, error) {
	var n int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			return nil // a file that vanished or cannot be read does not stop the sum
		}
		if !d.IsDir() {
			if info, ierr := d.Info(); ierr == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n, err
}

// plainDir reports whether path is a real directory: not missing, and not a
// symlink or junction (removing through one would reach outside the install).
func plainDir(path string) (exists bool, err error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 || !fi.IsDir() {
		return true, fmt.Errorf("%s is not a plain folder (a link?); remove it by hand", path)
	}
	return true, nil
}

// venvNames lists the own-venv folders of the install.
func venvNames(root string) []string {
	ents, err := os.ReadDir(filepath.Join(root, ".venvs"))
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func (l *Launcher) handleStorageState() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := uv.New(uv.Options{Root: l.root})
		st := storageState{
			UV:         storageRow{Path: p.Dir()},
			Cache:      storageRow{Path: p.CacheDir()},
			Venvs:      venvNames(l.root.Dir()),
			AppRunning: l.sup.Running(),
		}
		for _, row := range []*storageRow{&st.UV, &st.Cache} {
			if _, err := os.Lstat(row.Path); err == nil {
				row.Exists = true
				row.SizeBytes, _ = dirSize(r.Context(), row.Path)
			}
		}
		writeExtJSON(w, http.StatusOK, st)
	})
}

func (l *Launcher) handleStorageAction(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := l.extensions()
		if err != nil {
			writeExtJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		p := uv.New(uv.Options{Root: l.root})
		var log []string
		emit := func(s string) { log = append(log, s) }
		refuse := func(code int, msg string) { writeExtJSON(w, code, map[string]string{"error": msg}) }

		err = h.cat.WhileNoInstall(func() error {
			switch kind {
			case "uv":
				if l.sup.Running() {
					return refusal{http.StatusConflict, "The app is running; stop it first (its Python lives in the uv platform)."}
				}
				if v := venvNames(l.root.Dir()); len(v) > 0 {
					return refusal{http.StatusConflict, fmt.Sprintf("These engines run on the uv platform: %v. Uninstall them first.", v)}
				}
				if _, derr := plainDir(p.Dir()); derr != nil {
					return refusal{http.StatusConflict, derr.Error()}
				}
				return p.Uninstall(r.Context(), emit)
			case "cache":
				if _, derr := plainDir(p.CacheDir()); derr != nil {
					return refusal{http.StatusConflict, derr.Error()}
				}
				n, _ := dirSize(r.Context(), p.CacheDir())
				if rerr := os.RemoveAll(p.CacheDir()); rerr != nil {
					return fmt.Errorf("removing %s: %w", p.CacheDir(), rerr)
				}
				emit(fmt.Sprintf("Cleared the wheel cache %s (%.1f MB as the files add up)", p.CacheDir(), float64(n)/(1<<20)))
				emit("The next install downloads its packages again.")
			}
			return nil
		})
		var rf refusal
		switch {
		case errors.Is(err, extcatalog.ErrInstallRunning):
			refuse(http.StatusConflict, "An install is running; wait for it to finish.")
		case errors.As(err, &rf):
			refuse(rf.code, rf.msg)
		case err != nil:
			refuse(http.StatusInternalServerError, err.Error())
		default:
			writeExtJSON(w, http.StatusOK, storageAnswer{OK: true, Log: append([]string{}, log...)})
		}
	})
}

// refusal is an action that must not run now, with the status to answer.
type refusal struct {
	code int
	msg  string
}

func (r refusal) Error() string { return r.msg }

func (l *Launcher) registerStorage(srv *server.Server) {
	srv.Handle("GET /api/ext/storage", l.handleStorageState())
	srv.Handle("POST /api/ext/storage/uv/uninstall", l.handleStorageAction("uv"))
	srv.Handle("POST /api/ext/storage/cache/clear", l.handleStorageAction("cache"))
}
