package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
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

func TestParseCommandAcceptsOnlyNamedAccountFields(t *testing.T) {
	for _, arguments := range [][]string{
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", "Primary Operator"},
		{commandRecoverAdministrator, "--display-name", "Primary Operator", "--login-name", "operator"},
	} {
		parsed, err := parseCommand(arguments)
		if err != nil {
			t.Fatalf("parseCommand() error = %v", err)
		}
		if parsed.loginName != "operator" || parsed.displayName != "Primary Operator" {
			t.Fatalf("parseCommand() = %#v", parsed)
		}
	}
}

func TestParseCommandRejectsLooseOrSecretArguments(t *testing.T) {
	tests := [][]string{
		nil,
		{commandRecoverAdministrator},
		{"Recover-Admin", "--login-name", "operator", "--display-name", "Primary Operator"},
		{commandRecoverAdministrator, "--login-name=operator", "--display-name", "Primary Operator", "extra"},
		{commandRecoverAdministrator, "--login-name", "operator", "--password", "never-put-password-here"},
		{commandRecoverAdministrator, "--login-name", "operator", "--login-name", "operator"},
		{commandRecoverAdministrator, "--login-name", "bad name", "--display-name", "Primary Operator"},
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", " Primary Operator"},
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", "Primary\x00Operator"},
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", "\u200c"},
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", strings.Repeat("x", auth.MaximumDisplayRunes+1)},
		{commandRecoverAdministrator, "--login-name", "operator", "--display-name", "Primary Operator", "password"},
	}
	for index, arguments := range tests {
		if _, err := parseCommand(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("case %d: parseCommand() error = %v", index, err)
		}
	}
}

func TestReadPasswordAcceptsOneBoundedExactLine(t *testing.T) {
	minimum := strings.Repeat("m", auth.MinimumPasswordBytes)
	maximum := strings.Repeat("x", auth.MaximumPasswordBytes)
	for name, input := range map[string]string{
		"no terminator": minimum,
		"line feed":     minimum + "\n",
		"windows line":  maximum + "\r\n",
		"keeps spaces":  " leading and trailing ",
	} {
		t.Run(name, func(t *testing.T) {
			password, err := readPassword(strings.NewReader(input))
			if err != nil {
				t.Fatalf("readPassword() error = %v", err)
			}
			defer clear(password)
			want := strings.TrimSuffix(strings.TrimSuffix(input, "\n"), "\r")
			if string(password) != want {
				t.Fatalf("readPassword() = %q, want exact %q", password, want)
			}
		})
	}
}

func TestReadPasswordRejectsOverflowExtraLinesAndReaderFailure(t *testing.T) {
	valid := strings.Repeat("v", auth.MinimumPasswordBytes)
	inputs := []string{
		"",
		"too-short\n",
		strings.Repeat("x", auth.MaximumPasswordBytes+1),
		strings.Repeat("x", auth.MaximumPasswordBytes) + "\r\nextra",
		valid + "\n" + valid + "\n",
		valid + "\rhidden",
	}
	for index, input := range inputs {
		if password, err := readPassword(strings.NewReader(input)); !errors.Is(err, errInvalidPassword) || password != nil {
			t.Errorf("case %d: readPassword() = %q, %v", index, password, err)
		}
	}
	if password, err := readPassword(failingReader{}); !errors.Is(err, errInvalidPassword) || password != nil {
		t.Fatalf("readPassword(failure) = %q, %v", password, err)
	}
}

