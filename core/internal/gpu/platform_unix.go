//go:build !windows

package gpu

import (
	"os"
	"strings"
)

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readSysFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// isDGClass reports whether the PCI class string represents an Intel DG GPU.
// Intel Display Controller class is 0x030000 (VGA) or 0x030200 (3D).
func isDGClass(class string) bool {
	c := strings.ToLower(strings.TrimSpace(class))
	return strings.HasPrefix(c, "0x030200") || strings.HasPrefix(c, "0x030000")
}
