package runtimefiles

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrivateDirectoryAndDigestKeyRemainStable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime")
	directory, err := PrepareDataDirectory(root)
	if err != nil || directory != root {
		t.Fatal("private runtime directory rejected")
	}
	first, firstID, err := LoadOrCreateAuthDigestKey(directory)
	if err != nil {
		t.Fatal("initial digest key unavailable")
	}
	second, secondID, err := LoadOrCreateAuthDigestKey(directory)
	if err != nil || firstID == "" || firstID != secondID || !bytes.Equal(first[:], second[:]) {
		t.Fatal("digest key did not persist")
	}
	clear(first[:])
	third, _, err := LoadOrCreateAuthDigestKey(directory)
	if err != nil || !bytes.Equal(second[:], third[:]) {
		t.Fatal("caller modified persisted digest key")
	}
	clear(second[:])
	clear(third[:])
}

func TestDigestKeyRejectsUnsafeFileAndDirectory(t *testing.T) {
	for _, name := range []string{"short", "directory", "public"} {
		t.Run(name, func(t *testing.T) {
			if name == "public" && runtime.GOOS == "windows" {
				t.Skip("Windows access uses filesystem ACLs")
			}
			root := t.TempDir()
			path := filepath.Join(root, authDigestKeyFilename)
			var err error
			switch name {
			case "short":
				err = os.WriteFile(path, []byte("short"), 0o600)
			case "directory":
				err = os.Mkdir(path, 0o700)
			case "public":
				err = os.WriteFile(path, make([]byte, 32), 0o644)
			}
			if err != nil {
				t.Fatal("unsafe fixture could not be made")
			}
			if _, _, err := LoadOrCreateAuthDigestKey(root); !errors.Is(err, ErrAuthKeyUnavailable) {
				t.Fatal("unsafe digest file accepted")
			}
		})
	}
	for _, path := range []string{"", ".", filepath.VolumeName(t.TempDir()) + string(os.PathSeparator), "../escape"} {
		if _, err := PrepareDataDirectory(path); !errors.Is(err, ErrUnsafeDataDirectory) {
			t.Fatal("unsafe runtime directory accepted")
		}
	}
}
