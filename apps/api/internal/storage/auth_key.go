package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	authDigestKeyFilename = ".stackplore.auth-key"
	authDigestKeyBytes    = 32
)

func loadOrCreateAuthDigestKey(dataDirectory string) ([authDigestKeyBytes]byte, string, error) {
	path := filepath.Join(dataDirectory, authDigestKeyFilename)
	exists, err := regularFileStatus(path)
	if err != nil {
		return [authDigestKeyBytes]byte{}, "", ErrAuthKeyUnavailable
	}
	if !exists {
		var generated [authDigestKeyBytes]byte
		if _, err := io.ReadFull(rand.Reader, generated[:]); err != nil {
			return [authDigestKeyBytes]byte{}, "", ErrAuthKeyUnavailable
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return [authDigestKeyBytes]byte{}, "", ErrAuthKeyUnavailable
		}
		writeErr := writeAndSyncAuthKey(file, generated[:])
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(path)
			return [authDigestKeyBytes]byte{}, "", ErrAuthKeyUnavailable
		}
		if err := syncDirectory(dataDirectory); err != nil {
			return [authDigestKeyBytes]byte{}, "", ErrAuthKeyUnavailable
		}
	}

	key, err := readAuthDigestKey(path)
	if err != nil {
		return [authDigestKeyBytes]byte{}, "", err
	}
	digest := sha256.Sum256(key[:])
	return key, "key_" + hex.EncodeToString(digest[:8]), nil
}

func writeAndSyncAuthKey(file *os.File, key []byte) error {
	if file == nil || len(key) != authDigestKeyBytes {
		return ErrAuthKeyUnavailable
	}
	written, err := file.Write(key)
	if err != nil || written != len(key) {
		return ErrAuthKeyUnavailable
	}
	if err := file.Sync(); err != nil {
		return ErrAuthKeyUnavailable
	}
	return nil
}

func readAuthDigestKey(path string) ([authDigestKeyBytes]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [authDigestKeyBytes]byte{}, ErrAuthKeyUnavailable
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if err != nil || pathErr != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() ||
		pathInfo.Size() != authDigestKeyBytes || !privatePermissions(pathInfo.Mode()) ||
		!hasSingleLink(path, pathInfo) || !os.SameFile(openedInfo, pathInfo) {
		return [authDigestKeyBytes]byte{}, ErrAuthKeyUnavailable
	}

	var result [authDigestKeyBytes]byte
	if _, err := io.ReadFull(file, result[:]); err != nil {
		return [authDigestKeyBytes]byte{}, ErrAuthKeyUnavailable
	}
	var extra [1]byte
	if count, err := file.Read(extra[:]); count != 0 || !errors.Is(err, io.EOF) {
		return [authDigestKeyBytes]byte{}, ErrAuthKeyUnavailable
	}
	return result, nil
}
