package assistantservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type fakeRepository struct {
	assistant.RuntimeRepository
	mu           sync.Mutex
	runs         map[string]assistant.Run
	actors       map[string]assistant.Actor
	digests      map[[32]byte]string
	fingerprints map[string][32]byte
	pending      map[string]assistant.Step
	trace        []string
	admissions   int
	admitError   error
}

func newRepository() *fakeRepository {
	return &fakeRepository{runs: map[string]assistant.Run{}, actors: map[string]assistant.Actor{}, digests: map[[32]byte]string{}, fingerprints: map[string][32]byte{}, pending: map[string]assistant.Step{}}
}

func (repository *fakeRepository) AdmitAssistant(ctx context.Context, a assistant.Admission) (assistant.Run, error) {
	outcome, err := repository.AdmitAssistantWithOutcome(ctx, a)
	return outcome.Run, err
}

func (repository *fakeRepository) AdmitAssistantWithOutcome(ctx context.Context, a assistant.Admission) (assistant.AdmissionResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if ctx.Err() != nil {
		return assistant.AdmissionResult{}, assistant.ErrUnavailable
	}
	if repository.admitError != nil {
		return assistant.AdmissionResult{}, repository.admitError
	}
	if id, exists := repository.digests[a.IdempotencyDigest]; exists {
		if repository.fingerprints[id] != a.Fingerprint {
			return assistant.AdmissionResult{}, assistant.ErrConflict
		}
		return assistant.AdmissionResult{Run: repository.runs[id]}, nil
	}
	if a.RequireExisting {
		return assistant.AdmissionResult{}, assistant.ErrConflict
	}
	run := runFrom(a)
	run.Version, run.ConfigurationVersion = 1, 1
	repository.runs[a.ID], repository.actors[a.ID], repository.digests[a.IdempotencyDigest], repository.fingerprints[a.ID] = run, a.Actor, a.ID, a.Fingerprint
	repository.admissions++
	return assistant.AdmissionResult{Run: run, Created: true}, nil
}

