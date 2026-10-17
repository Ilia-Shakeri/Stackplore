package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

func TestProviderCredentialFilesAreBoundedRegularAndScoped(t *testing.T) {
	directory := t.TempDir()
	regular := filepath.Join(directory, "ca.pem")
	if os.WriteFile(regular, []byte("public fixture"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	if value, err := readBoundedFile(regular, 32); err != nil || string(value) != "public fixture" {
		t.Fatal("regular credential file rejected")
	}
	if _, err := readBoundedFile(regular, 2); !errors.Is(err, errProductStart) {
		t.Fatal("oversize credential file accepted")
	}
	if _, err := readBoundedFile(directory, 32); !errors.Is(err, errProductStart) {
		t.Fatal("directory accepted as credential file")
	}
	projected := filepath.Join(directory, "..version")
	if os.Mkdir(projected, 0700) != nil {
		t.Fatal("fixture directory failed")
	}
	if os.WriteFile(filepath.Join(projected, "token"), []byte("fixture credential reference"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	link := filepath.Join(directory, "token")
	if err := os.Symlink(filepath.Join(projected, "token"), link); err != nil {
		t.Skip("host does not permit test symlink creation")
	}
	if _, err := readBoundedFile(link, 64); err != nil {
		t.Fatal("projected credential link within mount rejected")
	}
	outside := filepath.Join(t.TempDir(), "outside.pem")
	if os.WriteFile(outside, []byte("public fixture"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	escape := filepath.Join(directory, "escape.pem")
	if os.Symlink(outside, escape) != nil {
		t.Fatal("fixture symlink failed")
	}
	if _, err := readBoundedFile(escape, 32); !errors.Is(err, errProductStart) {
		t.Fatal("credential link escaped selected mount")
	}
}

func TestDockerTLSReferencesFailClosedWithoutDisclosingInput(t *testing.T) {
	settings := defaultSettings(t)
	settings.DockerAccessProfile = config.DockerProfileRestrictedProxy
	settings.DockerProxyOrigin = "https://docker-proxy.example:8443"
	directory := t.TempDir()
	settings.DockerProxyCAFile = filepath.Join(directory, "ca.pem")
	settings.DockerProxyCertFile = filepath.Join(directory, "client.pem")
	settings.DockerProxyKeyFile = filepath.Join(directory, "client-key.pem")
	if os.WriteFile(settings.DockerProxyCAFile, []byte("untrusted fixture input"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	if transport, _, err := dockerProviderTransport(settings); transport != nil || !errors.Is(err, errProductStart) || err.Error() != errProductStart.Error() {
		t.Fatal("malformed credential reference did not fail safely")
	}
}
