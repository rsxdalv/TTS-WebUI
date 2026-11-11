//go:build windows

package gpu

import "os"

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readSysFile(_ string) string { return "" }

// isDGClass is unused on Windows (no sysfs).
func isDGClass(_ string) bool { return false }
