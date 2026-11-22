package extcatalog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden file holds what CPython's json.loads answered for a list of
// inputs: the error message (with its position), or the value and how it is
// dumped again. It is written by
// testdata/make_pyjson_golden.py (run it with the project python to refresh
// it after changing the list of cases).
//
// The Extension Catalog tab shows that error to the user, so Go's scanner has
// to answer exactly like CPython.
func TestPyParseMatchesTheCPythonGolden(t *testing.T) {
	path := filepath.Join("testdata", "pyjson_golden.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	type record struct {
		Input string `json:"input"`
		Error string `json:"error"`
		Value any    `json:"value"`
		Dump  string `json:"dump"`
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	cases := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("golden line: %v", err)
		}
		cases++
		v, perr := pyParse(rec.Input)
		if rec.Error != "" {
			if perr == nil {
				t.Errorf("%q: parsed, want the error %q", rec.Input, rec.Error)
				continue
			}
			if perr.Error() != rec.Error {
				t.Errorf("%q: %q, want %q", rec.Input, perr.Error(), rec.Error)
			}
			continue
		}
		if perr != nil {
			t.Errorf("%q: %v, want the value %s", rec.Input, perr, rec.Dump)
			continue
		}
		got := string(Marshal(v, ""))
		// The golden dump uses Python's default separators (", " and ": ");
		// Go writes them compactly. Compare the values, not the spacing.
		if !sameJSON(t, got, rec.Dump) {
			t.Errorf("%q: %s, want %s", rec.Input, got, rec.Dump)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if cases < 50 {
		t.Fatalf("only %d cases in the golden file", cases)
	}
}

// sameJSON compares two JSON texts by value: both are read and written again
// in the same (compact) way, so only the spacing can differ.
func sameJSON(t *testing.T, a, b string) bool {
	t.Helper()
	av, err := Parse([]byte(a))
	if err != nil {
		return false
	}
	bv, err := Parse([]byte(b))
	if err != nil {
		return false
	}
	return string(Marshal(av, "")) == string(Marshal(bv, ""))
}
