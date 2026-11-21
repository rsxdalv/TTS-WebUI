package history

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// row is one line of the History table: [date, name, when, path].
type row []string

func (r row) path() string { return r[3] }

// rows reads the table of a collection.
func rows(t *testing.T, s *Store, collection string) []row {
	t.Helper()
	got, err := s.Rows(collection)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]row, 0, len(got))
	for _, r := range got {
		out = append(out, row(r))
	}
	return out
}

func TestRowsOrderAndNames(t *testing.T) {
	s, root := newStore(t)
	// The names the old Python tab was written for.
	gen(t, root, "outputs", "2023-06-02_10-54-24__bark__None")
	gen(t, root, "outputs", "audio__bark__continued_generation__2023-05-21_15-36-14_long")
	gen(t, root, "outputs", "audio__bark__continued_generation__2023-05-21_15-36-14")
	gen(t, root, "outputs", "no_date_here")

	got := rows(t, s, "outputs")
	if len(got) != 4 {
		t.Fatalf("got %d rows: %v", len(got), got)
	}
	// Newest first. The two folders of 2023-05-21 have the same timestamp, so
	// the path decides (the Python tab kept the order of the directory scan).
	want := []struct{ date, name, path string }{
		{"2023-06-02T10:54:24", "Bark history: none", "outputs/2023-06-02_10-54-24__bark__None/2023-06-02_10-54-24__bark__None.wav"},
		{"2023-05-21T15:36:14", "Bark history: continued", "outputs/audio__bark__continued_generation__2023-05-21_15-36-14/audio__bark__continued_generation__2023-05-21_15-36-14.wav"},
		{"2023-05-21T15:36:14", "Bark history: continued (long)", "outputs/audio__bark__continued_generation__2023-05-21_15-36-14_long/audio__bark__continued_generation__2023-05-21_15-36-14_long.wav"},
		{"1970-01-01T00:00:00", "No_date_here", "outputs/no_date_here/no_date_here.wav"},
	}
	for i, w := range want {
		if got[i][0] != w.date {
			t.Errorf("row %d date = %q, want %q", i, got[i][0], w.date)
		}
		if got[i][1] != w.name {
			t.Errorf("row %d name = %q, want %q", i, got[i][1], w.name)
		}
		if got[i].path() != w.path {
			t.Errorf("row %d path = %q, want %q", i, got[i].path(), w.path)
		}
	}
	// The name without a date cannot be dated, so the age is unknown.
	if got[3][2] != "Unknown" {
		t.Errorf("when = %q, want Unknown", got[3][2])
	}
	// The others know how old they are.
	for i := 0; i < 3; i++ {
		if !strings.HasSuffix(got[i][2], "ago") {
			t.Errorf("row %d when = %q", i, got[i][2])
		}
	}
}

func TestRowsSameDateKeepsPathOrder(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "b_2023-06-02_10-54-24")
	gen(t, root, "outputs", "a_2023-06-02_10-54-24")
	gen(t, root, "outputs", "c_2023-06-02_10-54-24")
	got := rows(t, s, "outputs")
	if len(got) != 3 {
		t.Fatalf("got %d rows", len(got))
	}
	// Same timestamp: the paths decide, so the table is the same every time.
	if got[0].path() > got[1].path() || got[1].path() > got[2].path() {
		t.Fatalf("not ordered: %v", got)
	}
}

func TestRowsUnparsableDate(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "2023-13-45_99-99-99_broken")
	gen(t, root, "outputs", "2023-02-30_10-00-00_no_day")
	gen(t, root, "outputs", "2023-05-16_11-45-00_ok")
	got := rows(t, s, "outputs")
	if len(got) != 3 {
		t.Fatalf("got %d rows: %v", len(got), got)
	}
	// The two broken ones fall back to 1970 and are the oldest.
	for i := 1; i < 3; i++ {
		if got[i][0] != "1970-01-01T00:00:00" {
			t.Errorf("row %d date = %q, want the 1970 fallback", i, got[i][0])
		}
		if got[i][2] != "Unknown" {
			t.Errorf("row %d when = %q, want Unknown", i, got[i][2])
		}
	}
	if got[0][0] != "2023-05-16T11:45:00" {
		t.Errorf("the good one = %q", got[0][0])
	}
}

