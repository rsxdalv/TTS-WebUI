package history

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// stampRe matches the timestamp the generators put in a folder name:
// 2023-05-16_11-45-00. The Python tab found the first match in the path and
// parsed it; a name without one was "unparsable".
var stampRe = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2}`)

// isoLayout is datetime.isoformat() for a value without microseconds.
const isoLayout = "2006-01-02T15:04:05"

// Rows lists the generation folders of a collection for the History tab
// (Outputs > History and its Favorites and Collections siblings). Every row is
// [date, name, when, path]:
//
//   - date: the timestamp of the folder name, ISO 8601; 1970-01-01T00:00:00
//     when the name has no parsable timestamp;
//   - name: the pretty name (see prettyName);
//   - when: "3 days ago" (see relativeDate) or "Unknown";
//   - path: the .wav file of the folder, relative to the project root.
//
// Newest first; folders with the same timestamp keep the path order.
//
// The rows are the table of the tab: the Python side only hands them to
// gr.Dataframe. The date is an ISO string, which is what Gradio sends to the
// browser for a datetime object anyway.
func (s *Store) Rows(collection string) ([][]string, error) {
	abs, canon, err := s.resolveCollection(collection)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	type item struct {
		date string
		name string
		when string
		path string
	}
	items := make([]item, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		// Only real folders: a link is not followed (path safety), and a name
		// that the path rules refuse could not be selected anyway.
		if !e.IsDir() || !validSegment(name) {
			continue
		}
		rel := canon + "/" + name
		stamp, ok := parseStamp(rel)
		if !ok {
			stamp = time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
		}
		items = append(items, item{
			date: stamp.Format(isoLayout),
			name: prettyName(name),
			when: relativeDate(stamp, now, ok),
			path: rel + "/" + name + ".wav",
		})
	}
	// Newest first; folders with the same timestamp keep the path order.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].date != items[j].date {
			return items[i].date > items[j].date
		}
		return items[i].path < items[j].path
	})
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, []string{it.date, it.name, it.when, it.path})
	}
	return rows, nil
}

// parseStamp finds the first timestamp in a path and parses it. It reports
// false when there is none or when it is not a real date (the Python tab
// caught the exception and treated the name as unparsable).
func parseStamp(text string) (time.Time, bool) {
	m := stampRe.FindString(text)
	if m == "" {
		return time.Time{}, false
	}
	date, clock, ok := strings.Cut(m, "_")
	if !ok {
		return time.Time{}, false
	}
	ys := strings.Split(date, "-")
	ts := strings.Split(clock, "-")
	if len(ys) != 3 || len(ts) != 3 {
		return time.Time{}, false
	}
	num := make([]int, 6)
	for i, s := range []string{ys[0], ys[1], ys[2], ts[0], ts[1], ts[2]} {
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Time{}, false
		}
		num[i] = n
	}
	y, mo, d, h, mi, sec := num[0], num[1], num[2], num[3], num[4], num[5]
	if y < 1 || y > 9999 || mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) ||
		h > 23 || mi > 59 || sec > 59 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(mo), d, h, mi, sec, 0, time.Local), true
}

// daysIn is the number of days of a month, like calendar.monthrange.
func daysIn(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	}
	return 0
}

// relativeDate is the "when" column: how long ago the folder was made. The
// Python tab subtracted two naive datetimes, so the difference is wall clock
// and not the elapsed time; a name without a timestamp was "Unknown".
func relativeDate(stamp, now time.Time, known bool) string {
	if !known {
		return "Unknown"
	}
	total := int64(wallClock(now).Sub(wallClock(stamp)) / time.Second)
	days := floorDiv(total, 86400)
	rest := total - days*86400
	switch {
	case days > 0:
		return fmt.Sprintf("%d days ago", days)
	case rest > 3600:
		return fmt.Sprintf("%d hours ago", rest/3600)
	case rest > 60:
		return fmt.Sprintf("%d minutes ago", rest/60)
	default:
		return fmt.Sprintf("%d seconds ago", rest)
	}
}

// wallClock drops the zone: the year, month, day, hour, minute and second as
// a UTC time, so the difference is the wall clock difference.
func wallClock(t time.Time) time.Time {
	y, mo, d := t.Date()
	h, mi, s := t.Clock()
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
}

// floorDiv divides with the floor, so a negative difference keeps the sign in
// the days (like a Python timedelta).
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// prettyName is the name of a generation: the folder name without the
// timestamp, with the words the generators put in it spelled out. The rules
// are the ones of the old Python tab (generate_pretty_name.py, now gone), in
// the same order.
func prettyName(name string) string {
	n := stampRe.ReplaceAllString(name, "")
	n = strings.ReplaceAll(n, "audio", "")
	n = strings.ReplaceAll(n, "_long", "(long)")
	n = strings.ReplaceAll(n, "continued_generation", "history: continued")
	n = strings.ReplaceAll(n, "from_", "history: ")
	n = strings.ReplaceAll(n, "None", "history: none")
	n = strings.ReplaceAll(n, "__", " ")
	return capitalize(strings.TrimSpace(n))
}

// capitalize is Python's str.capitalize(): the first letter in upper case, the
// rest in lower case.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToTitle(r[0])
	return string(r)
}
