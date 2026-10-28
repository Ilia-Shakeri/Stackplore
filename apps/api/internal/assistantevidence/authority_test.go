package assistantevidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type captureStub struct {
	capture  domain.InventoryCapture
	calls    int
	read     func(context.Context) error
	metrics  func(context.Context, domain.CurrentMetricQuery) (domain.CurrentMetricResult, error)
	events   func(context.Context, domain.EventQuery) (domain.EventPage, error)
	health   func(context.Context, domain.HealthQuery) (domain.HealthResult, error)
	topology func(context.Context, domain.TopologyQuery) (domain.RelationshipPage, error)
}

func (provider *captureStub) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if provider.health == nil {
		return domain.HealthResult{}, ErrUnavailable
	}
	return provider.health(ctx, query)
}
func (provider *captureStub) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if provider.topology == nil {
		return domain.RelationshipPage{}, ErrUnavailable
	}
	return provider.topology(ctx, query)
}

func (provider *captureStub) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if provider.metrics == nil {
		return domain.CurrentMetricResult{}, ErrUnavailable
	}
	return provider.metrics(ctx, query)
}
func (provider *captureStub) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if provider.events == nil {
		return domain.EventPage{}, ErrUnavailable
	}
	return provider.events(ctx, query)
}

func (provider *captureStub) CaptureInventory(ctx context.Context) (domain.InventoryCapture, error) {
	provider.calls++
	if provider.read != nil {
		if err := provider.read(ctx); err != nil {
			return domain.InventoryCapture{}, err
		}
	}
	return provider.capture, nil
}
func captureFixture(t *testing.T) (CaptureAuthority, contextbuilder.Scope, *captureStub) {
	t.Helper()
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	source := domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_fixture"}
	ref := newRef(t, source, "native_fixture")
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	provider := &captureStub{capture: domain.InventoryCapture{Source: source, Status: status, Complete: true, Environments: []domain.Environment{{ID: ref.Identity.EnvironmentID, SourceID: source.SourceID, Provider: source.Provider, Name: "fixture", Status: status}}, Resources: []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: ref, DisplayName: "fixture"}, Status: status}}, Coverage: []domain.InventoryCoverage{{Source: source, EnvironmentID: ref.Identity.EnvironmentID, Kind: ref.Identity.Kind, ScopeMode: domain.InventoryExactScope, Scope: ref.Identity.Scope, Status: status}}}}
	return CaptureAuthority{Source: source, Provider: provider, Now: func() time.Time { return at }}, contextbuilder.Scope{Source: source, Resources: []domain.ResourceRef{ref}}, provider
}
func TestCaptureAuthorityRequiresCurrentExactCoverage(t *testing.T) {
	authority, scope, provider := captureFixture(t)
	if err := authority.Check(context.Background(), scope, assistant.Inventory, false); err != nil || provider.calls != 1 {
		t.Fatal("current capture denied")
	}
	for name, edit := range map[string]func(*CaptureAuthority, *contextbuilder.Scope, *captureStub){
		"denied": func(_ *CaptureAuthority, _ *contextbuilder.Scope, p *captureStub) {
			p.capture.Status.State = domain.AvailabilityPermissionDenied
			p.capture.Status.ReasonCode = "source.denied"
			p.capture.Complete = false
			p.capture.Resources = nil
			p.capture.Coverage = nil
			p.capture.Environments = nil
		},
		"missing": func(_ *CaptureAuthority, _ *contextbuilder.Scope, p *captureStub) { p.capture.Resources = nil },
		"stale": func(a *CaptureAuthority, _ *contextbuilder.Scope, _ *captureStub) {
			at := a.Now().Add(6 * time.Second)
			a.Now = func() time.Time { return at }
		},
		"future": func(a *CaptureAuthority, _ *contextbuilder.Scope, _ *captureStub) {
			at := a.Now().Add(-time.Second)
			a.Now = func() time.Time { return at }
		},
		"partition": func(_ *CaptureAuthority, _ *contextbuilder.Scope, p *captureStub) {
			p.capture.Coverage[0].Scope.Name = "other"
		},
		"duplicate": func(_ *CaptureAuthority, s *contextbuilder.Scope, _ *captureStub) {
			s.Resources = append(s.Resources, s.Resources[0])
		},
		"unknown-source": func(_ *CaptureAuthority, s *contextbuilder.Scope, _ *captureStub) { s.Source.SourceID = "other" },
		"incomplete-no-partition": func(_ *CaptureAuthority, _ *contextbuilder.Scope, p *captureStub) {
			p.capture.Complete = false
			p.capture.Coverage = nil
		},
		"zero-clock": func(a *CaptureAuthority, _ *contextbuilder.Scope, _ *captureStub) {
			a.Now = func() time.Time { return time.Time{} }
		},
		"opaque-error": func(_ *CaptureAuthority, _ *contextbuilder.Scope, p *captureStub) {
			p.read = func(context.Context) error { return errors.New("private_provider_text") }
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, s, p := captureFixture(t)
			edit(&a, &s, p)
			if err := a.Check(context.Background(), s, assistant.Inventory, false); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("invalid live proof accepted or error exposed")
			}
		})
	}
}

