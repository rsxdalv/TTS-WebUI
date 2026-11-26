//go:build windows

package hfcache

import (
	"io/fs"
	"syscall"
)

// accessTime is the last access time of a file, or its modification time when
// the platform did not say.
func accessTime(fi fs.FileInfo) float64 {
	if d, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return seconds(timeFromFiletime(d.LastAccessTime))
	}
	return seconds(fi.ModTime())
}
