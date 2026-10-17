package httpapi

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

type applicationPolicyReader interface {
	Get(context.Context) (domain.ApplicationSettingsRecord, error)
}

type configuredLogReader struct {
	reader   domain.LogReadProvider
	settings applicationPolicyReader
	policy   logsafe.Policy
}

func (reader configuredLogReader) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if reader.settings != nil {
		settings, err := reader.settings.Get(ctx)
		if err != nil || settings.Validate() != nil {
			return domain.LogPage{}, domain.ErrRepositoryUnavailable
		}
		query.LineLimit = min(query.LineLimit, settings.Values.LogLineLimit)
	}
	page, err := reader.reader.ReadLogs(ctx, query)
	if err != nil || !reader.policy.HasAdditionalFields() {
		return page, err
	}
	page.Records = append([]domain.LogRecord(nil), page.Records...)
	remaining := query.ByteLimit
	for index := range page.Records {
		record := &page.Records[index]
		message, redacted, truncated := reader.policy.Message(record.Message, record.Truncated)
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
	return page, nil
}

type configuredLogStreamProvider struct {
	ProductProvider
	configuredLogReader
}

func (provider configuredLogStreamProvider) ObservationTime() time.Time {
	return observationTime(provider.ProductProvider)
}
