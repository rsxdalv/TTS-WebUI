//go:build windows

package hfcache

import (
	"syscall"
	"time"
)

func timeFromFiletime(ft syscall.Filetime) time.Time { return time.Unix(0, ft.Nanoseconds()) }