func TestCaptureCancellationAndFixedDemoTimestamp(t *testing.T) {
	authority, scope, provider := captureFixture(t)
	provider.read = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := authority.Check(ctx, scope, assistant.Inventory, false); !errors.Is(err, ErrCancelled) {
		t.Fatal("capture cancellation lost")
	}
	f := newFixture(t)
	resource, _ := f.reads.GetResource(context.Background(), domain.ResourceDetailQuery{Source: f.request.Scope.Source, ResourceID: f.request.Scope.Resources[0].ID})
	resource.Status.ObservedAt = resource.Status.ObservedAt.Add(-24 * time.Hour)
	status := resource.Status
	provider = &captureStub{capture: domain.InventoryCapture{Source: f.request.Scope.Source, Status: status, Complete: true, Environments: []domain.Environment{{ID: resource.Resource.Ref.Identity.EnvironmentID, SourceID: f.request.Scope.Source.SourceID, Provider: domain.ProviderDemo, Name: "fixture", Status: status, Synthetic: true}}, Resources: []domain.ResourceSnapshot{resource}}}
	authority = CaptureAuthority{Source: f.request.Scope.Source, Provider: provider, Now: func() time.Time { return *f.now }}
	scope = f.request.Scope
	scope.Resources = scope.Resources[:1]
	if err := authority.Check(context.Background(), scope, assistant.Inventory, false); err != nil {
		t.Fatal("fixed synthetic observation denied")
	}
}

func TestCaptureAuthoritySeparatelyChecksMetricEventAndLogPermissions(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.Health, assistant.Topology, assistant.Metrics, assistant.Events, assistant.Logs} {
		t.Run(string(kind), func(t *testing.T) {
			authority, scope, provider := captureFixture(t)
			scope.Range = domain.TimeRange{Start: authority.Now().Add(-time.Minute), End: authority.Now()}
			if err := authority.Check(context.Background(), scope, kind, true); err == nil {
				t.Fatal("missing family authority accepted")
			}
			provider.metrics = func(_ context.Context, q domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
				result := domain.CurrentMetricResult{Status: provider.capture.Status}
				for _, ref := range q.Resources {
					status := provider.capture.Status
					status.Completeness = domain.CompletenessPartial
					status.ReasonCode = "fixture.partial"
					result.Sets = append(result.Sets, domain.CurrentMetricSet{ResourceID: ref.ID, Status: status})
				}
				return result, nil
			}
			provider.health = func(_ context.Context, q domain.HealthQuery) (domain.HealthResult, error) {
				result := domain.HealthResult{Status: provider.capture.Status}
				for _, ref := range q.Resources {
					result.Snapshots = append(result.Snapshots, domain.HealthSnapshot{ResourceID: ref.ID, Status: provider.capture.Status, State: domain.HealthUnknown})
				}
				return result, nil
			}
			provider.topology = func(_ context.Context, q domain.TopologyQuery) (domain.RelationshipPage, error) {
				return domain.RelationshipPage{Status: provider.capture.Status, Page: domain.PageInfo{Limit: q.Page.Limit}}, nil
			}
			clock := authority.Now()
			authority.Now = func() time.Time { return clock }
			provider.events = func(_ context.Context, q domain.EventQuery) (domain.EventPage, error) {
				clock = clock.Add(time.Second)
				status := provider.capture.Status
				status.ObservedAt = clock
				return domain.EventPage{Status: status, Page: domain.PageInfo{Limit: q.Page.Limit}}, nil
			}
			if kind != assistant.Logs {
				if err := authority.Check(context.Background(), scope, kind, true); err != nil {
					t.Fatal("fresh family proof denied")
				}
			}
			denied := provider.capture.Status
			denied.State = domain.AvailabilityPermissionDenied
			denied.ReasonCode = "source.denied"
			provider.health = func(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
				return domain.HealthResult{Status: denied}, nil
			}
			provider.topology = func(_ context.Context, q domain.TopologyQuery) (domain.RelationshipPage, error) {
				return domain.RelationshipPage{Status: denied, Page: domain.PageInfo{Limit: q.Page.Limit}}, nil
			}
			provider.metrics = func(context.Context, domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
				return domain.CurrentMetricResult{Status: denied}, nil
			}
			provider.events = func(_ context.Context, q domain.EventQuery) (domain.EventPage, error) {
				return domain.EventPage{Status: denied, Page: domain.PageInfo{Limit: q.Page.Limit}}, nil
			}
			if kind == assistant.Logs {
				f := newFixture(t)
				authority.Source = f.request.Scope.Source
				scope = f.request.Scope
				resource, _ := f.reads.GetResource(context.Background(), domain.ResourceDetailQuery{Source: scope.Source, ResourceID: scope.Resources[0].ID})
				scope.Resources = scope.Resources[:1]
				provider.capture = domain.InventoryCapture{Source: scope.Source, Status: resource.Status, Complete: true, Environments: []domain.Environment{{ID: resource.Resource.Ref.Identity.EnvironmentID, SourceID: scope.Source.SourceID, Provider: scope.Source.Provider, Name: "fixture", Status: resource.Status, Synthetic: true}}, Resources: []domain.ResourceSnapshot{resource}}
				authority.Logs = f.reader.sources[scope.Source.SourceID].Logs
				if err := authority.Check(context.Background(), scope, kind, true); err != nil {
					t.Fatal("fresh consented log proof denied")
				}
				if err := authority.Check(context.Background(), scope, kind, false); !errors.Is(err, ErrDenied) {
					t.Fatal("log probe ignored consent")
				}
				f.reads.logEdit = func(page *domain.LogPage) {
					page.Records = nil
					page.Status.State = domain.AvailabilityPermissionDenied
					page.Status.ReasonCode = "source.denied"
				}
			}
			if err := authority.Check(context.Background(), scope, kind, true); !errors.Is(err, ErrDenied) {
				t.Fatal("family denial hidden by inventory availability")
			}
		})
	}
}

