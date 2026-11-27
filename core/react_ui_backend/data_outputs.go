package react_ui_backend

// Data endpoint for frontend/src/pages/outputs/[name].tsx (task B5a): the
// `outputs` array that getStaticProps builds with getDataFromJSON(name) in
// frontend/src/data/getVoicesData.tsx. Behind the claim switch data-outputs.
//
// getDataFromJSON reads <root>/<name>/<dir>/<dir>.json for every entry of
// <root>/<name>, keeps those with a truthy "date", and sorts them by date,
// newest first. This port returns the same JSON bytes.
//
// Fixes over the original (CONTRACTS.md section 7): the name must be one path
// segment that does not start with ".", otherwise 404, so the endpoint cannot
// list or read outside the install root.
//
// Known gaps: dates that V8's Date.parse accepts but this port does not (only
// "YYYY-MM-DD HH:MM[:SS[.fff]]" with a space, and the YYYY-MM-DD_HH-MM-SS form
// are understood) compare as equal, and a "date" that is an array sorts as the epoch.
// A number, boolean or object "date" is a 500 when two or more items are
// listed, where the original throws in its sort.

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "data-outputs",
		Patterns: []string{"GET /api/go-data/outputs/{name}"},
		Handler:  newDataOutputs,
	})
}

func newDataOutputs(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !validCollectionName(name) {
			http.NotFound(w, r)
			return
		}
		outputs, err := dataFromJSON(deps.Root, name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError) // getStaticProps throws
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, stringifyJS(outputs))
	})
}

// validCollectionName: one segment, not starting with ".".
func validCollectionName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	return !windowsUnsafeSegment(name)
}

type outputItem struct {
	obj  *jsObject
	date time.Time
	ok   bool // the date could be parsed
}

// dataFromJSON is getDataFromJSON(collection).
func dataFromJSON(root, collection string) ([]any, error) {
	base := filepath.Join(root, collection)
	out := []any{}
	names := readdirNames(base)
	var items []outputItem
	badDate := false
	for _, dirname := range names {
		raw, err := os.ReadFile(filepath.Join(base, dirname, dirname+".json"))
		if err != nil {
			continue
		}
		parsed, err := parseJS(raw)
		if err != nil {
			continue
		}
		result, isObj := parsed.(*jsObject)
		if !isObj {
			continue // spreading anything else leaves no "date"
		}
		item := generateResult(result, collection, dirname, filepath.Join(base, dirname))
		date, _ := item.get("date")
		if !truthy(date) {
			continue
		}
		t, ok := parseMetadataDate(date)
		items = append(items, outputItem{obj: item, date: t, ok: ok})
		if _, isString := date.(string); !isString {
			if _, isArray := date.([]any); !isArray {
				badDate = true
			}
		}
	}
	if badDate && len(items) > 1 {
		// date.includes is not a function: the sort comparator throws.
		return nil, errBadDate
	}
	// Newest first. The comparator is b - a on getTime(), where an Invalid Date
	// gives NaN, which the sort treats as 0; V8's own algorithm is used so the
	// order matches even then.
	v8Sort(items, func(a, b outputItem) float64 {
		if !a.ok || !b.ok {
			return 0
		}
		return float64(b.date.UnixMilli() - a.date.UnixMilli())
	})
	for _, it := range items {
		out = append(out, it.obj)
	}
	return out, nil
}

var errBadDate = errors.New("date is not a string")

// readdirNames lists a directory in the order Node's fs.readdir does: the
// operating system's order on Windows, sorted on other systems (libuv sorts).
// A directory that cannot be read has no entries.
func readdirNames(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	names, _ := f.Readdirnames(-1)
	if runtime.GOOS != "windows" {
		sort.Strings(names)
	}
	return names
}

// generateResult adds the fields the page reads. The audio is the .ogg the
// original named; a folder that has a .wav and no .ogg (the Chatterbox and openai
// generations, which write no .ogg) points at the .wav, which the page can play.
func generateResult(result *jsObject, collection, dirname, dir string) *jsObject {
	item := newJSObject()
	for _, k := range result.keys {
		item.set(k, result.vals[k])
	}
	ext := ".ogg"
	if _, err := os.Stat(filepath.Join(dir, dirname+".ogg")); err != nil {
		if _, err := os.Stat(filepath.Join(dir, dirname+".wav")); err == nil {
			ext = ".wav"
		}
	}
	coreFilename := filepath.Join(dirname, dirname+ext)
	item.set("semantic_prompt", nil)
	item.set("coarse_prompt", nil)
	item.set("filename", filepath.ToSlash(filepath.Join("api", "webui-generations", collection, coreFilename)))
	item.set("folder_root", filepath.Join(collection, dirname))
	item.set("api_filename", filepath.Join(collection, coreFilename))
	return item
}

