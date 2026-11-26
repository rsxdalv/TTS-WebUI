package hfcache

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestParityDump is the Go half of testdata/parity.py: it scans (and optionally
// deletes from) a cache the script built and writes the rows as JSON so the
// script can compare them with huggingface_hub's. It does nothing unless
// HFCACHE_PARITY_DIR is set.
func TestParityDump(t *testing.T) {
	dir := os.Getenv("HFCACHE_PARITY_DIR")
	out := os.Getenv("HFCACHE_PARITY_OUT")
	if dir == "" || out == "" {
		t.Skip("set HFCACHE_PARITY_DIR and HFCACHE_PARITY_OUT (see testdata/parity.py)")
	}
	info, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{}
	if hashes := os.Getenv("HFCACHE_PARITY_DELETE"); hashes != "" {
		var sel []Selector
		for _, h := range strings.Split(hashes, ",") {
			sel = append(sel, Selector{CommitHash: h})
		}
		plan, err := PlanDelete(info, sel)
		if err != nil {
			t.Fatal(err)
		}
		res := plan.Execute()
		result["errors"] = res.Errors
		result["expected_freed"] = plan.ExpectedFreed
		result["not_found"] = len(plan.NotFound)
		if info, err = Scan(dir); err != nil {
			t.Fatal(err)
		}
	}
	result["rows"] = info.Rows(time.Now())
	result["warnings"] = info.Warnings
	result["size_on_disk"] = info.SizeOnDisk
	b, _ := json.Marshal(result)
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
