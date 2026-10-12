package config

import (
	"errors"
	"testing"
)

func TestDockerSocketOnlyAllowsLoopbackDevelopment(t *testing.T) {
	for _, values := range []map[string]string{
		{dockerEnabledKey: "true", environmentKey: "test"},
		{dockerEnabledKey: "true", httpAddressKey: "0.0.0.0:8080", publicOriginKey: "http://localhost:8080"},
		{dockerEnabledKey: "true", publicOriginKey: "http://stackplore.example"},
		{dockerEnabledKey: "true", environmentKey: "production", databaseURLKey: testProductionDatabaseURL, dataDirectoryKey: productionDataDirectory()},
	} {
		if _, err := Load(getter(values)); !errors.Is(err, ErrInvalid) {
			t.Fatal("non-loopback development socket profile accepted")
		}
	}
	if _, err := Load(getter(map[string]string{dockerEnabledKey: "true", dockerSocketPathKey: sourceAbsolutePath("docker.sock")})); err != nil {
		t.Fatal("loopback development socket profile rejected")
	}
}

func TestPostgreSQLRejectsSingleSlotAndLegacyBusyTimeout(t *testing.T) {
	for _, values := range []map[string]string{
		{databaseMaxConnectionsKey: "1", databaseMinConnectionsKey: "1"},
		{"STACKPLORE_DATABASE_BUSY_TIMEOUT": "5s"},
	} {
		if _, err := Load(getter(values)); !errors.Is(err, ErrInvalid) {
			t.Fatal("unsafe or obsolete persistence setting accepted")
		}
	}
}
