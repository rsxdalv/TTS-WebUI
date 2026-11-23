package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Error kinds, mapped to HTTP statuses by the handler.
var (
	errInvalid  = errors.New("invalid")   // 400
	errNotFound = errors.New("not found") // 404
	errExists   = errors.New("exists")    // 409
)

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errInvalid, fmt.Sprintf(format, a...))
}

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// validSegment reports whether s is a safe single path element on every
// platform we run on (Windows is the strictest).
func validSegment(s string) bool {
	if s == "" || s == "." || s == ".." || len(s) > 200 {
		return false
	}
	if strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
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

// splitRel turns a client path into segments. Both separators are accepted
// (Gradio on Windows sends backslashes); absolute paths, drive letters, "..",
// empty segments and anything validSegment rejects are refused.
func splitRel(p string) ([]string, error) {
	if p == "" {
		return nil, invalidf("empty path")
	}
	segs := strings.Split(strings.ReplaceAll(p, "\\", "/"), "/")
	for _, s := range segs {
		if !validSegment(s) {
			return nil, invalidf("bad path element %q", s)
		}
	}
	return segs, nil
}

// collectionTop returns how many leading segments name a collection root:
// ["outputs"], ["favorites"] or ["collections", name].
func collectionTop(segs []string) (int, error) {
	switch segs[0] {
	case "outputs", "favorites":
		return 1, nil
	case "collections":
		if len(segs) < 2 {
			return 0, invalidf("collections needs a name")
		}
		return 2, nil
	}
	return 0, invalidf("path must start with outputs, favorites or collections/<name>")
}

// resolve checks that rel names an existing path inside a collection and that
// no symlink changes it: the fully resolved path must equal the resolved
// collection root plus the remaining segments. It returns the absolute path,
// the segments and the number of segments that make up the collection root.
func (s *Store) resolve(rel string) (string, []string, int, error) {
	segs, err := splitRel(rel)
	if err != nil {
		return "", nil, 0, err
	}
	top, err := collectionTop(segs)
	if err != nil {
		return "", nil, 0, err
	}
	rootReal, err := filepath.EvalSymlinks(s.opts.Root)
	if err != nil {
		return "", nil, 0, err
	}
	abs := filepath.Join(append([]string{rootReal}, segs...)...)
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, 0, fmt.Errorf("%w: %s", errNotFound, rel)
		}
		return "", nil, 0, err
	}
	// outputs/ or favorites/ may itself be a link the user made (another
	// disk); collections/<name> and everything below a root may not.
	var topReal string
	if segs[0] == "collections" {
		cr, err := filepath.EvalSymlinks(filepath.Join(rootReal, "collections"))
		if err != nil {
			return "", nil, 0, fmt.Errorf("%w: %s", errNotFound, rel)
		}
		topReal = filepath.Join(cr, segs[1])
	} else {
		topReal, err = filepath.EvalSymlinks(filepath.Join(rootReal, segs[0]))
		if err != nil {
			return "", nil, 0, fmt.Errorf("%w: %s", errNotFound, rel)
		}
	}
	want := filepath.Join(append([]string{topReal}, segs[top:]...)...)
	if !strings.EqualFold(filepath.Clean(real), filepath.Clean(want)) {
		return "", nil, 0, invalidf("path leaves the collection (symlink)")
	}
	return real, segs, top, nil
}

// resolveGeneration is resolve for a generation directory: a real directory
// strictly below a collection root.
func (s *Store) resolveGeneration(rel string) (string, []string, error) {
	abs, segs, top, err := s.resolve(rel)
	if err != nil {
		return "", nil, err
	}
	if len(segs) <= top {
		return "", nil, invalidf("%q is a collection, not a generation", rel)
	}
	fi, err := os.Lstat(abs)
	if err != nil {
		return "", nil, err
	}
	if !fi.IsDir() {
		return "", nil, invalidf("%q is not a directory", rel)
	}
	return abs, segs, nil
}

// resolveCollection accepts "outputs", "favorites" or "collections/<name>".
// It returns the absolute path and the canonical (slash) form.
func (s *Store) resolveCollection(rel string) (string, string, error) {
	abs, segs, top, err := s.resolve(rel)
	if err != nil {
		return "", "", err
	}
	if len(segs) != top {
		return "", "", invalidf("%q is not a collection", rel)
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return "", "", fmt.Errorf("%w: %s", errNotFound, rel)
	}
	return abs, strings.Join(segs, "/"), nil
}
