package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/storage"
)

type mapGetter map[string]string

func (values mapGetter) LookupEnv(key string) (string, bool) {
	value, ok := values[key]
	return value, ok
}

func (values mapGetter) Environ() []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func absoluteTestFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func testSettings(t *testing.T) config.Config {
	t.Helper()
	settings, err := config.Load(mapGetter{})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return settings
}

func TestParseCommandAcceptsExactForms(t *testing.T) {
	destination := absoluteTestFile(t, "backup.db")
	source := absoluteTestFile(t, "restore.db")

	tests := []struct {
		arguments []string
		want      command
	}{
		{[]string{"backup", "--destination", destination}, command{name: commandBackup, path: destination}},
		{[]string{"restore", "--source", source}, command{name: commandRestore, path: source}},
	}
	for _, test := range tests {
		got, err := parseCommand(test.arguments)
		if err != nil {
			t.Fatalf("parseCommand(%q) error = %v", test.arguments[0], err)
		}
		if got != test.want {
			t.Errorf("parseCommand(%q) = %#v; want %#v", test.arguments[0], got, test.want)
		}
	}
}

func TestParseCommandRejectsLooseArguments(t *testing.T) {
	absolute := absoluteTestFile(t, "archive.db")
	nonCanonical := filepath.Dir(absolute) + string(filepath.Separator) + "part" + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(absolute)
	root := filepath.VolumeName(absolute) + string(filepath.Separator)
	if filepath.VolumeName(absolute) == "" {
		root = string(filepath.Separator)
	}

	tests := [][]string{
		nil,
		{"backup"},
		{"snapshot", "--destination", absolute},
		{"Backup", "--destination", absolute},
		{"backup", "--source", absolute},
		{"restore", "--destination", absolute},
		{"backup", "--destination=" + absolute},
		{"backup", "--destination", absolute, "extra"},
		{"backup", "--destination", "relative.db"},
		{"backup", "--destination", absolute + " "},
		{"backup", "--destination", nonCanonical},
		{"backup", "--destination", root},
		{"backup", "--destination", absolute + "\x00tail"},
		{"backup", "--destination", absolute + "\u0001tail"},
		{"backup", "--destination", string([]byte{filepath.Separator, 'x', 0xff})},
		{"backup", "--destination", string(filepath.Separator) + strings.Repeat("x", maximumCommandPathBytes)},
		{"backup", "--destination", `\\server\share\backup.db`},
		{"backup", "--destination", filepath.Join(filepath.Dir(absolute), "NUL")},
		{"backup", "--destination", filepath.Join(filepath.Dir(absolute), "COM\u00b9.db")},
		{"restore", "--source", ""},
	}
	for index, arguments := range tests {
		if _, err := parseCommand(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("case %d: parseCommand() error = %v; want errInvalidArguments", index, err)
		}
	}
}

func TestRunCommandPassesBoundedOptions(t *testing.T) {
	settings := testSettings(t)
	destination := absoluteTestFile(t, "backup.db")
	source := absoluteTestFile(t, "restore.db")
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")

	var gotBackup storage.BackupOptions
	var gotRestore storage.RestoreOptions
	operations := storageOperations{
		backup: func(gotContext context.Context, options storage.BackupOptions) error {
			if gotContext != ctx {
				t.Error("backup context changed")
			}
			gotBackup = options
			return nil
		},
		restore: func(gotContext context.Context, options storage.RestoreOptions) error {
			if gotContext != ctx {
				t.Error("restore context changed")
			}
			gotRestore = options
			return nil
		},
	}

	if err := runCommand(ctx, settings, command{name: commandBackup, path: destination}, operations); err != nil {
		t.Fatalf("backup runCommand() error = %v", err)
	}
	if gotBackup.DataDirectory != settings.DataDirectory ||
		gotBackup.BusyTimeout != settings.DatabaseBusyTimeout ||
		gotBackup.Destination != destination {
		t.Errorf("backup options = %#v; want exact settings and destination", gotBackup)
	}

	if err := runCommand(ctx, settings, command{name: commandRestore, path: source}, operations); err != nil {
		t.Fatalf("restore runCommand() error = %v", err)
	}
	if gotRestore.DataDirectory != settings.DataDirectory ||
		gotRestore.BusyTimeout != settings.DatabaseBusyTimeout ||
		gotRestore.Source != source {
		t.Errorf("restore options = %#v; want exact settings and source", gotRestore)
	}
}

