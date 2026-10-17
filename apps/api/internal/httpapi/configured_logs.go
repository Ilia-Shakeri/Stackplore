package httpapi

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type applicationPolicyReader interface {
	Get(context.Context) (domain.ApplicationSettingsRecord, error)
}

type configuredLogReader struct {
	reader   domain.LogReadProvider
	settings applicationPolicyReader
}

func (reader configuredLogReader) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	settings, err := reader.settings.Get(ctx)
	if err != nil || settings.Validate() != nil {
		return domain.LogPage{}, domain.ErrRepositoryUnavailable
	}
	query.LineLimit = min(query.LineLimit, settings.Values.LogLineLimit)
	return reader.reader.ReadLogs(ctx, query)
}

type configuredLogStreamProvider struct {
	ProductProvider
	configuredLogReader
}
