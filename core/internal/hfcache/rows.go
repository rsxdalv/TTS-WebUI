package hfcache

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Headers are the keys of a row, in the order get_headers_json gives them.
var Headers = []string{
	"repo_id", "repo_type", "commit_hash", "size_on_disk_str", "repo_size_on_disk",
	"revision_size_on_disk", "nb_files", "last_accessed_str", "last_accessed",
	"last_modified_str", "last_modified", "refs", "snapshot_path",
}

// Row is one revision, with the keys of scan_cache.get_rows_json.
type Row struct {
	RepoID             string  `json:"repo_id"`
	RepoType           string  `json:"repo_type"`
	CommitHash         string  `json:"commit_hash"`
	SizeOnDiskStr      string  `json:"size_on_disk_str"`
	RepoSizeOnDisk     int64   `json:"repo_size_on_disk"`
	RevisionSizeOnDisk int64   `json:"revision_size_on_disk"`
	NbFiles            int     `json:"nb_files"`
	LastAccessedStr    string  `json:"last_accessed_str"`
	LastAccessed       float64 `json:"last_accessed"`
	LastModifiedStr    string  `json:"last_modified_str"`
	LastModified       float64 `json:"last_modified"`
	Refs               string  `json:"refs"`
	SnapshotPath       string  `json:"snapshot_path"`
}

// Rows lists every revision of every repo (repos by folder name, revisions by
// commit hash). now is what "x ago" is measured from.
func (in Info) Rows(now time.Time) []Row {
	repos := append([]Repo(nil), in.Repos...)
	sort.Slice(repos, func(i, j int) bool { return repos[i].Path < repos[j].Path })
	rows := []Row{}
	for _, r := range repos {
		for _, rev := range r.Revisions {
			rows = append(rows, Row{
				RepoID:             r.ID,
				RepoType:           r.Type,
				CommitHash:         rev.CommitHash,
				SizeOnDiskStr:      fmt.Sprintf("%12s", FormatSize(rev.SizeOnDisk)),
				RepoSizeOnDisk:     r.SizeOnDisk,
				RevisionSizeOnDisk: rev.SizeOnDisk,
				NbFiles:            len(rev.Files),
				LastAccessedStr:    FormatTimeSince(now, r.LastAccessed),
				LastAccessed:       r.LastAccessed,
				LastModifiedStr:    FormatTimeSince(now, rev.LastModified),
				LastModified:       rev.LastModified,
				Refs:               strings.Join(rev.Refs, ", "),
				SnapshotPath:       rev.SnapshotPath,
			})
		}
	}
	return rows
}

// FormatSize is huggingface_hub's _format_size: powers of 1000, one decimal.
func FormatSize(n int64) string {
	f := float64(n)
	for _, unit := range []string{"", "K", "M", "G", "T", "P", "E", "Z"} {
		if math.Abs(f) < 1000.0 {
			return fmt.Sprintf("%3.1f%s", f, unit)
		}
		f /= 1000.0
	}
	return fmt.Sprintf("%.1fY", f)
}

// FormatTimeSince is huggingface_hub's format_timesince ("3 days ago"): the
// first unit whose rounded value fits its limit; Python's round() is half to
// even, which math.RoundToEven is.
func FormatTimeSince(now time.Time, ts float64) string {
	delta := float64(now.UnixNano())/1e9 - ts
	if delta < 20 {
		return "a few seconds ago"
	}
	chunks := []struct {
		label   string
		divider float64
		max     float64 // < 0: no limit
	}{
		{"second", 1, 59},
		{"minute", 60, 59},
		{"hour", 3600, 23},
		{"day", 86400, 6},
		{"week", 86400 * 7, 6},
		{"month", 86400 * 30, 11},
		{"year", 86400 * 365, -1},
	}
	var value float64
	var label string
	for _, c := range chunks {
		value, label = math.RoundToEven(delta/c.divider), c.label
		if c.max >= 0 && value <= c.max {
			break
		}
	}
	s := ""
	if value > 1 {
		s = "s"
	}
	return fmt.Sprintf("%d %s%s ago", int64(value), label, s)
}
