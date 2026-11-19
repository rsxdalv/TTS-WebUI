package react_ui_backend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJSNumber(t *testing.T) {
	for in, want := range map[string]string{
		"1.0": "1", "1e21": "1e+21", "1e20": "100000000000000000000", "0.0000001": "1e-7", "0.000001": "0.000001",
		"12345678901234567890": "12345678901234567000", "-0": "0", "-1.50": "-1.5", "0.1": "0.1",
		"0.30000000000000004": "0.30000000000000004", "1.5e-10": "1.5e-10", "123456789.125": "123456789.125",
		"1e999": "null", "5e-324": "5e-324", "1.7976931348623157e308": "1.7976931348623157e+308",
	} {
		v, err := parseJS([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := stringifyJS(v); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

func TestJSONKeyOrderAndStrings(t *testing.T) {
	v, err := parseJS([]byte(`{"b":1,"2":"x","1":"y","a":2,"b":3,"01":4}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := stringifyJS(v); got != `{"1":"y","2":"x","b":3,"a":2,"01":4}` {
		t.Errorf("order: %s", got)
	}
	v, _ = parseJS([]byte(`"<b>& \u0007\"\\\/é\n"`))
	if got := stringifyJS(v); got != "\"<b>& \\u0007\\\"\\\\/é\\n\"" {
		t.Errorf("string: %s", got)
	}
	for _, bad := range []string{`{`, `{"a":1} x`, string(rune(0xFEFF)) + "{}", ``, `{"a":}`} {
		if _, err := parseJS([]byte(bad)); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestParseMetadataDate(t *testing.T) {
	a, ok1 := parseMetadataDate("2024-08-30 16:19:51.647890")
	b, ok2 := parseMetadataDate("2024-08-30 16:19:51.647999")
	if !ok1 || !ok2 || a.UnixMilli() != b.UnixMilli() {
		t.Errorf("microseconds must be cut to milliseconds: %v %v", a, b)
	}
	if u, ok := parseMetadataDate("2023-05-27_13-06-13"); !ok || u.Hour() != 13 || u.Minute() != 6 {
		t.Errorf("underscore form: %v %v", u, ok)
	}
	if e, ok := parseMetadataDate("nodash"); !ok || e.UnixMilli() != 0 {
		t.Errorf("fallback is the epoch: %v %v", e, ok)
	}
	if _, ok := parseMetadataDate("yesterday evening"); ok {
		t.Error("unparseable text with a space is an Invalid Date")
	}
}

func TestV8SortMatchesNodeWithNaN(t *testing.T) {
	// Orders recorded from Node 22 for the comparator (a, b) => b - a with
	// NaN for the "bad" element: [d1 = 3, bad, d2 = 3, d3 = 1] sorts to d1 bad d4 d2 d3.
	mk := func(name string, ms int64, ok bool) outputItem {
		return outputItem{obj: func() *jsObject { o := newJSObject(); o.set("n", name); return o }(), date: time.UnixMilli(ms), ok: ok}
	}
	items := []outputItem{mk("d1", 3, true), mk("bad", 0, false), mk("d2", 3, true), mk("d3", 1, true), mk("d4", 5, true)}
	v8Sort(items, func(a, b outputItem) float64 {
		if !a.ok || !b.ok {
			return 0
		}
		return float64(b.date.UnixMilli() - a.date.UnixMilli())
	})
	var got []string
	for _, it := range items {
		n, _ := it.obj.get("n")
		got = append(got, n.(string))
	}
	if strings.Join(got, " ") != "d1 bad d4 d2 d3" {
		t.Errorf("got %v", got)
	}
}

func dataFixture(t *testing.T) (root string, srv *httptest.Server) {
	t.Helper()
	root = t.TempDir()
	put := func(coll, dir, json string) {
		d := filepath.Join(root, coll, dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, dir+".json"), []byte(json), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("coll", "new", `{"date":"2024-08-30 16:19:51.647890","text":"a","seed":1.0}`)
	put("coll", "old", `{"date":"2023-05-27_13-06-13","text":"b"}`)
	put("coll", "nodate", `{"text":"c"}`)
	put("coll", "broken", `{nope`)
	if err := os.WriteFile(filepath.Join(root, "coll", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.json"), []byte(`{"date":"2024-01-01 00:00:00","k":"SECRET"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	put(".hidden", "x", `{"date":"2024-01-01 00:00:00","k":"SECRET"}`)

	mux := http.NewServeMux()
	mux.Handle("GET /api/go-data/outputs/{name}", newDataOutputs(Deps{Root: root}))
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return root, srv
}

func TestDataOutputs(t *testing.T) {
	_, srv := dataFixture(t)
	resp, body := get(t, srv, "/api/go-data/outputs/coll", nil)
	sep := string(filepath.Separator)
	jsonSep := strings.ReplaceAll(sep, `\`, `\\`)
	want := `[{"date":"2024-08-30 16:19:51.647890","text":"a","seed":1,"semantic_prompt":null,"coarse_prompt":null,` +
		`"filename":"api/webui-generations/coll/new/new.ogg","folder_root":"coll` + jsonSep + `new","api_filename":"coll` + jsonSep + `new` + jsonSep + `new.ogg"},` +
		`{"date":"2023-05-27_13-06-13","text":"b","semantic_prompt":null,"coarse_prompt":null,` +
		`"filename":"api/webui-generations/coll/old/old.ogg","folder_root":"coll` + jsonSep + `old","api_filename":"coll` + jsonSep + `old` + jsonSep + `old.ogg"}]`
	if resp.StatusCode != 200 || body != want || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("%d\n got %s\nwant %s", resp.StatusCode, body, want)
	}
	// A missing directory and a file both give an empty list, as the original does.
	for _, name := range []string{"nosuch", "secret.json"} {
		if resp, body := get(t, srv, "/api/go-data/outputs/"+name, nil); resp.StatusCode != 200 || body != "[]" {
			t.Errorf("%s: %d %q", name, resp.StatusCode, body)
		}
	}
}

// The endpoint cannot be pointed outside the install root, and no dotted
// directory is listed.
func TestDataOutputsRejectsBadNames(t *testing.T) {
	_, srv := dataFixture(t)
	for _, p := range []string{
		"/api/go-data/outputs/.hidden", "/api/go-data/outputs/..", "/api/go-data/outputs/.",
		"/api/go-data/outputs/%2e%2e", "/api/go-data/outputs/%2ehidden",
		"/api/go-data/outputs/..%5ccoll", "/api/go-data/outputs/a%2fb",
	} {
		resp, body := get(t, srv, p, nil)
		if resp.StatusCode == 200 || strings.Contains(body, "SECRET") {
			t.Errorf("%s: %d %q", p, resp.StatusCode, body)
		}
	}
	for _, name := range []string{"", ".", "..", ".hidden", "a/b", `a\b`, "a\x00b"} {
		if validCollectionName(name) {
			t.Errorf("%q accepted", name)
		}
	}
	if !validCollectionName("outputs") || !validCollectionName("t coll") || !validCollectionName("my.coll") {
		t.Error("ordinary names must be accepted")
	}
}

func TestDataOutputsNonStringDateIs500WithTwoItems(t *testing.T) {
	root, srv := dataFixture(t)
	d := filepath.Join(root, "coll2", "x")
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, "x.json"), []byte(`{"date":1700000000}`), 0o644)
	if resp, body := get(t, srv, "/api/go-data/outputs/coll2", nil); resp.StatusCode != 200 || !strings.HasPrefix(body, "[") {
		t.Fatalf("one item is not sorted, so no throw: %d %q", resp.StatusCode, body)
	}
	d = filepath.Join(root, "coll2", "y")
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, "y.json"), []byte(`{"date":"2024-01-01 00:00:00"}`), 0o644)
	resp, _ := get(t, srv, "/api/go-data/outputs/coll2", nil)
	if resp.StatusCode != 500 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
