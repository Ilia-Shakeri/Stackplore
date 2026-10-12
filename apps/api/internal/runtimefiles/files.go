// Package runtimefiles owns the private local directory and keyed-digest file.
package runtimefiles

import (
	"errors"
	"os"
	"runtime"
)

var (
	ErrUnsafeDataDirectory = errors.New("unsafe runtime data directory")
	ErrAuthKeyUnavailable  = errors.New("runtime authentication key unavailable")
)

// SyncDirectory flushes a newly published filename where the platform supports it.
func SyncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
