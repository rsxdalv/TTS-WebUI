//go:build darwin || freebsd || netbsd

package hfcache

import (
	"io/fs"
	"syscall"
)

// accessTime is the last access time of a file (st_atime), or its modification
// time when the platform did not say.
func accessTime(fi fs.FileInfo) float64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return float64(st.Atimespec.Sec) + float64(st.Atimespec.Nsec)/1e9
	}
	return seconds(fi.ModTime())
}
