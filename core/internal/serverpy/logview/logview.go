// Package logview is the Go side of the Log Viewer extension
// (extensions/builtin/extension_log_viewer): list, read, search, analyse and
// delete the *.log files in installer_scripts/logs. Python's Gradio handlers
// call it through tts_webui/core_bridge (bridge.logs) when TTS_CORE_URL is set;
// Go does all the file work, so the path checks live in one place.
//
// Names from clients are single file names inside the logs directory: no
// separators, no "..", no drive letters, must end in ".log", must be a regular
// file (symlinks are never followed, read or deleted).
package logview

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// maxRead is how much of one file is read (its tail).
	maxRead = 32 << 20
	// MaxLinesLimit is the largest max_lines accepted.
	MaxLinesLimit = 100000
	timeLayout    = "2006-01-02 15:04:05"
)

// Error kinds, mapped to HTTP statuses by the handler.
var errInvalid = errors.New("invalid") // 400

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errInvalid, fmt.Sprintf(format, a...))
}

// Options configures a Store.
type Options struct {
	Root  string // project root: logs live in Root/installer_scripts/logs
	Token string // required; requests without it get 403
	Logf  func(format string, args ...any)
}

// Store serves the log API for one project root.
type Store struct {
	opts Options
	dir  string
	mu   sync.Mutex // serialises deletes
}

func New(o Options) *Store {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Store{opts: o, dir: filepath.Join(o.Root, "installer_scripts", "logs")}
}

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// validName reports whether s is a safe single file name on every platform we
// run on (Windows is the strictest) that ends in .log.
func validName(s string) bool {
	if s == "" || s == "." || s == ".." || len(s) > 255 {
		return false
	}
	if strings.HasPrefix(s, ".") || strings.HasSuffix(s, " ") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(s), ".log") {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("<>:\"/\\|?*", r) {
			return false
		}
	}
	stem := s
	if i := strings.IndexByte(s, '.'); i >= 0 {
		stem = s[:i]
	}
	return !reservedNames[strings.ToLower(strings.TrimRight(stem, " "))]
}

func (s *Store) path(name string) (string, error) {
	if !validName(name) {
		return "", invalidf("%q is not a log file name", name)
	}
	return filepath.Join(s.dir, name), nil
}

type entry struct {
	name  string
	size  int64
	mtime time.Time
}

// entries lists the regular *.log files, newest first (ties by name). A
// missing logs directory is an empty list; it is not created.
func (s *Store) entries() ([]entry, error) {
	ents, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []entry
	for _, e := range ents {
		if !e.Type().IsRegular() || !validName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, entry{e.Name(), info.Size(), info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].mtime.Equal(out[j].mtime) {
			return out[i].mtime.After(out[j].mtime)
		}
		return out[i].name < out[j].name
	})
	return out, nil
}

// Files returns the log file names, newest first.
func (s *Store) Files() ([]string, error) {
	es, err := s.entries()
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.name
	}
	return names, err
}

// PipLog is a pip install or uninstall log: Name is the file name without the
// "pip-install-"/"pip-uninstall-" prefix and the ".log" suffix.
type PipLog struct {
	Name string `json:"name"`
	File string `json:"file"`
}

func kindPrefix(kind string) (string, error) {
	switch kind {
	case "install":
		return "pip-install-", nil
	case "uninstall":
		return "pip-uninstall-", nil
	}
	return "", invalidf("kind must be install or uninstall")
}

// PipLogs lists the logs of one kind, newest first.
func (s *Store) PipLogs(kind string) ([]PipLog, error) {
	prefix, err := kindPrefix(kind)
	if err != nil {
		return nil, err
	}
	es, err := s.entries()
	out := []PipLog{}
	for _, e := range es {
		if strings.HasPrefix(e.name, prefix) {
			out = append(out, PipLog{Name: strings.TrimSuffix(strings.TrimPrefix(e.name, prefix), ".log"), File: e.name})
		}
	}
	return out, err
}

// Stats is the Markdown of the file count and total size.
func (s *Store) Stats() (string, error) {
	es, err := s.entries()
	if err != nil {
		return "", err
	}
	var total int64
	for _, e := range es {
		total += e.size
	}
	return fmt.Sprintf("**Total Log Files:** %d\n**Total Size:** %.2f MB", len(es), float64(total)/(1024*1024)), nil
}

// open opens a regular file of the logs directory, refusing symlinks (also
// when swapped in between the check and the open).
func (s *Store) open(name string) (*os.File, os.FileInfo, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, nil, err
	}
	li, err := os.Lstat(p)
	if err != nil {
		return nil, nil, err
	}
	if !li.Mode().IsRegular() {
		return nil, nil, invalidf("%q is not a regular file", name)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil || !os.SameFile(li, fi) || !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, invalidf("%q changed while opening", name)
	}
	return f, fi, nil
}

