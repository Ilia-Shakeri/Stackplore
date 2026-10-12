package runtimefiles

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maximumPathBytes = 4096

func prepareDataDirectory(value string) (string, error) {
	return resolveDataDirectory(value, true)
}

// PrepareDataDirectory validates and creates the private runtime directory used
// for the digest-key file while PostgreSQL owns application data.
func PrepareDataDirectory(value string) (string, error) {
	return prepareDataDirectory(value)
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
