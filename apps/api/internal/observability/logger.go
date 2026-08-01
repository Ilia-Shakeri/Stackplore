// Package observability builds the process logger.
package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"unicode"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

const (
	redactedValue    = "[redacted]"
	unsupportedValue = "[unsupported]"
	maximumTextRunes = 4096
)

var (
	// ErrWriterRequired marks a missing log destination.
	ErrWriterRequired = errors.New("log writer is required")
	// ErrLevelInvalid marks a level outside the configured set.
	ErrLevelInvalid = errors.New("log level is invalid")
	// ErrFormatInvalid marks an unsupported output format.
	ErrFormatInvalid = errors.New("log format is invalid")
)

// New creates a structured logger with stable fields and safe value handling.
func New(writer io.Writer, level slog.Level, format config.LogFormat) (*slog.Logger, error) {
	if nilLike(writer) {
		return nil, ErrWriterRequired
	}
	if !validLevel(level) {
		return nil, ErrLevelInvalid
	}

	options := &slog.HandlerOptions{
		AddSource:   false,
		Level:       level,
		ReplaceAttr: replaceAttribute,
	}

	var handler slog.Handler
	switch format {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(writer, options)
	case config.LogFormatText:
		handler = slog.NewTextHandler(writer, options)
	default:
		return nil, ErrFormatInvalid
	}

	return slog.New(&safeHandler{next: handler}), nil
}

type safeHandler struct {
	next slog.Handler
}

func (h *safeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *safeHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attribute slog.Attr) bool {
		clean.AddAttrs(renameReserved(attribute))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *safeHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attributes))
	for index, attribute := range attributes {
		clean[index] = renameReserved(attribute)
	}
	return &safeHandler{next: h.next.WithAttrs(clean)}
}

func (h *safeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &safeHandler{next: h.next.WithGroup(safeKey(name))}
}

func validLevel(level slog.Level) bool {
	switch level {
	case slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError:
		return true
	default:
		return false
	}
}

func renameReserved(attribute slog.Attr) slog.Attr {
	attribute.Key = safeKey(attribute.Key)
	switch attribute.Key {
	case slog.TimeKey, slog.LevelKey, slog.MessageKey, slog.SourceKey:
		attribute.Key = "field_" + attribute.Key
	}
	return attribute
}

func replaceAttribute(groups []string, attribute slog.Attr) slog.Attr {
	if attribute.Equal(slog.Attr{}) {
		return attribute
	}
	if isSensitive(groups, attribute.Key) {
		return slog.String(attribute.Key, redactedValue)
	}

	switch attribute.Value.Kind() {
	case slog.KindString:
		attribute.Value = slog.StringValue(safeText(attribute.Value.String()))
	case slog.KindTime:
		attribute.Value = slog.TimeValue(attribute.Value.Time().UTC())
	case slog.KindAny:
		attribute.Value = safeAny(attribute.Value.Any())
	}
	return attribute
}

func safeAny(value any) slog.Value {
	switch typed := value.(type) {
	case error:
		return safeError(typed)
	case slog.Level:
		return slog.StringValue(typed.String())
	case []byte:
		return slog.StringValue(redactedValue)
	default:
		return slog.StringValue(unsupportedValue)
	}
}

func safeError(value error) (result slog.Value) {
	result = slog.StringValue(unsupportedValue)
	if nilLike(value) {
		return result
	}
	defer func() {
		if recover() != nil {
			result = slog.StringValue(unsupportedValue)
		}
	}()
	return slog.StringValue(safeText(value.Error()))
}

func nilLike(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func safeText(value string) string {
	var result strings.Builder
	count := 0
	truncated := false
	for _, character := range value {
		if count == maximumTextRunes {
			truncated = true
			break
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			result.WriteByte(' ')
		} else {
			result.WriteRune(character)
		}
		count++
	}
	if truncated {
		result.WriteString("...")
	}
	return result.String()
}

func safeKey(value string) string {
	if value == "" {
		return "field"
	}
	var result strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			result.WriteRune(character)
		case character >= 'A' && character <= 'Z':
			result.WriteRune(character + ('a' - 'A'))
		case character >= '0' && character <= '9':
			result.WriteRune(character)
		case character == '_' || character == '.' || character == '-':
			result.WriteRune(character)
		default:
			result.WriteByte('_')
		}
		if result.Len() == 128 {
			break
		}
	}
	if result.Len() == 0 {
		return "field"
	}
	return result.String()
}

func isSensitive(groups []string, key string) bool {
	for _, group := range groups {
		if sensitiveName(group) {
			return true
		}
	}
	return sensitiveName(key)
}

func sensitiveName(value string) bool {
	lower := strings.ToLower(value)
	compact := strings.NewReplacer("_", "", "-", "", ".", "").Replace(lower)
	for _, protected := range []string{
		"apikey", "authorization", "cookie", "credential", "csrf", "kubeconfig",
		"passwd", "password", "privatekey", "secret", "session", "token",
	} {
		if strings.Contains(compact, protected) {
			return true
		}
	}
	for _, part := range strings.FieldsFunc(lower, func(character rune) bool {
		return character == '_' || character == '-' || character == '.' || character == '/'
	}) {
		switch part {
		case "authorization", "cookie", "credential", "credentials", "csrf", "passwd", "password", "secret", "session", "token":
			return true
		}
	}
	return false
}