func TestTopologyPartialProbeCannotAuthorizePersistedDerivedEdges(t *testing.T) {
	authority, scope, provider := captureFixture(t)
	provider.topology = func(_ context.Context, q domain.TopologyQuery) (domain.RelationshipPage, error) {
		status := provider.capture.Status
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "source.partial"
		return domain.RelationshipPage{Status: status, Page: domain.PageInfo{Limit: q.Page.Limit}}, nil
	}
	if err := authority.Check(context.Background(), scope, assistant.Topology, false); !errors.Is(err, ErrDenied) {
		t.Fatal("partial third partition authorized cached edge")
	}
}

func TestCitationResolutionRechecksOwnerGenerationAndLiveScope(t *testing.T) {
	f := newFixture(t)
	evidence, err := f.reader.Read(context.Background(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	id := evidence.References()[0].ID
	completed := f.request.Run
	completed.State = assistant.Completed
	completed.Version++
	completed.Fence++
	locator, err := f.reader.Resolve(context.Background(), f.request.Actor, completed, evidence, id)
	if err != nil || locator.ResourceID != f.request.Scope.Resources[0].ID || locator.Kind != "resource" {
		t.Fatal("same-owner reference failed")
	}
	refs := evidence.References()
	refs[0].ID = "changed"
	if _, err := f.reader.Resolve(context.Background(), f.request.Actor, completed, evidence, id); err != nil {
		t.Fatal("caller mutated sealed references")
	}
	for name, edit := range map[string]func(*fixture, *assistant.Actor, *assistant.Run, *Evidence, *string){
		"owner": func(_ *fixture, a *assistant.Actor, _ *assistant.Run, _ *Evidence, _ *string) { a.UserID = "other" },
		"session": func(_ *fixture, a *assistant.Actor, _ *assistant.Run, _ *Evidence, _ *string) {
			a.SessionDigest = [32]byte{2}
		},
		"policy":  func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) { r.PolicyVersion++ },
		"restore": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) { r.RestoreGeneration++ },
		"window": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) {
			r.From = r.From.Add(-time.Second)
		},
		"running": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) {
			r.State = assistant.Running
		},
		"reclaimed-fence": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) { r.Fence++ },
		"changed-attempt": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) { r.Attempts++ },
		"unknown-usage": func(_ *fixture, _ *assistant.Actor, r *assistant.Run, _ *Evidence, _ *string) {
			r.UsageUncertain = true
		},
		"expired": func(f *fixture, _ *assistant.Actor, _ *assistant.Run, _ *Evidence, _ *string) {
			*f.now = (*f.now).Add(VolatileLifetime)
		},
		"unknown-id": func(_ *fixture, _ *assistant.Actor, _ *assistant.Run, _ *Evidence, id *string) {
			*id = "ev_" + strings.Repeat("0", 64)
		},
		"link": func(_ *fixture, _ *assistant.Actor, _ *assistant.Run, _ *Evidence, id *string) {
			*id = "https://example.invalid"
		},
		"unsealed":     func(_ *fixture, _ *assistant.Actor, _ *assistant.Run, e *Evidence, _ *string) { *e = Evidence{} },
		"other-reader": func(_ *fixture, _ *assistant.Actor, _ *assistant.Run, e *Evidence, _ *string) { e.reader = &Reader{} },
		"live-revoked": func(f *fixture, _ *assistant.Actor, _ *assistant.Run, _ *Evidence, _ *string) { f.live.err = ErrDenied },
		"metadata-revoked": func(f *fixture, _ *assistant.Actor, _ *assistant.Run, _ *Evidence, _ *string) {
			f.gate.check = func(context.Context, assistant.Actor, assistant.Run, time.Time) error { return ErrDenied }
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			e, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			a := f.request.Actor
			r := f.request.Run
			r.State = assistant.Completed
			r.Version++
			r.Fence++
			id := e.References()[0].ID
			edit(&f, &a, &r, &e, &id)
			if got, err := f.reader.Resolve(context.Background(), a, r, e, id); err == nil || got.Kind != "" {
				t.Fatal("invalid citation resolved")
			}
		})
	}
}

