package httpapi

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
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
	return (logpolicy.Reader{Provider: reader.reader, Settings: reader.settings, Redactor: reader.policy}).ReadLogs(ctx, query)
}

type configuredLogStreamProvider struct {
	ProductProvider
	configuredLogReader
}

func (provider configuredLogStreamProvider) ObservationTime() time.Time {
	return observationTime(provider.ProductProvider)
}
