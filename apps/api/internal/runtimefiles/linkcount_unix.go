//go:build !windows

package runtimefiles

import (
	"os"
	"syscall"
)

func hasSingleLink(_ string, info os.FileInfo) bool {
	status, ok := info.Sys().(*syscall.Stat_t)
	return ok && status.Nlink == 1
}
