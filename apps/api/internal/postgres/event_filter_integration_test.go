package postgres

import (
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"testing"
	"time"
)

func TestEventFiltersApplyBeforePaginationAndBindCursor(t *testing.T) {
	f := newObservationFixture(t)
	query := domain.EventQuery{Source: f.token.Source, Range: domain.TimeRange{Start: f.at.Add(-time.Hour), End: f.at.Add(time.Second)}, Page: domain.PageRequest{Limit: 1}}
	first, err := f.store.ListEvents(f.ctx, query)
	if err != nil || len(first.Events) != 1 || !first.Page.HasMore {
		t.Fatalf("event fixture page: %v", err)
	}
	query.Search = string(first.Events[0].ID)
	query.EnvironmentID = *first.Events[0].EnvironmentID
	query.Severities = []domain.Severity{first.Events[0].Severity}
	filtered, err := f.store.ListEvents(f.ctx, query)
	if err != nil || len(filtered.Events) != 1 || filtered.Page.HasMore || filtered.Events[0].ID != first.Events[0].ID {
		t.Fatalf("compound event filter: %v", err)
	}
	query.Page.Cursor = first.Page.NextCursor
	if _, err = f.store.ListEvents(f.ctx, query); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatal("cursor accepted changed event filters")
	}
	query.Page.Cursor = ""
	query.Search = "%"
	filtered, err = f.store.ListEvents(f.ctx, query)
	if err != nil || len(filtered.Events) != 0 {
		t.Fatal("event search treated literal percent as SQL wildcard")
	}
}
