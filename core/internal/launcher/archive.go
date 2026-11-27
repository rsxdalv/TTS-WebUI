package launcher

// Updating an install that came from the archive (D32): the installer zip CI
// publishes holds the repo, the launchers and frontend/out, and archive.json at
// its root says which commit it was built from. CI publishes the same file next
// to the zip as tts-webui-installer.json, so a check is one small request and
// needs no git. Updating downloads the zip and extracts it over the install; the
// running launcher binary is written as <name>.new and swapped in on the next
// start (Windows cannot overwrite a running exe).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// archiveMarker is the file at the archive's root that names its commit.
const archiveMarker = "archive.json"

// archiveInfo is archive.json.
type archiveInfo struct {
	Commit string `json:"commit"`
	Built  string `json:"built,omitempty"`
}

func readArchiveInfo(root paths.Root) (archiveInfo, bool) {
	b, err := os.ReadFile(filepath.Join(root.Dir(), archiveMarker))
	if err != nil {
		return archiveInfo{}, false
	}
	var a archiveInfo
	if json.Unmarshal(b, &a) != nil || a.Commit == "" {
		return archiveInfo{}, false
	}
	return a, true
}

// archiveURL is where the archive comes from: TTS_ARCHIVE_URL (a mirror, a test
// server) or the release asset.
func archiveURL() string {
	if u := os.Getenv("TTS_ARCHIVE_URL"); u != "" {
		return u
	}
	return canonicalDownloadURL
}

// archiveInfoURL is the small file published beside the zip.
func archiveInfoURL(zipURL string) string { return strings.TrimSuffix(zipURL, ".zip") + ".json" }

var updateHTTP = &http.Client{Timeout: 8 * time.Second}

func fetchLatest(ctx context.Context, zipURL string) (archiveInfo, error) {
	u := archiveInfoURL(zipURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return archiveInfo{}, err
	}
	resp, err := updateHTTP.Do(req)
	if err != nil {
		return archiveInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return archiveInfo{}, fmt.Errorf("%s answered HTTP %d", u, resp.StatusCode)
	}
	var a archiveInfo
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil || a.Commit == "" {
		return archiveInfo{}, fmt.Errorf("%s is not an archive description", u)
	}
	return a, nil
}

// updateState is what the update step shows. Detail is set whenever the answer
// is not a plain "up to date": an update is waiting, or it could not be checked.
type updateState struct {
	Done   bool
	Detail string
}

func short(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}

// archiveUpdateState compares the install's archive.json with the published one.
// An install that is not from the archive (a git clone) or that cannot reach the
// release says so; it is never reported as up to date.
func archiveUpdateState(ctx context.Context, root paths.Root) updateState {
	local, ok := readArchiveInfo(root)
	if !ok {
		if _, err := os.Stat(filepath.Join(root.Dir(), ".git")); err == nil {
			return updateState{Done: true, Detail: "This folder is a git clone, not an archive install: update it with git pull."}
		}
		return updateState{Done: true, Detail: "This folder has no " + archiveMarker + ", so its version is unknown and it cannot be updated from here: download the installer zip again."}
	}
	latest, err := fetchLatest(ctx, archiveURL())
	if err != nil {
		return updateState{Done: true, Detail: "Could not check for an update: " + err.Error()}
	}
	if latest.Commit == local.Commit {
		return updateState{Done: true}
	}
	return updateState{Done: false, Detail: fmt.Sprintf("An update is available: %s (this install is %s).", short(latest.Commit), short(local.Commit))}
}

// gitUpdateState is the check of an install with the old env: git has to be there.
func gitUpdateState(ctx context.Context, root paths.Root) updateState {
	if _, err := exec.LookPath("git"); err != nil {
		return updateState{Done: true, Detail: "git was not found, so updates cannot be checked."}
	}
	updated, _ := isGitUpdated(ctx, root)
	return updateState{Done: updated}
}

// updateStatus answers the update step. The answer is kept for a minute; a run of
// the step clears it.
func (l *Launcher) updateStatus() updateState {
	l.updMu.Lock()
	defer l.updMu.Unlock()
	if !l.updAt.IsZero() && time.Since(l.updAt) < time.Minute {
		return l.updState
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var st updateState
	if l.legacy() {
		if installDone(l.root) {
			st = gitUpdateState(ctx, l.root)
		}
	} else {
		st = archiveUpdateState(ctx, l.root)
	}
	l.updState, l.updAt = st, time.Now()
	return st
}

func (l *Launcher) forgetUpdate() {
	l.updMu.Lock()
	l.updAt = time.Time{}
	l.updMu.Unlock()
}

// updateRunning reports whether the archive update goroutine has not finished.
func (l *Launcher) updateRunning() bool {
	lg := l.getLog("update")
	if lg == nil {
		return false
	}
	select {
	case <-lg.Done():
		return false
	default:
		return true
	}
}

// runArchiveUpdate downloads the archive and extracts it over the install.
func (l *Launcher) runArchiveUpdate(ctx context.Context, lg *Log) error {
	local, ok := readArchiveInfo(l.root)
	if !ok {
		return fmt.Errorf("this folder has no %s: it is not an archive install, so there is nothing to compare; download the installer zip again", archiveMarker)
	}
	zipURL := archiveURL()
	latest, err := fetchLatest(ctx, zipURL)
	if err != nil {
		return fmt.Errorf("could not check for an update: %w", err)
	}
	if latest.Commit == local.Commit {
		lg.AddLine("Already up to date (" + short(local.Commit) + ").")
		return nil
	}
	lg.AddLine(fmt.Sprintf("Updating %s to %s", short(local.Commit), short(latest.Commit)))
	if err := downloadAndExtract(ctx, zipURL, l.root.Dir(), lg); err != nil {
		return err
	}
	if exe := runningExe(); exe != "" {
		if _, err := os.Stat(exe + pendingSuffix); err == nil {
			lg.AddLine("The new launcher program was saved as " + filepath.Base(exe) + pendingSuffix + "; it replaces the running one the next time the launcher starts.")
		}
	}
	lg.AddLine("Update complete. Restart the launcher to use it.")
	return nil
}

// startArchiveUpdate runs the update in the background and answers at once.
func (l *Launcher) startArchiveUpdate(w http.ResponseWriter) {
	lg := newLog()
	l.setLog("update", lg)
	l.forgetUpdate()
	go func() {
		defer lg.Close()
		if err := l.runArchiveUpdate(context.Background(), lg); err != nil {
			lg.AddLine("ERROR: " + err.Error())
			lg.SetExit(1)
		}
		l.forgetUpdate()
	}()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"started":true}`))
}