// truthy is JavaScript truthiness for a parsed JSON value.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && t == t
	case string:
		return t != ""
	}
	return true
}

var metadataDateRE = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2}`)

// parseMetadataDate is components/parseMetadataDate.tsx. ok is false for an
// Invalid Date.
func parseMetadataDate(v any) (time.Time, bool) {
	date, isString := v.(string)
	if !isString {
		return time.UnixMilli(0), true
	}
	if strings.Contains(date, " ") {
		return parseSpaceDate(date)
	}
	if metadataDateRE.MatchString(date) {
		raw := dateSplitRE.Split(date, -1) // like split(/[-_]/): empty strings are kept
		if len(raw) < 6 {
			return time.Time{}, false
		}
		var n [6]int64
		for i := 0; i < 6; i++ {
			x, ok := jsParseInt(raw[i])
			if !ok {
				return time.Time{}, false
			}
			n[i] = x
		}
		return time.Date(int(n[0]), time.Month(n[1]), int(n[2]), int(n[3]), int(n[4]), int(n[5]), 0, time.Local), true
	}
	return time.UnixMilli(0), true
}

var dateSplitRE = regexp.MustCompile(`[-_]`)

var spaceDateRE = regexp.MustCompile(`^\s*(\d{4})-(\d{1,2})-(\d{1,2})[ T](\d{1,2}):(\d{2})(?::(\d{2})(?:\.(\d+))?)?\s*$`)

// parseSpaceDate understands "2024-08-30 16:19:51.647890", read as local time
// with millisecond resolution, which is what V8 does with it.
func parseSpaceDate(s string) (time.Time, bool) {
	m := spaceDateRE.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	atoi := func(x string) int {
		n := 0
		for _, c := range x {
			n = n*10 + int(c-'0')
		}
		return n
	}
	ms := 0
	if m[7] != "" {
		frac := (m[7] + "000")[:3]
		ms = atoi(frac)
	}
	t := time.Date(atoi(m[1]), time.Month(atoi(m[2])), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6]), ms*1e6, time.Local)
	if t.Month() != time.Month(atoi(m[2])) || t.Day() != atoi(m[3]) {
		return time.Time{}, false // 2024-13-45 is an Invalid Date
	}
	return t, true
}

// windowsUnsafeSegment reports names Windows would treat as a drive-relative
// path, a stream or a device, so they never reach the file system.
func windowsUnsafeSegment(name string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if strings.ContainsAny(name, ":<>\"|?*") {
		return true
	}
	stem := strings.ToUpper(strings.TrimRight(strings.SplitN(name, ".", 2)[0], " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	return len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9'
}

// v8Sort is Array.prototype.sort as V8 does it. Below 64 elements V8 finds the
// first run and finishes with a binary insertion sort, which this reproduces
// exactly. Larger arrays are merged by a stable sort; the two only differ when
// the comparator is inconsistent.
func v8Sort(a []outputItem, cmp func(x, y outputItem) float64) {
	n := len(a)
	if n < 2 {
		return
	}
	if n >= 64 {
		sort.SliceStable(a, func(i, j int) bool { return cmp(a[i], a[j]) < 0 })
		return
	}
	// CountAndMakeRun(0, n)
	runLength := 2
	descending := cmp(a[1], a[0]) < 0
	for i := 2; i < n; i++ {
		order := cmp(a[i], a[i-1])
		if descending && order >= 0 || !descending && order < 0 {
			break
		}
		runLength++
	}
	if descending {
		for lo, hi := 0, runLength-1; lo < hi; lo, hi = lo+1, hi-1 {
			a[lo], a[hi] = a[hi], a[lo]
		}
	}
	// BinaryInsertionSort(0, runLength, n)
	for start := runLength; start < n; start++ {
		left, right, pivot := 0, start, a[start]
		for left < right {
			mid := left + (right-left)>>1
			if cmp(pivot, a[mid]) < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		copy(a[left+1:start+1], a[left:start])
		a[left] = pivot
	}
}
