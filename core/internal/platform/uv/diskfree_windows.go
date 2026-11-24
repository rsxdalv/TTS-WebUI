package uv

import "golang.org/x/sys/windows"

// freeBytes is the free space of the volume that holds dir.
func freeBytes(dir string) (uint64, bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var free, total, totalFree uint64
	if windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree) != nil {
		return 0, false
	}
	return free, true
}
