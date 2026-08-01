package observability

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

func TestNewJSONLogger(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("service ready", "request_id", "req_123", "count", 3, "ready", true)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}
	if record["level"] != "INFO" || record["msg"] != "service ready" {
		t.Errorf("built-in fields = (%v, %v); want INFO and message", record["level"], record["msg"])
	}
	if record["request_id"] != "req_123" || record["count"] != float64(3) || record["ready"] != true {
		t.Errorf("structured fields = %v; want preserved values", record)
	}
	if _, exists := record["source"]; exists {
		t.Error("source field present; want host path omitted")
	}
	timestamp, ok := record["time"].(string)
	if !ok || !strings.HasSuffix(timestamp, "Z") {
		t.Errorf("time = %v; want UTC timestamp", record["time"])
	}
}

func TestNewTextLogger(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelDebug, config.LogFormatText)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Debug("probe ready", "request_id", "req_456", "attempt", 2)

	line := output.String()
	for _, fragment := range []string{"level=DEBUG", `msg="probe ready"`, "request_id=req_456", "attempt=2"} {
		if !strings.Contains(line, fragment) {
			t.Errorf("text output missing %q: %q", fragment, line)
		}
	}
	if strings.Count(line, "\n") != 1 {
		t.Errorf("text output has %d lines; want one", strings.Count(line, "\n"))
	}
}

func TestNewValidatesOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		writer *bytes.Buffer
		level  slog.Level
		format config.LogFormat
		want   error
	}{
		{"missing writer", nil, slog.LevelInfo, config.LogFormatJSON, ErrWriterRequired},
		{"low custom level", &bytes.Buffer{}, slog.Level(-8), config.LogFormatJSON, ErrLevelInvalid},
		{"high custom level", &bytes.Buffer{}, slog.Level(12), config.LogFormatJSON, ErrLevelInvalid},
		{"unknown format", &bytes.Buffer{}, slog.LevelInfo, config.LogFormat("console"), ErrFormatInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			logger, err := New(test.writer, test.level, test.format)
			if logger != nil {
				t.Error("New() logger is non-nil on error")
			}
			if !errors.Is(err, test.want) {
				t.Errorf("New() error = %v; want %v", err, test.want)
			}
		})
	}
}

func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		threshold slog.Level
		logged    slog.Level
		want      bool
	}{
		{"debug hidden at info", slog.LevelInfo, slog.LevelDebug, false},
		{"info shown at info", slog.LevelInfo, slog.LevelInfo, true},
		{"warning shown at info", slog.LevelInfo, slog.LevelWarn, true},
		{"debug shown at debug", slog.LevelDebug, slog.LevelDebug, true},
		{"info hidden at warning", slog.LevelWarn, slog.LevelInfo, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			logger, err := New(&output, test.threshold, config.LogFormatJSON)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			logger.Log(t.Context(), test.logged, "checked")
			if got := output.Len() > 0; got != test.want {
				t.Errorf("output present = %v; want %v", got, test.want)
			}
		})
	}
}

func TestSensitiveFieldsAreRedacted(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.With("password", "first-value").Info(
		"safe",
		"api_key", "second-value",
		"authorization", "third-value",
		"accessToken", "camel-value",
		"request_id", "req_safe",
		slog.Group("credential", "value", "fourth-value"),
	)

	line := output.String()
	for _, forbidden := range []string{"first-value", "second-value", "third-value", "fourth-value", "camel-value"} {
		if strings.Contains(line, forbidden) {
			t.Errorf("output contains protected field value %q", forbidden)
		}
	}
	if got := strings.Count(line, redactedValue); got != 5 {
		t.Errorf("redaction count = %d; want 5; output = %q", got, line)
	}
	if !strings.Contains(line, "req_safe") {
		t.Error("safe request identifier was removed")
	}
}

func TestUnsafeValuesDoNotEnterOutput(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("checked", "raw", []byte("byte-value"), "object", struct{ Value string }{"object-value"})

	line := output.String()
	if strings.Contains(line, "byte-value") || strings.Contains(line, "object-value") {
		t.Errorf("output disclosed unsupported value: %q", line)
	}
	if !strings.Contains(line, redactedValue) || !strings.Contains(line, unsupportedValue) {
		t.Errorf("output lacks safe replacements: %q", line)
	}
}

func TestTextIsSingleLineAndBounded(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	longValue := strings.Repeat("x", maximumTextRunes) + "hidden-tail"
	logger.Info("first\nsecond\u202ethird", "detail", "left\rright\tend", "long", longValue)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record["msg"] != "first second third" {
		t.Errorf("msg = %q; want control-safe text", record["msg"])
	}
	if record["detail"] != "left right end" {
		t.Errorf("detail = %q; want control-safe text", record["detail"])
	}
	logged, _ := record["long"].(string)
	if len([]rune(logged)) != maximumTextRunes+3 || !strings.HasSuffix(logged, "...") || strings.Contains(logged, "hidden-tail") {
		t.Errorf("long field was not bounded: rune count = %d", len([]rune(logged)))
	}
}

func TestReservedFieldsCannotShadowBuiltIns(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.With("LEVEL", "bound").Info("real message", "MSG", "shadow", "TIME", "shadow-time", "SOURCE", "shadow-source")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record["level"] != "INFO" || record["msg"] != "real message" {
		t.Errorf("built-ins were shadowed: %v", record)
	}
	for _, key := range []string{"field_level", "field_msg", "field_time", "field_source"} {
		if _, ok := record[key]; !ok {
			t.Errorf("renamed field %q missing: %v", key, record)
		}
	}
}

func TestEmptyGroupKeepsRootFields(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.WithGroup("").Info("checked", "request_id", "req_root")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record["request_id"] != "req_root" {
		t.Errorf("request_id = %v; want root field", record["request_id"])
	}
	if _, exists := record["field"]; exists {
		t.Error("empty group created a field group")
	}
}

func TestTimesUseUTC(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("offset", 3*60*60+30*60)
	observed := time.Date(2026, time.August, 1, 12, 30, 0, 0, zone)
	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("observed", slog.Time("observed_at", observed))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record["observed_at"] != "2026-08-01T09:00:00Z" {
		t.Errorf("observed_at = %v; want UTC", record["observed_at"])
	}
}

func TestErrorsAreControlSafe(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, err := New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Error("failed", "error", errors.New("first\nsecond"))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record["error"] != "first second" {
		t.Errorf("error = %q; want control-safe text", record["error"])
	}
}