func TestRunCommandPassesOnlyValidatedRecoveryRequest(t *testing.T) {
	settings := testSettings(t)
	request := recoveryRequest{
		dataDirectory: settings.DataDirectory,
		queryTimeout:  settings.DatabaseQueryTimeout,
		loginName:     "operator",
		displayName:   "Primary Operator",
		password:      []byte("correct horse value"),
		requestID:     "request_operator_recovery",
	}
	defer clear(request.password)
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")
	called := false
	err := runCommand(ctx, settings, request, func(gotContext context.Context, gotSettings config.Config, got recoveryRequest) error {
		called = true
		if gotContext != ctx || gotSettings.DatabaseURL.Reveal() != settings.DatabaseURL.Reveal() ||
			got.loginName != request.loginName || got.displayName != request.displayName ||
			got.dataDirectory != request.dataDirectory || got.queryTimeout != request.queryTimeout ||
			got.requestID != request.requestID || !bytes.Equal(got.password, request.password) {
			t.Fatalf("recovery request changed: %#v", got)
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("runCommand() error = %v, called = %t", err, called)
	}
}

func TestRunCommandFailsClosedBeforeRecovery(t *testing.T) {
	settings := testSettings(t)
	base := recoveryRequest{
		dataDirectory: settings.DataDirectory,
		queryTimeout:  settings.DatabaseQueryTimeout,
		loginName:     "operator",
		displayName:   "Primary Operator",
		password:      []byte("correct horse value"),
		requestID:     "request_operator_recovery",
	}
	defer clear(base.password)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name    string
		ctx     context.Context
		change  func(*recoveryRequest)
		recover recoveryOperation
	}{
		{"nil context", nil, func(*recoveryRequest) {}, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"canceled", canceled, func(*recoveryRequest) {}, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"nil operation", context.Background(), func(*recoveryRequest) {}, nil},
		{"wrong directory", context.Background(), func(value *recoveryRequest) { value.dataDirectory += "-other" }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"wrong timeout", context.Background(), func(value *recoveryRequest) { value.queryTimeout++ }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"invalid login", context.Background(), func(value *recoveryRequest) { value.loginName = "bad name" }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"invalid display", context.Background(), func(value *recoveryRequest) { value.displayName = " hidden" }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"invalid password", context.Background(), func(value *recoveryRequest) { value.password = []byte("short") }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
		{"invalid request", context.Background(), func(value *recoveryRequest) { value.requestID = "bad request" }, func(context.Context, config.Config, recoveryRequest) error { return nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			test.change(&request)
			called := false
			operation := test.recover
			if operation != nil {
				operation = func(context.Context, config.Config, recoveryRequest) error {
					called = true
					return nil
				}
			}
			if err := runCommand(test.ctx, settings, request, operation); err == nil {
				t.Fatal("runCommand() error = nil")
			}
			if called {
				t.Fatal("recovery ran across failed boundary")
			}
		})
	}
}

func TestRunMainUsesSealedFailureOutput(t *testing.T) {
	const marker = "do-not-disclose-secret"
	var standardOutput bytes.Buffer
	var standardError bytes.Buffer
	exitCode := runMain(
		[]string{commandRecoverAdministrator, "--login-name", "secrethandle", "--display-name", marker},
		strings.NewReader(marker+"-password\n"),
		&standardOutput,
		&standardError,
		mapGetter{},
		backgroundContext,
		func(context.Context, config.Config, recoveryRequest) error { return errors.New(marker) },
		bytes.NewReader(make([]byte, requestRandomBytes)),
	)
	if exitCode != exitFailure {
		t.Fatalf("runMain() = %d", exitCode)
	}
	combined := standardOutput.String() + standardError.String()
	if strings.Contains(combined, marker) || strings.Contains(combined, "secrethandle") {
		t.Fatalf("output disclosed input: %s", combined)
	}
	if !strings.Contains(standardError.String(), "operation_failed") || standardOutput.Len() != 0 {
		t.Fatalf("sealed failure output = %q / %q", standardOutput.String(), standardError.String())
	}
}

func TestRunMainClearsPasswordAndWritesFixedSuccess(t *testing.T) {
	passwordText := "correct horse value"
	var standardOutput bytes.Buffer
	var standardError bytes.Buffer
	var passedPassword []byte
	stopped := false
	exitCode := runMain(
		[]string{commandRecoverAdministrator, "--login-name", "operator", "--display-name", "Primary Operator"},
		strings.NewReader(passwordText+"\n"),
		&standardOutput,
		&standardError,
		mapGetter{},
		func() (context.Context, context.CancelFunc) {
			return context.Background(), func() { stopped = true }
		},
		func(_ context.Context, _ config.Config, request recoveryRequest) error {
			if string(request.password) != passwordText || request.requestID.Validate() != nil {
				t.Fatalf("recovery input invalid")
			}
			passedPassword = request.password
			return nil
		},
		bytes.NewReader(make([]byte, requestRandomBytes)),
	)
	if exitCode != exitSuccess || !stopped || standardOutput.String() != successOutput {
		t.Fatalf("runMain() = %d, stopped = %t, output = %q", exitCode, stopped, standardOutput.String())
	}
	for index, value := range passedPassword {
		if value != 0 {
			t.Fatalf("password byte %d was not cleared", index)
		}
	}
	if strings.Contains(standardError.String(), passwordText) || strings.Contains(standardError.String(), "operator_") {
		t.Fatalf("success log disclosed protected input: %s", standardError.String())
	}
}

func TestFailureReasonMapsPostgreSQLBoundaries(t *testing.T) {
	tests := map[error]string{
		postgres.ErrInvalidOptions:       "invalid_options",
		postgres.ErrOwnershipUnavailable: "ownership_unavailable",
		postgres.ErrDatabaseUnavailable:  "database_unavailable",
		postgres.ErrMigrationRejected:    "migration_rejected",
		postgres.ErrStoreClosed:          "storage_close",
	}
	for input, want := range tests {
		if got := failureReason(input); got != want {
			t.Errorf("failureReason(%v) = %q, want %q", input, got, want)
		}
	}
}

func testSettings(t *testing.T) config.Config {
	t.Helper()
	settings, err := config.Load(mapGetter{})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return settings
}

func backgroundContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