// readText returns the file as text: at most the last maxRead bytes, invalid
// UTF-8 dropped (errors="ignore"), line ends normalised to \n. found is false
// when the file does not exist.
func (s *Store) readText(name string) (text string, truncated, found bool, err error) {
	f, fi, err := s.open(name)
	if os.IsNotExist(err) {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	defer f.Close()
	size := fi.Size()
	off := int64(0)
	if size > maxRead {
		off, truncated = size-maxRead, true
	}
	b := make([]byte, size-off)
	n, err := f.ReadAt(b, off)
	if err != nil && err != io.EOF {
		return "", false, true, err
	}
	t := strings.ToValidUTF8(string(b[:n]), "")
	t = strings.ReplaceAll(t, "\r\n", "\n")
	t = strings.ReplaceAll(t, "\r", "\n")
	return t, truncated, true, nil
}

// Info is the Markdown header of a selected file.
func (s *Store) Info(name string) (string, error) {
	p, err := s.path(name)
	if err != nil {
		return "", err
	}
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return "File not found", nil
	}
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", invalidf("%q is not a regular file", name)
	}
	return fmt.Sprintf("**File:** %s\n**Size:** %.2f KB\n**Modified:** %s",
		name, float64(fi.Size())/1024, fi.ModTime().Format(timeLayout)), nil
}

// Read returns the file's text filtered by a case-insensitive search term and
// limited to its last maxLines lines. Messages (not found, empty, no match,
// read errors) are returned in-band as the original extension did.
func (s *Store) Read(name, search string, maxLines int) (string, error) {
	if maxLines < 1 || maxLines > MaxLinesLimit {
		return "", invalidf("max_lines must be between 1 and %d", MaxLinesLimit)
	}
	text, truncated, found, err := s.readText(name)
	if errors.Is(err, errInvalid) {
		return "", err
	}
	if err != nil {
		return "Error reading file: " + err.Error(), nil
	}
	if !found {
		return "File not found", nil
	}
	lines := strings.SplitAfter(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if search != "" {
		want := strings.ToLower(search)
		kept := lines[:0:0]
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l), want) {
				kept = append(kept, l)
			}
		}
		lines = kept
	}
	var content string
	if total := len(lines); total > maxLines {
		content = fmt.Sprintf("[Showing last %d of %d lines]\n\n", maxLines, total) + strings.Join(lines[total-maxLines:], "")
	} else {
		content = strings.Join(lines, "")
	}
	if strings.TrimSpace(content) == "" {
		if search != "" {
			return fmt.Sprintf("No matches found for '%s'", search), nil
		}
		return "Log file is empty", nil
	}
	if truncated {
		content = fmt.Sprintf("[Only the last %d MiB of the file were read]\n", maxRead>>20) + content
	}
	return content, nil
}

// DeleteResult is the status line and the file list after a delete.
type DeleteResult struct {
	Status string   `json:"status"`
	Files  []string `json:"files"`
}

// Delete removes one log file. Only a regular file of the logs directory with a
// valid name is removed; anything else is refused or reported as not found.
func (s *Store) Delete(name string) (DeleteResult, error) {
	p, err := s.path(name)
	if err != nil {
		return DeleteResult{}, err
	}
	s.mu.Lock()
	status := ""
	li, lerr := os.Lstat(p)
	switch {
	case os.IsNotExist(lerr):
		status = "File not found"
	case lerr != nil:
		status = "Error deleting file: " + lerr.Error()
	case !li.Mode().IsRegular():
		s.mu.Unlock()
		return DeleteResult{}, invalidf("%q is not a regular file", name)
	default:
		if err := os.Remove(p); err != nil {
			status = "Error deleting file: " + err.Error()
		} else {
			status = "Successfully deleted " + name
			s.opts.Logf("logview: deleted %s", name)
		}
	}
	s.mu.Unlock()
	files, err := s.Files()
	return DeleteResult{Status: status, Files: files}, err
}

// DeleteAll removes every regular *.log file of the logs directory.
func (s *Store) DeleteAll() (DeleteResult, error) {
	s.mu.Lock()
	es, err := s.entries()
	if err != nil {
		s.mu.Unlock()
		return DeleteResult{}, err
	}
	deleted := 0
	var errs []string
	for _, e := range es {
		if err := os.Remove(filepath.Join(s.dir, e.name)); err != nil {
			errs = append(errs, e.name+": "+err.Error())
		} else {
			deleted++
		}
	}
	s.mu.Unlock()
	s.opts.Logf("logview: deleted %d log file(s)", deleted)
	status := fmt.Sprintf("Deleted %d log file(s)", deleted)
	if len(errs) > 0 {
		status += "\n\nErrors:\n" + strings.Join(errs, "\n")
	}
	files, err := s.Files()
	return DeleteResult{Status: status, Files: files}, err
}