func (repository *fakeRepository) RecoverAssistant(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (repository *fakeRepository) ClaimAssistant(ctx context.Context, claim assistant.Claim) (assistant.Run, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if ctx.Err() != nil {
		return assistant.Run{}, false, assistant.ErrUnavailable
	}
	ids := []string{}
	for id, run := range repository.runs {
		if run.State == assistant.Queued {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return assistant.Run{}, false, nil
	}
	run := repository.runs[ids[0]]
	run.State, run.LeaseOwner, run.LeaseUntil = assistant.Running, claim.Worker, claim.At.Add(claim.Lease)
	if run.LeaseUntil.After(run.Deadline) {
		run.LeaseUntil = run.Deadline
	}
	run.Attempts++
	run.UpdatedAt = claim.At
	run.Version++
	run.Fence++
	repository.runs[run.ID] = run
	return run, true, nil
}

func (repository *fakeRepository) RenewAssistantLease(ctx context.Context, lease assistant.Lease, duration time.Duration) (assistant.Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run := repository.runs[lease.RunID]
	if ctx.Err() != nil || run.State != assistant.Running || run.LeaseOwner != lease.Worker || run.Fence != lease.Fence {
		return run, assistant.ErrConflict
	}
	run.LeaseUntil = lease.At.Add(duration)
	if run.LeaseUntil.After(run.Deadline) {
		run.LeaseUntil = run.Deadline
	}
	repository.runs[run.ID] = run
	return run, nil
}

func (repository *fakeRepository) BeginAssistantStep(ctx context.Context, request assistant.StepStart) (assistant.Run, assistant.Step, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run := repository.runs[request.Lease.RunID]
	if ctx.Err() != nil || run.Version != request.Lease.Version || run.State != assistant.Running || run.Fence != request.Lease.Fence {
		return run, assistant.Step{}, assistant.ErrConflict
	}
	if _, pending := repository.pending[run.ID]; pending {
		return run, assistant.Step{}, assistant.ErrConflict
	}
	if request.Kind == assistant.Inference && request.MaximumUnits <= 0 || request.Kind != assistant.Inference && request.MaximumUnits != 0 {
		return run, assistant.Step{}, assistant.ErrInvalid
	}
	if request.MaximumUnits > run.ReservedUnits-run.UsedUnits {
		return run, assistant.Step{}, assistant.ErrQuota
	}
	step := assistant.Step{Number: run.ToolCalls + run.InferenceCalls + 1, Kind: request.Kind, State: "running", MaximumUnits: request.MaximumUnits}
	if request.Kind == assistant.Inference {
		run.InferenceCalls++
	} else {
		run.ToolCalls++
	}
	run.Version++
	run.UpdatedAt = request.Lease.At
	repository.runs[run.ID], repository.pending[run.ID] = run, step
	repository.trace = append(repository.trace, "begin:"+string(request.Kind))
	return run, step, nil
}

func (repository *fakeRepository) FinishAssistantStep(ctx context.Context, request assistant.StepFinish) (assistant.Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run := repository.runs[request.Lease.RunID]
	step, exists := repository.pending[run.ID]
	if ctx.Err() != nil || !exists || step.Number != request.Number || run.Version != request.Lease.Version || run.Fence != request.Lease.Fence {
		return run, assistant.ErrConflict
	}
	delete(repository.pending, run.ID)
	if request.Uncertain {
		run.State = assistant.Uncertain
		run.UsageUncertain = true
		run.Fence++
		run.LeaseOwner, run.LeaseUntil = "", time.Time{}
	}
	run.Version++
	run.UpdatedAt = request.Lease.At
	run.UsedUnits += request.Units
	repository.runs[run.ID] = run
	repository.trace = append(repository.trace, "finish:"+string(step.Kind))
	return run, nil
}

func (repository *fakeRepository) FinishAssistant(ctx context.Context, request assistant.Finish) (assistant.Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run := repository.runs[request.Lease.RunID]
	if ctx.Err() != nil || run.Version != request.Lease.Version || run.Fence != request.Lease.Fence || run.State != assistant.Running {
		return run, assistant.ErrConflict
	}
	run.State = request.State
	run.LeaseOwner, run.LeaseUntil = "", time.Time{}
	run.UpdatedAt = request.Lease.At
	run.Version++
	run.Fence++
	repository.runs[run.ID] = run
	delete(repository.pending, run.ID)
	repository.trace = append(repository.trace, "terminal:"+string(request.State))
	return run, nil
}

func (repository *fakeRepository) GetAssistant(ctx context.Context, actor assistant.Actor, id string, _ time.Time) (assistant.Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run, exists := repository.runs[id]
	if ctx.Err() != nil || !exists || repository.actors[id] != actor {
		return assistant.Run{}, assistant.ErrDenied
	}
	return run, nil
}

func (repository *fakeRepository) CancelAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	run := repository.runs[id]
	if ctx.Err() != nil || repository.actors[id] != actor {
		return assistant.Run{}, assistant.ErrDenied
	}
	if !run.State.Terminal() {
		run.State = assistant.Cancelled
		run.LeaseOwner, run.LeaseUntil = "", time.Time{}
		run.UpdatedAt = at
		run.Version++
		run.Fence++
		repository.runs[id] = run
	}
	return run, nil
}

type fakeAuthorizer struct {
	denied atomic.Bool
	calls  atomic.Int32
	reader *assistantevidence.Reader
	reads  *fixtureInventoryReads
}

func (authorizer *fakeAuthorizer) check(ctx context.Context) error {
	authorizer.calls.Add(1)
	if ctx.Err() != nil || authorizer.denied.Load() || authorizer.reader == nil {
		return assistant.ErrDenied
	}
	return nil
}

func (authorizer *fakeAuthorizer) AuthorizeAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	if err := authorizer.check(ctx); err != nil {
		return err
	}
	return authorizer.reader.AuthorizeAdmission(ctx, actor, run, scope)
}

func (authorizer *fakeAuthorizer) Authorize(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	if err := authorizer.check(ctx); err != nil {
		return err
	}
	return authorizer.reader.Authorize(ctx, actor, run, scope)
}

func (authorizer *fakeAuthorizer) AuthorizeEvidence(ctx context.Context, actor assistant.Actor, run assistant.Run, evidence assistantevidence.Evidence) error {
	if err := authorizer.check(ctx); err != nil {
		return err
	}
	return authorizer.reader.AuthorizeEvidence(ctx, actor, run, evidence)
}

type fixtureMetadataGate struct {
	repository *fakeRepository
	authorizer *fakeAuthorizer
}