func TestConcurrencyDeadlineAndCancelledWaiter(t *testing.T) {
	f := newFixture(t)
	entered := make(chan struct{}, MaximumConcurrent)
	release := make(chan struct{})
	var active, maximum atomic.Int64
	f.gate.check = func(ctx context.Context, _ assistant.Actor, _ assistant.Run, _ time.Time) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > MaximumDuration+time.Second {
			return ErrInvalid
		}
		current := active.Add(1)
		defer active.Add(-1)
		for previous := maximum.Load(); current > previous; previous = maximum.Load() {
			if maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return ErrDenied
		}
	}
	results := make(chan error, 3)
	for range MaximumConcurrent {
		go func() {
			results <- f.reader.Authorize(context.Background(), f.request.Actor, f.request.Run, f.request.Scope)
		}()
	}
	for range MaximumConcurrent {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not reach gate")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.reader.Authorize(ctx, f.request.Actor, f.request.Run, f.request.Scope); !errors.Is(err, ErrCancelled) {
		t.Fatal("cancelled queue admitted")
	}
	if f.gate.checks.Load() != 2 || maximum.Load() != 2 {
		t.Fatal("concurrency cap violated")
	}
	close(release)
	for range MaximumConcurrent {
		if err := <-results; !errors.Is(err, ErrDenied) {
			t.Fatal("worker failed")
		}
	}
}

type authorityStub struct{ gateStub }

func (repo *authorityStub) AuthorizeAssistantEvidence(ctx context.Context, a assistant.Actor, r assistant.Run, at time.Time) error {
	return repo.Check(ctx, a, r, at)
}
func (repo *authorityStub) AuthorizeAssistantAdmission(ctx context.Context, a assistant.Actor, r assistant.Run, at time.Time) error {
	return repo.CheckAdmission(ctx, a, r, at)
}
func TestRepositoryGateAndProtectedValues(t *testing.T) {
	f := newFixture(t)
	repo := &authorityStub{}
	gate := RepositoryGate{Repository: repo}
	if gate.Check(context.Background(), f.request.Actor, f.request.Run, *f.now) != nil || gate.CheckAdmission(context.Background(), f.request.Actor, f.request.Run, *f.now) != nil || repo.checks.Load() != 1 || repo.admissions.Load() != 1 {
		t.Fatal("metadata purpose changed")
	}
	if (RepositoryGate{}).Check(context.Background(), f.request.Actor, f.request.Run, *f.now) == nil || (RepositoryGate{}).CheckAdmission(context.Background(), f.request.Actor, f.request.Run, *f.now) == nil {
		t.Fatal("missing gate accepted")
	}
	evidence, err := f.reader.Read(context.Background(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{f.reader, f.request, Options{Gate: gate}, Source{Read: f.reads}, gate, CaptureAuthority{Provider: &captureStub{}}, evidence, CitationLocator{ObjectID: "private_fixture"}} {
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", value), "fixture") {
			t.Fatal("ordinary formatting exposed evidence")
		}
	}
}
