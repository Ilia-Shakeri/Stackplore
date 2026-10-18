package observation

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type deniedEventPageProvider struct {
	*demo.Provider
	calls         int
	denialAsError bool
}

func (provider *deniedEventPageProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	provider.calls++
	if provider.calls == 2 {
		if provider.denialAsError {
			failure, _ := domain.NewProviderError("fixture.events.read", domain.ProviderErrorPermissionDenied, false, errors.New("fixture permission denied"))
			return domain.EventPage{}, failure
		}
		return domain.EventPage{
			Status: unavailableStatus(domain.AvailabilityPermissionDenied, demo.ObservationTime(), "observation.permission_denied"),
			Events: []domain.InfrastructureEvent{},
			Page:   domain.PageInfo{Limit: query.Page.Limit},
		}, nil
	}
	result, err := provider.Provider.ListEvents(ctx, query)
	if err != nil {
		return result, err
	}
	result.Events = result.Events[:1]
	result.Page.HasMore = true
	result.Page.NextCursor = domain.PageCursor(base64.RawURLEncoding.EncodeToString([]byte("fixture-event-page-two")))
	return result, nil
}

func TestEventCollectionLaterPageDenialDiscardsEarlierFacts(t *testing.T) {
	for _, denialAsError := range []bool{false, true} {
		t.Run(map[bool]string{false: "status", true: "provider_error"}[denialAsError], func(t *testing.T) {
			provider := &deniedEventPageProvider{Provider: demo.MustNew(), denialAsError: denialAsError}
			inventory, err := provider.CaptureInventory(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			capabilities, err := provider.GetCapabilities(context.Background(), inventory.Source)
			if err != nil {
				t.Fatal(err)
			}
			collected := collection{inventory: inventory, capabilities: capabilities}
			worker := &Worker{maximumEvents: 100, maximumPages: 10, eventLookback: 24 * time.Hour}
			worker.collectEvents(context.Background(), provider, &collected, demo.ObservationTime)
			if provider.calls != 2 {
				t.Fatal("event collection did not reach the later-page denial")
			}
			for _, capability := range collected.capabilities.Capabilities {
				if capability.Name == domain.CapabilityEventHistory {
					if capability.Status.State != domain.AvailabilityPermissionDenied || capability.Status.Completeness != domain.CompletenessComplete || len(collected.events) != 0 {
						t.Fatal("later-page denial published earlier facts or reopened event authority")
					}
					return
				}
			}
			t.Fatal("event capability was omitted")
		})
	}
}
