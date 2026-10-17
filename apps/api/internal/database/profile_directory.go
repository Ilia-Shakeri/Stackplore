package database

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runtimefiles"
)

func prepareProfileDirectory(path string, demo bool) (string, error) {
	directory, err := runtimefiles.PrepareDataDirectory(path)
	if err != nil {
		return "", err
	}
	mode := "normal\n"
	if demo {
		mode = "demo\n"
	}
	marker := filepath.Join(directory, "deployment-profile")
	info, err := os.Lstat(marker)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() > 16 {
			return "", postgres.ErrInvalidOptions
		}
		file, err := os.Open(marker)
		if err != nil {
			return "", postgres.ErrInvalidOptions
		}
		content, readErr := io.ReadAll(io.LimitReader(file, 17))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || string(content) != mode {
			return "", postgres.ErrInvalidOptions
		}
		return directory, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", postgres.ErrInvalidOptions
	}
	if demo {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			return "", postgres.ErrInvalidOptions
		}
	}
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", postgres.ErrInvalidOptions
	}
	_, writeErr := file.WriteString(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return "", postgres.ErrInvalidOptions
	}
	return directory, nil
}