func (gate fixtureMetadataGate) CheckAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	if ctx.Err() != nil || gate.authorizer.denied.Load() || !actor.Valid() || run.OwnerID != actor.UserID || run.Version != 0 || run.ConfigurationVersion != 0 || run.Fence != 0 || run.State != assistant.Queued || !run.Deadline.After(at) {
		return assistant.ErrDenied
	}
	return nil
}

func (gate fixtureMetadataGate) Check(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	gate.repository.mu.Lock()
	defer gate.repository.mu.Unlock()
	stored, found := gate.repository.runs[run.ID]
	// Active I/O may hold an earlier metadata snapshot while its own lease renews.
	expected := run
	if run.State == assistant.Running && stored.State == assistant.Running {
		if stored.Version < run.Version || stored.UpdatedAt.Before(run.UpdatedAt) || stored.LeaseUntil.Before(run.LeaseUntil) || stored.ToolCalls < run.ToolCalls || stored.InferenceCalls < run.InferenceCalls || stored.UsedUnits < run.UsedUnits {
			return assistant.ErrDenied
		}
		if stored.Version == run.Version && (!stored.UpdatedAt.Equal(run.UpdatedAt) || stored.ToolCalls != run.ToolCalls || stored.InferenceCalls != run.InferenceCalls || stored.UsedUnits != run.UsedUnits) {
			return assistant.ErrDenied
		}
		expected.Version, expected.UpdatedAt, expected.LeaseUntil = stored.Version, stored.UpdatedAt, stored.LeaseUntil
		expected.ToolCalls, expected.InferenceCalls, expected.UsedUnits = stored.ToolCalls, stored.InferenceCalls, stored.UsedUnits
	}
	if ctx.Err() != nil || gate.authorizer.denied.Load() || !found || gate.repository.actors[run.ID] != actor || expected != stored || run.UsageUncertain {
		return assistant.ErrDenied
	}
	if run.State == assistant.Running && (!stored.LeaseUntil.After(at) || !run.Deadline.After(at)) {
		return assistant.ErrDenied
	}
	return nil
}

type fixtureInventoryReads struct {
	beforeLive       func(context.Context) error
	source           domain.ProviderRef
	resource         domain.ResourceRef
	now              func() time.Time
	invalidSynthetic atomic.Bool
}

func (reads *fixtureInventoryReads) status() domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: reads.now().UTC().Truncate(time.Microsecond)}
}

func (reads *fixtureInventoryReads) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if ctx.Err() != nil || query.Validate() != nil || query.Source != reads.source || query.ResourceID != reads.resource.ID {
		return domain.ResourceSnapshot{}, assistantevidence.ErrDenied
	}
	return domain.ResourceSnapshot{Resource: domain.Resource{Ref: reads.resource, DisplayName: "Fixture service", Synthetic: !reads.invalidSynthetic.Load()}, Status: reads.status(), Health: domain.HealthHealthy}, nil
}

