package logview

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errorPatterns   = []string{"ERROR:", "Error:", "error:", "FAILED", "Failed", "failed", "Could not", "could not", "Exception:", "Traceback"}
	warningPatterns = []string{"WARNING:", "Warning:", "warning:", "deprecated", "DEPRECATION"}
	successPatterns = []string{"Successfully installed", "Successfully uninstalled", "Requirement already satisfied", "finished with status 'done'"}
)

func matchesAny(line string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

// unique keeps the first occurrence of each line, in order. (The original
// listed an arbitrary subset of a Python set; first-seen is deterministic.)
func unique(lines []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

// Analyze is the Markdown analysis of a pip install/uninstall log: status,
// counts, and the first unique success (5), error (10) and warning (5) lines.
func (s *Store) Analyze(name string) (string, error) {
	text, _, found, err := s.readText(name)
	if errors.Is(err, errInvalid) {
		return "", err
	}
	if err != nil {
		return "Error analyzing file: " + err.Error(), nil
	}
	if !found {
		return "File not found", nil
	}
	var errs, warns, oks []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if matchesAny(line, errorPatterns) {
			errs = append(errs, t)
		}
		if matchesAny(line, warningPatterns) {
			warns = append(warns, t)
		}
		if matchesAny(line, successPatterns) {
			oks = append(oks, t)
		}
	}
	errs, warns, oks = unique(errs), unique(warns), unique(oks)

	var r []string
	r = append(r, fmt.Sprintf("# 📊 Analysis: %s\n", name))
	switch {
	case len(errs) > 0:
		r = append(r, "## ❌ Status: FAILED\n")
	case len(oks) > 0:
		r = append(r, "## ✅ Status: SUCCESS\n")
	default:
		r = append(r, "## ❓ Status: UNKNOWN\n")
	}
	r = append(r,
		fmt.Sprintf("**Total Lines:** %d", len(lines)),
		fmt.Sprintf("**Errors Found:** %d", len(errs)),
		fmt.Sprintf("**Warnings Found:** %d", len(warns)),
		fmt.Sprintf("**Success Messages:** %d\n", len(oks)))
	section := func(title string, items []string, max int) {
		if len(items) == 0 {
			return
		}
		r = append(r, title)
		for _, it := range items[:min(len(items), max)] {
			r = append(r, "- "+it)
		}
		r = append(r, "")
	}
	section("### ✅ Success Indicators:", oks, 5)
	section("### ❌ Errors Found:", errs, 10)
	section("### ⚠️ Warnings Found:", warns, 5)
	return strings.Join(r, "\n"), nil
}

// Summary is the Markdown of the left column of the pip tabs. For "install" it
// is the summary of all pip logs with the status of the ten newest installs;
// for "uninstall" it is the count line.
func (s *Store) Summary(kind string) (string, error) {
	if _, err := kindPrefix(kind); err != nil {
		return "", err
	}
	installs, err := s.PipLogs("install")
	if err != nil {
		return "", err
	}
	uninstalls, err := s.PipLogs("uninstall")
	if err != nil {
		return "", err
	}
	if kind == "uninstall" {
		return fmt.Sprintf("**Total Uninstall Logs:** %d", len(uninstalls)), nil
	}
	r := []string{
		"# 📦 Pip Logs Summary\n",
		fmt.Sprintf("**Installation Logs:** %d", len(installs)),
		fmt.Sprintf("**Uninstallation Logs:** %d\n", len(uninstalls)),
		"## Recent Installations (Last 10):",
	}
	for _, l := range installs[:min(len(installs), 10)] {
		text, _, found, err := s.readText(l.File)
		if err != nil || !found {
			r = append(r, fmt.Sprintf("\n❓ **%s** - Error reading file", l.Name))
			continue
		}
		low := strings.ToLower(text)
		hasErr := containsAny(low, "error:", "failed building", "exception:", "traceback (most")
		hasOK := containsAny(low, "successfully installed", "requirement already satisfied")
		status := "❓"
		switch {
		case hasErr && !hasOK:
			status = "❌"
		case hasOK || strings.Contains(low, "finished with status 'done'"):
			status = "✅"
		}
		size, mod := s.sizeAndTime(l.File)
		r = append(r, fmt.Sprintf("\n%s **%s** - %.1fKB - %s", status, l.Name, size, mod))
	}
	return strings.Join(r, "\n"), nil
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// sizeAndTime is the size in KB and the "YYYY-MM-DD HH:MM" modification time.
func (s *Store) sizeAndTime(name string) (float64, string) {
	es, _ := s.entries()
	for _, e := range es {
		if e.name == name {
			return float64(e.size) / 1024, e.mtime.Format("2006-01-02 15:04")
		}
	}
	return 0, ""
}