func TestRunCommandFailsClosed(t *testing.T) {
	settings := testSettings(t)
	path := absoluteTestFile(t, "archive.db")
	called := false
	operations := storageOperations{
		backup: func(context.Context, storage.BackupOptions) error {
			called = true
			return nil
		},
		restore: func(context.Context, storage.RestoreOptions) error {
			called = true
			return nil
		},
	}

	tests := []struct {
		name    string
		ctx     context.Context
		change  func(*config.Config)
		command command
	}{
		{"nil context", nil, func(*config.Config) {}, command{name: commandBackup, path: path}},
		{"canceled context", canceledContext(), func(*config.Config) {}, command{name: commandBackup, path: path}},
		{"forged config", context.Background(), func(value *config.Config) { value.DatabaseBusyTimeout = 0 }, command{name: commandBackup, path: path}},
		{"unknown command", context.Background(), func(*config.Config) {}, command{name: commandName("unknown"), path: path}},
		{"relative path", context.Background(), func(*config.Config) {}, command{name: commandBackup, path: "archive.db"}},
	}
	for _, test := range tests {
		value := settings
		test.change(&value)
		if err := runCommand(test.ctx, value, test.command, operations); err == nil {
			t.Errorf("%s: runCommand() error = nil", test.name)
		}
	}
	if called {
		t.Fatal("operation ran across failed boundary")
	}
}

func TestRunMainUsesSealedLogs(t *testing.T) {
	const marker = "do-not-disclose-12d0b7"
	path := absoluteTestFile(t, marker+".db")
	tests := []struct {
		name       string
		arguments  []string
		getter     config.Getter
		operations storageOperations
		wantExit   int
		wantReason string
	}{
		{
			name:       "arguments",
			arguments:  []string{"backup", "--destination", marker},
			getter:     mapGetter{},
			wantExit:   exitUsage,
			wantReason: "invalid_arguments",
		},
		{
			name:      "configuration",
			arguments: []string{"backup", "--destination", path},
			getter:    mapGetter{"STACKPLORE_HTTP_ADDRESS": marker},
			wantExit:  exitFailure, wantReason: "invalid_configuration",
		},
		{
			name:      "operation",
			arguments: []string{"backup", "--destination", path},
			getter:    mapGetter{},
			operations: storageOperations{backup: func(context.Context, storage.BackupOptions) error {
				return errors.New(marker)
			}},
			wantExit: exitFailure, wantReason: "operation_failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			exitCode := runMain(test.arguments, &output, test.getter, backgroundContext, test.operations)
			if exitCode != test.wantExit {
				t.Errorf("runMain() = %d; want %d", exitCode, test.wantExit)
			}
			if strings.Contains(output.String(), marker) {
				t.Fatalf("log disclosed input: %s", output.String())
			}
			if !strings.Contains(output.String(), test.wantReason) {
				t.Errorf("log lacks reason %q: %s", test.wantReason, output.String())
			}
		})
	}
}

func TestRunMainCompletesWithoutStandardOutput(t *testing.T) {
	path := absoluteTestFile(t, "backup.db")
	var output bytes.Buffer
	called := false
	stopped := false
	exitCode := runMain(
		[]string{"backup", "--destination", path},
		&output,
		mapGetter{},
		func() (context.Context, context.CancelFunc) {
			return context.Background(), func() { stopped = true }
		},
		storageOperations{backup: func(context.Context, storage.BackupOptions) error {
			called = true
			return nil
		}},
	)
	if exitCode != exitSuccess || !called || !stopped {
		t.Fatalf("runMain() = %d, called = %t, stopped = %t; want success and cleanup", exitCode, called, stopped)
	}
	if !strings.Contains(output.String(), "storage command complete") {
		t.Errorf("completion log missing: %s", output.String())
	}
}

func TestFailureReasonIsClosed(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{context.Canceled, "canceled"},
		{context.DeadlineExceeded, "canceled"},
		{errCommandBoundary, "invalid_options"},
		{storage.ErrInvalidOptions, "invalid_options"},
		{storage.ErrUnsafeDataDirectory, "unsafe_data_directory"},
		{storage.ErrOwnershipUnavailable, "ownership_unavailable"},
		{storage.ErrDatabaseUnavailable, "database_unavailable"},
		{storage.ErrDatabaseIntegrity, "database_integrity"},
		{storage.ErrMigrationRejected, "migration_rejected"},
		{storage.ErrBackupRejected, "backup_rejected"},
		{storage.ErrRestoreRejected, "restore_rejected"},
		{errors.New("do-not-disclose"), "operation_failed"},
	}
	for _, test := range tests {
		if got := failureReason(test.err); got != test.want {
			t.Errorf("failureReason(%v) = %q; want %q", test.err, got, test.want)
		}
	}
}

func backgroundContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestProcessContextStartsLive(t *testing.T) {
	ctx, stop := processContext()
	defer stop()
	select {
	case <-ctx.Done():
		t.Fatal("process context started canceled")
	case <-time.After(time.Millisecond):
	}
}