func (reads *fixtureInventoryReads) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if ctx.Err() != nil || source != reads.source {
		return domain.CapabilitySet{}, assistantevidence.ErrDenied
	}
	result := domain.CapabilitySet{Source: source}
	for _, name := range domain.CapabilityNames() {
		status := reads.status()
		if name != domain.CapabilityInventoryDetail {
			status.State, status.ReasonCode = domain.AvailabilityUnsupported, "fixture.unsupported"
		}
		result.Capabilities = append(result.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	return result, nil
}

func (*fixtureInventoryReads) GetHealth(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
	return domain.HealthResult{}, assistantevidence.ErrDenied
}
func (*fixtureInventoryReads) GetMetricHistory(context.Context, domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	return domain.MetricHistoryResult{}, assistantevidence.ErrDenied
}
func (*fixtureInventoryReads) ListEvents(context.Context, domain.EventQuery) (domain.EventPage, error) {
	return domain.EventPage{}, assistantevidence.ErrDenied
}
func (*fixtureInventoryReads) GetRelationships(context.Context, domain.TopologyQuery) (domain.RelationshipPage, error) {
	return domain.RelationshipPage{}, assistantevidence.ErrDenied
}

func (reads *fixtureInventoryReads) Check(ctx context.Context, scope contextbuilder.Scope, kind assistant.StepKind, logConsent bool) error {
	if ctx.Err() != nil || scope.Source != reads.source || len(scope.Resources) != 1 || scope.Resources[0] != reads.resource || kind != assistant.Inventory || logConsent {
		return assistantevidence.ErrDenied
	}
	if reads.beforeLive != nil {
		return reads.beforeLive(ctx)
	}
	return nil
}

type collectorFunc func(context.Context, assistantevidence.ReadRequest) (assistantevidence.Evidence, error)

func (collect collectorFunc) Read(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
	return collect(ctx, request)
}

type modelFixture struct {
	mu         sync.Mutex
	refs       map[int32]string
	counts     atomic.Int32
	calls      atomic.Int32
	hook       func(context.Context) error
	countError bool
}

func (model *modelFixture) Count(_ context.Context, request inference.Request) (inference.CountedInput, error) {
	if model.countError {
		return inference.CountedInput{}, inference.ErrUnavailable
	}
	index := model.counts.Add(1)
	model.mu.Lock()
	model.refs[index] = request.Evidence[0].ID
	model.mu.Unlock()
	return inference.CountedInput{Tokens: []int32{index, 1, 2}, Bytes: len(request.Question) + len(request.Evidence[0].Text), Revision: "fixture_revision"}, nil
}

func (model *modelFixture) Generate(ctx context.Context, input inference.CountedInput, _ int) (inference.Generated, error) {
	model.calls.Add(1)
	if model.hook != nil {
		if err := model.hook(ctx); err != nil {
			return inference.Generated{}, err
		}
	}
	model.mu.Lock()
	ref := model.refs[input.Tokens[0]]
	model.mu.Unlock()
	content, _ := json.Marshal(inference.Answer{Sections: []inference.Section{{Kind: inference.SectionFacts, Text: "Health is healthy.", Citations: []string{ref}}}})
	return inference.Generated{Content: content, InputTokens: len(input.Tokens), OutputTokens: 20, Complete: true, Revision: input.Revision}, nil
}

func inputFixture(t *testing.T, index int) Input {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Microsecond)
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, "native_fixture")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	a := assistant.Admission{ID: fmt.Sprintf("run_%02d", index), ConversationID: "conversation", Actor: assistant.Actor{UserID: "user", SessionDigest: [32]byte{1}, DigestKeyID: "fixture_key", AuthRevision: 1}, IdempotencyDigest: [32]byte{byte(index + 1)}, Goal: assistant.ExplainResource, SourceID: string(source.SourceID), SourceVersion: 1, ResourceID: string(ref.ID), From: at.Add(-time.Minute), Until: at, PolicyVersion: 1, ProfileID: "profile", ProfileVersion: 1, ReservedUnits: inference.MaximumInputTokens + 128}
	return Input{Admission: a, Scope: contextbuilder.Scope{Source: source, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: a.From, End: a.Until}, Resources: []domain.ResourceRef{ref}}, Locale: "en", Question: "Explain the selected resource."}
}

func fixtureService(t *testing.T, alter func(*Options, *modelFixture)) (*Service, *fakeRepository, *fakeAuthorizer, *modelFixture) {
	t.Helper()
	repository, authorizer, model := newRepository(), &fakeAuthorizer{}, &modelFixture{refs: map[int32]string{}}
	gateway, err := inference.New(model, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{Repository: repository, Gateway: gateway, Authorizer: authorizer, Lifetime: context.Background(), Now: time.Now, ProcessID: "process", FingerprintKey: append([]byte{1}, make([]byte, 31)...), Workers: 1, PollInterval: 100 * time.Millisecond, Heartbeat: time.Second, OutputLimit: 128}
	fixtureScope := inputFixture(t, 0).Scope
	reads := &fixtureInventoryReads{source: fixtureScope.Source, resource: fixtureScope.Resources[0], now: func() time.Time { return options.Now() }}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: fixtureMetadataGate{repository: repository, authorizer: authorizer}, Now: func() time.Time { return options.Now() }, Sources: []assistantevidence.Source{{Reference: reads.source, ConfigurationVersion: 1, Read: reads, Live: reads}}})
	if err != nil {
		t.Fatal(err)
	}
	authorizer.reader, authorizer.reads = reader, reads
	options.Collector = collectorFunc(func(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
		repository.mu.Lock()
		repository.trace = append(repository.trace, "read:"+string(request.Kind))
		repository.mu.Unlock()
		return reader.Read(ctx, request)
	})
	if alter != nil {
		alter(&options, model)
	}
	service, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	return service, repository, authorizer, model
}
