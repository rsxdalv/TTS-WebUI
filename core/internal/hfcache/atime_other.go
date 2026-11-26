//go:build !linux && !darwin && !freebsd && !netbsd && !windows

package hfcache

import "io/fs"

// accessTime falls back to the modification time where st_atime is not read.
func accessTime(fi fs.FileInfo) float64 { return seconds(fi.ModTime()) }
