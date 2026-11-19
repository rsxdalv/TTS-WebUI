//go:build windows

package react_ui_backend

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// realNameHidden reports whether the file at full, below base, has a real
// (long) name with a part that starts with a dot. NTFS gives .hidden an 8.3
// alias such as HIDDEN~1, and os.Stat resolves it, so checking the URL alone
// is not enough. On any error the file counts as hidden.
func realNameHidden(base, full string) bool {
	longFull, err := longPathName(full)
	if err != nil {
		return true
	}
	longBase, err := longPathName(base)
	if err != nil {
		return true
	}
	rel, err := filepath.Rel(longBase, longFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// longPathName expands every 8.3 alias in an existing path.
func longPathName(p string) (string, error) {
	in, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, 260)
	for {
		n, err := windows.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil {
			return "", err
		}
		if n <= uint32(len(buf)) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}
