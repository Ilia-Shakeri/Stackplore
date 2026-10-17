package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDemoDataDirectoryCannotReuseNormalOrUnmarkedRuntimeKeys(t *testing.T) {
	normal := filepath.Join(t.TempDir(), "normal")
	if _, err := prepareProfileDirectory(normal, false); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileDirectory(normal, true); err == nil {
		t.Fatal("demo accepted normal key directory")
	}
	old := filepath.Join(t.TempDir(), "old-normal")
	if err := os.Mkdir(old, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "unmarked-key-placeholder"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileDirectory(old, true); err == nil {
		t.Fatal("demo accepted unmarked nonempty directory")
	}
	demo := filepath.Join(t.TempDir(), "demo")
	if _, err := prepareProfileDirectory(demo, true); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileDirectory(demo, true); err != nil {
		t.Fatal("demo directory could not reopen")
	}
	if _, err := prepareProfileDirectory(demo, false); err == nil {
		t.Fatal("normal process reused demo key directory")
	}
}
