package observation

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (provider *CachedProvider) GetHealthHistory(ctx context.Context, query domain.HealthHistoryQuery) (domain.HealthHistoryResult, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.HealthHistoryResult{}, err
	}
	reader, ok := provider.reader.(domain.HealthHistoryProvider)
	if !ok {
		return domain.HealthHistoryResult{}, readError(ctx, "observation.health.history", ErrUnavailable)
	}
	result, err := reader.GetHealthHistory(ctx, query)
	if err == nil && result.ValidateFor(query) != nil {
		err = ErrUnavailable
	}
	return result, readError(ctx, "observation.health.history", err)
}
