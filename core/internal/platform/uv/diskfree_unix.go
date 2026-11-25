//go:build !windows

package uv

import "golang.org/x/sys/unix"

// freeBytes is the free space of the volume that holds dir.
func freeBytes(dir string) (uint64, bool) {
	var s unix.Statfs_t
	if unix.Statfs(dir, &s) != nil {
		return 0, false
	}
	return uint64(s.Bavail) * uint64(s.Bsize), true
}
