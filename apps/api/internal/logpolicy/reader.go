// Package logpolicy applies effective application log limits outside transport.
package logpolicy

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type SettingsReader interface {
	Get(context.Context) (domain.ApplicationSettingsRecord, error)
}

type Redactor interface {
	HasAdditionalFields() bool
	Message(string, bool) (string, bool, bool)
}

type Reader struct {
	Provider domain.LogReadProvider
	Settings SettingsReader
	Redactor Redactor
}

var ErrInvalidCapture = errors.New("log capture is invalid")

// Capture carries a validated, policy-applied page without exposing mutable state.
type Capture struct {
	query domain.LogReadQuery
	page  domain.LogPage
	valid bool
}

func (Capture) Format(state fmt.State, _ rune) { _, _ = state.Write([]byte("[protected log capture]")) }
func (Capture) MarshalJSON() ([]byte, error)   { return []byte("{}"), nil }

func (capture Capture) Snapshot() (domain.LogReadQuery, domain.LogPage, bool) {
	query, page := capture.query, capture.page
	query.Targets = append([]domain.LogTarget(nil), query.Targets...)
	page.Records = append([]domain.LogRecord(nil), page.Records...)
	return query, page, capture.valid
}

// ReadLogs preserves provider default masking and adds deployment-specific fields.
func (reader Reader) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	page, _, err := reader.read(ctx, query, false)
	return page, err
}

// Capture always reapplies built-in masking, including when there are no added fields.
func (reader Reader) Capture(ctx context.Context, query domain.LogReadQuery) (Capture, error) {
	if query.Validate() != nil || reader.Redactor == nil {
		return Capture{}, ErrInvalidCapture
	}
	page, effective, err := reader.read(ctx, query, true)
	if err != nil {
		return Capture{}, err
	}
	if page.ValidateFor(effective) != nil {
		return Capture{}, ErrInvalidCapture
	}
	effective.Targets = append([]domain.LogTarget(nil), effective.Targets...)
	page.Records = append([]domain.LogRecord(nil), page.Records...)
	return Capture{query: effective, page: page, valid: true}, nil
}

func (reader Reader) read(ctx context.Context, query domain.LogReadQuery, force bool) (domain.LogPage, domain.LogReadQuery, error) {
	if ctx == nil || reader.Provider == nil {
		return domain.LogPage{}, query, domain.ErrRepositoryUnavailable
	}
	if err := ctx.Err(); err != nil {
		return domain.LogPage{}, query, err
	}
	if reader.Settings != nil {
		settings, err := reader.Settings.Get(ctx)
		if err != nil || settings.Validate() != nil {
			return domain.LogPage{}, query, domain.ErrRepositoryUnavailable
		}
		query.LineLimit = min(query.LineLimit, settings.Values.LogLineLimit)
	}
	page, err := reader.Provider.ReadLogs(ctx, query)
	if cancelled := ctx.Err(); cancelled != nil {
		return domain.LogPage{}, query, cancelled
	}
	if err == nil {
		if len(page.Records) > domain.MaxLogLines || len(page.Records) > query.LineLimit {
			return domain.LogPage{}, query, ErrInvalidCapture
		}
		for _, record := range page.Records {
			if len(record.Message) > domain.MaxLogLineBytes {
				return domain.LogPage{}, query, ErrInvalidCapture
			}
		}
	}
	if err != nil || reader.Redactor == nil || !force && !reader.Redactor.HasAdditionalFields() {
		return page, query, err
	}
	page.Records = append([]domain.LogRecord(nil), page.Records...)
	remaining := query.ByteLimit
	for index := range page.Records {
		record := &page.Records[index]
		message, redacted, truncated := reader.Redactor.Message(record.Message, record.Truncated)
		if len(message) > remaining {
			message = message[:max(0, remaining)]
			for !utf8.ValidString(message) {
				message = message[:len(message)-1]
			}
			truncated = true
		}
		record.Message, record.Redacted, record.Truncated = message, record.Redacted || redacted, truncated
		remaining -= len(message)
	}
	return page, query, nil
}
