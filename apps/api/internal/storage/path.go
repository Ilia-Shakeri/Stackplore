package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofrs/flock"
)

const (
	databaseFilename        = "stackplore.db"
	ownerLockFilename       = ".stackplore.owner.lock"
	maintenanceLockFilename = ".stackplore.maintenance.lock"
	maximumPathBytes        = 4096
)

var databaseSidecarSuffixes = [...]string{"-wal", "-shm", "-journal"}

func prepareDataDirectory(value string) (string, error) {
	return resolveDataDirectory(value, true)
}

func existingDataDirectory(value string) (string, error) {
	return resolveDataDirectory(value, false)
}

func resolveDataDirectory(value string, create bool) (string, error) {
	if !validDataDirectoryValue(value) {
		return "", ErrUnsafeDataDirectory
	}

	resolved, err := filepath.Abs(value)
	if err != nil || isFilesystemRoot(resolved) || hasRemoteOrDevicePrefix(resolved) {
		return "", ErrUnsafeDataDirectory
	}
	resolved = filepath.Clean(resolved)
	if err := validateSecureDirectory(resolved, create); err != nil {
		return "", ErrUnsafeDataDirectory
	}
	return resolved, nil
}

func validDataDirectoryValue(value string) bool {
	if value == "" || len(value) > maximumPathBytes || !utf8.ValidString(value) {
		return false
	}
	if filepath.Clean(value) != value || value == "." || isFilesystemRoot(value) || hasRemoteOrDevicePrefix(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return filepath.IsAbs(value) || filepath.IsLocal(value)
}

func isFilesystemRoot(value string) bool {
	cleaned := filepath.Clean(value)
	return filepath.Dir(cleaned) == cleaned
}

func hasRemoteOrDevicePrefix(value string) bool {
	slashed := filepath.ToSlash(value)
	return strings.HasPrefix(slashed, "//")
}

func validateSecureDirectory(target string, create bool) error {
	volume := filepath.VolumeName(target)
	root := volume + string(os.PathSeparator)
	if volume == "" {
		root = string(os.PathSeparator)
	}
	remainder := strings.TrimPrefix(target, root)
	current := root
	parts := strings.Split(remainder, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if !create {
				return ErrUnsafeDataDirectory
			}
			if err := os.Mkdir(current, 0o700); err != nil {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return ErrUnsafeDataDirectory
		}
		if index == len(parts)-1 && !privatePermissions(info.Mode()) {
			return ErrUnsafeDataDirectory
		}
	}
	return nil
}

func privatePermissions(mode os.FileMode) bool {
	return runtime.GOOS == "windows" || mode.Perm()&0o077 == 0
}

func acquireOwnerLock(dataDirectory string) (*flock.Flock, error) {
	return acquireFileLock(filepath.Join(dataDirectory, ownerLockFilename), true)
}

func acquireMaintenanceLock(dataDirectory string, exclusive bool) (*flock.Flock, error) {
	return acquireFileLock(filepath.Join(dataDirectory, maintenanceLockFilename), exclusive)
}

func acquireFileLock(path string, exclusive bool) (*flock.Flock, error) {
	if err := validateOptionalRegularFile(path); err != nil {
		return nil, ErrOwnershipUnavailable
	}

	lock := flock.New(
		path,
		flock.SetFlag(os.O_CREATE|os.O_RDWR),
		flock.SetPermissions(0o600),
	)
	var locked bool
	var err error
	if exclusive {
		locked, err = lock.TryLock()
	} else {
		locked, err = lock.TryRLock()
	}
	if err != nil || !locked {
		_ = lock.Close()
		return nil, ErrOwnershipUnavailable
	}

	openedInfo, err := lock.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if err != nil || pathErr != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() ||
		!privatePermissions(pathInfo.Mode()) || !os.SameFile(openedInfo, pathInfo) {
		_ = lock.Close()
		return nil, ErrOwnershipUnavailable
	}
	return lock, nil
}

func prepareDatabaseFile(dataDirectory string) (string, error) {
	path := filepath.Join(dataDirectory, databaseFilename)
	databaseExists, err := regularFileStatus(path)
	if err != nil {
		return "", ErrUnsafeDataDirectory
	}

	sidecarExists := false
	for _, suffix := range databaseSidecarSuffixes {
		exists, err := regularFileStatus(path + suffix)
		if err != nil {
			return "", ErrUnsafeDataDirectory
		}
		sidecarExists = sidecarExists || exists
	}
	if !databaseExists && sidecarExists {
		return "", ErrUnsafeDataDirectory
	}
	if databaseExists {
		return path, nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return "", ErrDatabaseUnavailable
	}
	if err := file.Close(); err != nil {
		return "", ErrDatabaseUnavailable
	}
	if _, err := regularFileStatus(path); err != nil {
		return "", ErrUnsafeDataDirectory
	}
	return path, nil
}

func existingDatabaseFile(dataDirectory string) (string, error) {
	path := filepath.Join(dataDirectory, databaseFilename)
	exists, err := regularFileStatus(path)
	if err != nil || !exists {
		return "", ErrDatabaseUnavailable
	}
	for _, suffix := range databaseSidecarSuffixes {
		if _, err := regularFileStatus(path + suffix); err != nil {
			return "", ErrUnsafeDataDirectory
		}
	}
	return path, nil
}

func validateOptionalRegularFile(path string) error {
	_, err := regularFileStatus(path)
	return err
}

func regularFileStatus(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !privatePermissions(info.Mode()) ||
		!hasSingleLink(path, info) {
		return false, ErrUnsafeDataDirectory
	}
	return true, nil
}
