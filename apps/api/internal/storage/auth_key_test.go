package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAuthDigestKeyPersistsAndReturnsCopies(t *testing.T) {
	directory := t.TempDir()
	first, firstID, err := loadOrCreateAuthDigestKey(directory)
	if err != nil {
		t.Fatalf("first key load error = %v", err)
	}
	second, secondID, err := loadOrCreateAuthDigestKey(directory)
	if err != nil {
		t.Fatalf("second key load error = %v", err)
	}
	if firstID == "" || firstID != secondID || !bytes.Equal(first[:], second[:]) {
		t.Fatal("key did not remain stable")
	}

	store := &Store{authDigestKey: first, authDigestKeyID: firstID}
	gotID, gotKey, err := store.AuthDigestKey()
	if err != nil || gotID != firstID || !bytes.Equal(gotKey, first[:]) {
		t.Fatalf("AuthDigestKey() = (%q, %x, %v)", gotID, gotKey, err)
	}
	gotKey[0] ^= 0xff
	_, untouched, err := store.AuthDigestKey()
	if err != nil || bytes.Equal(gotKey, untouched) {
		t.Fatal("caller changed stored key")
	}
}

func TestAuthDigestKeyRejectsUnsafeFiles(t *testing.T) {
	tests := []struct {
		name string
		make func(*testing.T, string)
	}{
		{
			name: "wrong size",
			make: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory",
			make: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name string
			make func(*testing.T, string)
		}{
			name: "public mode",
			make: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, make([]byte, authDigestKeyBytes), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			test.make(t, filepath.Join(directory, authDigestKeyFilename))
			if _, _, err := loadOrCreateAuthDigestKey(directory); err != ErrAuthKeyUnavailable {
				t.Fatalf("error = %v; want ErrAuthKeyUnavailable", err)
			}
		})
	}
}