func TestRowsEmptyAndMissing(t *testing.T) {
	s, root := newStore(t)
	if err := os.MkdirAll(filepath.Join(root, "outputs"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := rows(t, s, "outputs")
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if _, err := s.Rows("collections/nope"); !errors.Is(err, errNotFound) {
		t.Fatalf("missing collection: %v", err)
	}
	// A generation folder is not a collection.
	if err := os.MkdirAll(filepath.Join(root, "outputs", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rows("outputs/x"); !errors.Is(err, errInvalid) {
		t.Fatalf("not a collection: %v", err)
	}
	for _, bad := range []string{"", "../secret", "outputs/..", "secret", "C:/Windows"} {
		if _, err := s.Rows(bad); err == nil {
			t.Errorf("Rows(%q) succeeded", bad)
		}
	}
}

func TestRowsSkipsWhatCannotBeSelected(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "good")
	// A folder without a .wav: the old tab listed it, the row points at a
	// file that is not there.
	gen(t, root, "outputs", "nowav")
	if err := os.Remove(filepath.Join(root, "outputs", "nowav", "nowav.wav")); err != nil {
		t.Fatal(err)
	}
	// A hidden folder and a link cannot be selected.
	if err := os.MkdirAll(filepath.Join(root, "outputs", ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outputs", "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := rows(t, s, "outputs")
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	for _, r := range got {
		if strings.Contains(r.path(), "link") || strings.Contains(r.path(), ".hidden") {
			t.Errorf("listed %q", r.path())
		}
	}
	// The folder without a .wav is still listed (the old tab listed it), but
	// selecting it fails: there is no audio.
	if _, err := s.Select("outputs/nowav/nowav.wav"); err == nil {
		t.Error("selected a generation without a wav")
	}
}

func TestRowsInFavoritesAndCollection(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "favorites", "2023-06-02_10-54-24__bark__None")
	gen(t, root, "collections/mine", "2023-06-02_10-54-24__bark__None")
	got := rows(t, s, "favorites")
	if len(got) != 1 || got[0].path() != "favorites/2023-06-02_10-54-24__bark__None/2023-06-02_10-54-24__bark__None.wav" {
		t.Fatalf("favorites: %v", got)
	}
	got = rows(t, s, "collections/mine")
	if len(got) != 1 || got[0].path() != "collections/mine/2023-06-02_10-54-24__bark__None/2023-06-02_10-54-24__bark__None.wav" {
		t.Fatalf("collection: %v", got)
	}
}

func TestRowsHTTP(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "2023-06-02_10-54-24__bark__None")
	h := s.Handler()
	c, raw := callRaw(t, h, "GET", "/api/history/rows?directory=outputs", "", "tok")
	if c != 200 {
		t.Fatalf("rows: %d %s", c, raw)
	}
	if !strings.Contains(raw, `"2023-06-02T10:54:24"`) ||
		!strings.Contains(raw, `"Bark history: none"`) ||
		!strings.Contains(raw, `"outputs/2023-06-02_10-54-24__bark__None/2023-06-02_10-54-24__bark__None.wav"`) {
		t.Errorf("rows: %s", raw)
	}
	if c, _ = call(t, h, "GET", "/api/history/rows?directory=../secret", "", "tok"); c != 400 {
		t.Errorf("rows traversal: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/rows", "{}", "tok"); c != 405 {
		t.Errorf("POST rows: %d", c)
	}
}

// markerValue is the value of a Gradio marker of a component.
func markerValue(t *testing.T, ui map[string]any, key string) any {
	t.Helper()
	marker, ok := ui[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a marker: %v", key, ui[key])
	}
	props, ok := marker["props"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no props: %v", key, marker)
	}
	return props["value"]
}

func TestSelectWithoutImage(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "a")
	if err := os.Remove(filepath.Join(root, "outputs", "a", "a.png")); err != nil {
		t.Fatal(err)
	}
	ui, err := s.Select("outputs/a/a.wav")
	if err != nil {
		t.Fatal(err)
	}
	if v := markerValue(t, ui, "image"); v != nil {
		t.Fatalf("image = %v, want no value", v)
	}
	// With a picture the image is shown.
	gen(t, root, "outputs", "b")
	ui, err = s.Select("outputs/b/b.png")
	if err != nil {
		t.Fatal(err)
	}
	if v := markerValue(t, ui, "image"); v != "outputs/b/b.png" {
		t.Fatalf("image = %v", v)
	}
}

func TestRelativeDate(t *testing.T) {
	now := time.Date(2023, 6, 2, 12, 0, 0, 0, time.Local)
	// The answers of the old Python tab for the same ages.
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{0, "0 seconds ago"},
		{45 * time.Second, "45 seconds ago"},
		{60 * time.Second, "60 seconds ago"},
		{61 * time.Second, "1 minutes ago"},
		{60 * time.Minute, "60 minutes ago"},
		{90 * time.Minute, "1 hours ago"},
		{61 * time.Minute, "1 hours ago"},
		{23*time.Hour + 59*time.Minute, "23 hours ago"},
		{24 * time.Hour, "1 days ago"},
		{49 * time.Hour, "2 days ago"},
	}
	for _, c := range cases {
		if got := relativeDate(now.Add(-c.ago), now, true); got != c.want {
			t.Errorf("%s ago = %q, want %q", c.ago, got, c.want)
		}
	}
	if got := relativeDate(now, now, false); got != "Unknown" {
		t.Errorf("unknown = %q", got)
	}
	// A date in the future: the difference is negative, and the days carry the
	// sign, like a Python timedelta.
	if got := relativeDate(now.Add(time.Hour), now, true); got != "23 hours ago" {
		t.Errorf("future = %q", got)
	}
	if got := relativeDate(now.Add(24*time.Hour), now, true); got != "0 seconds ago" {
		t.Errorf("future day = %q", got)
	}
}

func TestPrettyName(t *testing.T) {
	// The names and the answers of the old Python tab
	// (tts_webui/history_tab/generate_pretty_name.py).
	cases := map[string]string{
		"2023-06-02_10-54-24__bark__None":                             "Bark history: none",
		"audio__bark__continued_generation__2023-05-21_15-36-14_long": "Bark history: continued (long)",
		"audio__bark__continued_generation__2023-05-21_15-36-14":      "Bark history: continued",
		"audio__tortoise__random__2023-05-31_14-19-13__n0":            "Tortoise random  n0",
		"audio__bark__continued_from_2023-05-31_14-19-13":             "Bark continued_history:",
		"plain":                      "Plain",
		"ALL_CAPS":                   "All_caps",
		"no_date_here":               "No_date_here",
		"2023-13-45_99-99-99_broken": "_broken",
		"2023-02-30_10-00-00_no_day": "_no_day",
		"audio__bark__None__2023-05-16_11-45-00_long": "Bark history: none (long)",
	}
	for in, want := range cases {
		if got := prettyName(in); got != want {
			t.Errorf("prettyName(%q) = %q, want %q", in, got, want)
		}
	}
}
