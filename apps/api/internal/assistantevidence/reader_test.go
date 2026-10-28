package assistantevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

type gateStub struct {
	checks, admissions atomic.Int64
	check              func(context.Context, assistant.Actor, assistant.Run, time.Time) error
}

func (gate *gateStub) Check(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	gate.checks.Add(1)
	if gate.check != nil {
		return gate.check(ctx, actor, run, at)
	}
	return nil
}
func (gate *gateStub) CheckAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	gate.admissions.Add(1)
	if gate.check != nil {
		return gate.check(ctx, actor, run, at)
	}
	return nil
}

type liveStub struct {
	calls atomic.Int64
	err   error
	check func(context.Context) error
}

func (live *liveStub) Check(ctx context.Context, _ contextbuilder.Scope, _ assistant.StepKind, _ bool) error {
	live.calls.Add(1)
	if live.check != nil {
		return live.check(ctx)
	}
	return live.err
}

type settingsStub struct {
	at      time.Time
	limit   int
	calls   int
	version int64
}

func (settings *settingsStub) Get(context.Context) (domain.ApplicationSettingsRecord, error) {
	settings.calls++
	values := domain.DefaultApplicationValues()
	values.LogLineLimit = settings.limit
	version := settings.version
	if version == 0 {
		version = 1
	}
	return domain.ApplicationSettingsRecord{Version: version, UpdatedAt: settings.at, Values: values}, nil
}

type readsStub struct {
	source          domain.ProviderRef
	status          domain.ObservationStatus
	resources       []domain.ResourceRef
	calls           map[assistant.StepKind]int
	resourceCalls   int
	capabilityState domain.AvailabilityState
	capabilityEdit  func(*domain.CapabilitySet)
	family          func(assistant.StepKind) error
	metricEdit      func(*domain.MetricHistoryResult)
	logEdit         func(*domain.LogPage)
	lastLogs        domain.LogReadQuery
	outside         *domain.ResourceRef
}

func (reads *readsStub) enter(kind assistant.StepKind) error {
	reads.calls[kind]++
	if reads.family != nil {
		return reads.family(kind)
	}
	return nil
}
func (reads *readsStub) GetResource(_ context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	reads.resourceCalls++
	for _, ref := range reads.resources {
		if ref.ID == query.ResourceID {
			return domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: "fixture", Synthetic: reads.source.Provider == domain.ProviderDemo}, Status: reads.status, Health: domain.HealthHealthy}, nil
		}
	}
	return domain.ResourceSnapshot{}, ErrDenied
}
func (reads *readsStub) GetCapabilities(context.Context, domain.ProviderRef) (domain.CapabilitySet, error) {
	result := domain.CapabilitySet{Source: reads.source}
	for _, name := range domain.CapabilityNames() {
		status := reads.status
		if reads.capabilityState != "" {
			status.State = reads.capabilityState
			status.ReasonCode = "source.capability"
		}
		result.Capabilities = append(result.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	if reads.capabilityEdit != nil {
		reads.capabilityEdit(&result)
	}
	return result, nil
}
func (reads *readsStub) GetHealth(_ context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if err := reads.enter(assistant.Health); err != nil {
		return domain.HealthResult{}, err
	}
	result := domain.HealthResult{Status: reads.status}
	for _, ref := range query.Resources {
		result.Snapshots = append(result.Snapshots, domain.HealthSnapshot{ResourceID: ref.ID, State: domain.HealthHealthy, Status: reads.status, Evidence: []domain.HealthEvidence{{ID: "health_fixture", Code: "fixture.health", SourceCode: "fixture.source", Status: domain.EvidencePositive, ObservedAt: reads.status.ObservedAt, Synthetic: reads.source.Provider == domain.ProviderDemo}}})
	}
	return result, nil
}
func (reads *readsStub) GetMetricHistory(_ context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if err := reads.enter(assistant.Metrics); err != nil {
		return domain.MetricHistoryResult{}, err
	}
	if query.PointLimit != MaximumRecords || query.Validate() != nil || len(query.Names) != 3 {
		return domain.MetricHistoryResult{}, ErrInvalid
	}
	result := domain.MetricHistoryResult{Status: reads.status, Samples: []domain.MetricSample{{ID: "metric_fixture", ResourceID: query.Resources[0].ID, Name: domain.MetricMemoryUsageBytes, Unit: domain.MetricUnitBytes, Value: 1024, SampledAt: query.Range.Start.Add(time.Second), Interval: time.Second, Synthetic: reads.source.Provider == domain.ProviderDemo}}}
	if reads.metricEdit != nil {
		reads.metricEdit(&result)
	}
	return result, nil
}
func (reads *readsStub) ListEvents(_ context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := reads.enter(assistant.Events); err != nil {
		return domain.EventPage{}, err
	}
	if query.Page.Limit != MaximumRecords || query.Page.Cursor != "" || query.Search != "" || query.Validate() != nil {
		return domain.EventPage{}, ErrInvalid
	}
	id := query.Resources[0].ID
	return domain.EventPage{Status: reads.status, Events: []domain.InfrastructureEvent{{ID: "event_fixture", Provider: reads.source.Provider, SourceID: reads.source.SourceID, ResourceID: &id, Kind: "fixture.event", Severity: domain.SeverityInfo, ReasonCode: "fixture.event", Summary: "drop_this_prose", OccurredAt: query.Range.Start.Add(time.Second), ObservedAt: reads.status.ObservedAt, Synthetic: reads.source.Provider == domain.ProviderDemo}}, Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
}
func (reads *readsStub) GetRelationships(_ context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := reads.enter(assistant.Topology); err != nil {
		return domain.RelationshipPage{}, err
	}
	to := query.Resources[len(query.Resources)-1]
	if reads.outside != nil {
		to = *reads.outside
	}
	edge, err := domain.NewRelationship(query.Resources[0].ID, to.ID, domain.RelationshipMemberOf, domain.RelationshipUndirected, []domain.RelationshipEvidence{{ID: "edge_fixture", Provider: reads.source.Provider, SourceID: reads.source.SourceID, Provenance: domain.ProvenanceNetworkMembership, Certainty: domain.EvidenceInferred, Confidence: .5, ObservedAt: reads.status.ObservedAt, Synthetic: reads.source.Provider == domain.ProviderDemo}})
	if err != nil {
		return domain.RelationshipPage{}, err
	}
	return domain.RelationshipPage{Status: reads.status, Relationships: []domain.Relationship{edge}, Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
}
func (reads *readsStub) ReadLogs(_ context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if err := reads.enter(assistant.Logs); err != nil {
		return domain.LogPage{}, err
	}
	reads.lastLogs = query
	page := domain.LogPage{Status: reads.status, Page: domain.PageInfo{Limit: query.LineLimit}, Records: []domain.LogRecord{{ID: "log_fixture", ResourceID: query.Targets[0].Resource.ID, Stream: domain.LogStreamStdout, Severity: domain.LogSeverityInfo, Sequence: 1, Timestamp: query.Range.Start.Add(time.Second), Message: "customer_id=redaction_fixture password=mask_fixture", Synthetic: reads.source.Provider == domain.ProviderDemo}}}
	if reads.logEdit != nil {
		reads.logEdit(&page)
	}
	return page, nil
}

type fixture struct {
	reader   *Reader
	request  ReadRequest
	gate     *gateStub
	live     *liveStub
	reads    *readsStub
	settings *settingsStub
	now      *time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_fixture"}
	refs := []domain.ResourceRef{newRef(t, source, "first"), newRef(t, source, "second")}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	reads := &readsStub{source: source, status: status, resources: refs, calls: map[assistant.StepKind]int{}}
	gate, live := &gateStub{}, &liveStub{}
	settings := &settingsStub{at: at, limit: 10}
	redactor, err := logsafe.NewPolicy([]string{"customer_id"})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := New(Options{Gate: gate, Now: func() time.Time { return at }, Sources: []Source{{Reference: source, ConfigurationVersion: 3, Read: reads, Live: live, Logs: &logpolicy.Reader{Provider: reads, Settings: settings, Redactor: redactor}}}})
	if err != nil {
		t.Fatal(err)
	}
	actor := assistant.Actor{UserID: "owner_fixture", SessionDigest: [32]byte{1}, DigestKeyID: "key_fixture", AuthRevision: 1}
	run := assistant.Run{ID: "run_fixture", ConversationID: "conversation_fixture", OwnerID: actor.UserID, SourceID: string(source.SourceID), Goal: assistant.InvestigateIncident, State: assistant.Running, SourceVersion: 3, PolicyVersion: 2, ProfileID: "profile_fixture", ProfileVersion: 1, Version: 2, Fence: 1, ConfigurationVersion: 4, Volatile: true, CreatedAt: at, UpdatedAt: at, Deadline: at.Add(120 * time.Second), From: at.Add(-5 * time.Minute), Until: at, ReservedUnits: 1000, Attempts: 1, LeaseOwner: "worker_fixture", LeaseUntil: at.Add(30 * time.Second)}
	scope := contextbuilder.Scope{RunID: domain.RequestID(run.ID), Source: source, ConfigurationVersion: run.SourceVersion, PolicyVersion: run.PolicyVersion, Range: domain.TimeRange{Start: run.From, End: run.Until}, ObservedAt: at, Resources: refs}
	return fixture{reader: reader, request: ReadRequest{Actor: actor, Run: run, Scope: scope, Kind: assistant.Inventory, Destination: contextbuilder.Local}, gate: gate, live: live, reads: reads, settings: settings, now: &at}
}
func newRef(t *testing.T, source domain.ProviderRef, native string) domain.ResourceRef {
	t.Helper()
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment_fixture", "fixture.service", domain.Scope{Kind: domain.ScopeProject, Name: "project_fixture"}, domain.NativeResourceID(native))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestSixFixedReadFamiliesAndSealedItems(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.Inventory, assistant.Health, assistant.Metrics, assistant.Events, assistant.Topology, assistant.Logs} {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = kind
			f.request.LogConsent = true
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			items, err := evidence.Bundle().Items()
			if err != nil || len(items) == 0 || len(items) != len(evidence.References()) {
				t.Fatal("missing bounded evidence")
			}
			checks := int64(6)
			if kind == assistant.Logs {
				checks++
			}
			if f.live.calls.Load() != 4 || f.gate.checks.Load() != checks {
				t.Fatal("authority not rechecked around read/build/publication")
			}
			for _, item := range items {
				if !json.Valid([]byte(item.Text)) || !canonicalCitation(item.ID) || strings.Contains(item.Text, "drop_this_prose") {
					t.Fatal("invalid projection")
				}
			}
			if kind == assistant.Logs {
				if f.settings.calls != 2 || f.reads.lastLogs.LineLimit != 10 || f.reads.lastLogs.ByteLimit != contextbuilder.MaximumLogBytes || f.reads.lastLogs.Cursor != "" {
					t.Fatal("shared log limits bypassed")
				}
				if strings.Contains(items[0].Text, "redaction_fixture") || strings.Contains(items[0].Text, "mask_fixture") {
					t.Fatal("shared masking bypassed")
				}
			}
		})
	}
}

func TestAdmissionPurposeCannotDowngradeEvidence(t *testing.T) {
	f := newFixture(t)
	candidate := f.request.Run
	candidate.Version, candidate.Fence, candidate.ConfigurationVersion = 0, 0, 0
	candidate.State = assistant.Queued
	candidate.Attempts = 0
	candidate.LeaseOwner = ""
	candidate.LeaseUntil = time.Time{}
	if err := f.reader.AuthorizeAdmission(context.Background(), f.request.Actor, candidate, f.request.Scope); err != nil {
		t.Fatal(err)
	}
	if f.gate.admissions.Load() != 1 || f.gate.checks.Load() != 0 || f.live.calls.Load() != 0 || f.reads.resourceCalls != 0 {
		t.Fatal("admission read provider or evidence")
	}
	if err := f.reader.Authorize(context.Background(), f.request.Actor, candidate, f.request.Scope); !errors.Is(err, ErrDenied) {
		t.Fatal("zero run gained evidence authority")
	}
	queued := candidate
	queued.Version = 1
	queued.ConfigurationVersion = 4
	if err := f.reader.Authorize(context.Background(), f.request.Actor, queued, f.request.Scope); err != nil || f.live.calls.Load() != 0 {
		t.Fatal("queued metadata poll gained provider reads or failed")
	}
	f.request.Run = candidate
	if _, err := f.reader.Read(context.Background(), f.request); err == nil {
		t.Fatal("zero run read evidence")
	}
	candidate.Version = 1
	if err := f.reader.AuthorizeAdmission(context.Background(), f.request.Actor, candidate, f.request.Scope); !errors.Is(err, ErrDenied) {
		t.Fatal("stored run accepted as candidate")
	}
}

func TestMalformedScopesNeverReadProvider(t *testing.T) {
	for name, edit := range map[string]func(*ReadRequest){
		"other-owner": func(r *ReadRequest) { r.Actor.UserID = "other" }, "no-session": func(r *ReadRequest) { r.Actor.SessionDigest = [32]byte{} },
		"source-version": func(r *ReadRequest) { r.Scope.ConfigurationVersion++ }, "policy": func(r *ReadRequest) { r.Scope.PolicyVersion++ }, "restore": func(r *ReadRequest) { r.Scope.RestoreGeneration++ },
		"stale-adapter-version": func(r *ReadRequest) { r.Scope.ConfigurationVersion++; r.Run.SourceVersion++ },
		"duplicate":             func(r *ReadRequest) { r.Scope.Resources = append(r.Scope.Resources, r.Scope.Resources[0]) }, "no-resource": func(r *ReadRequest) { r.Scope.Resources = nil },
		"future": func(r *ReadRequest) { r.Scope.ObservedAt = r.Scope.ObservedAt.Add(time.Second) }, "range": func(r *ReadRequest) { r.Scope.Range.Start = r.Scope.Range.Start.Add(-time.Second) },
		"kind": func(r *ReadRequest) { r.Kind = assistant.Inference }, "paid": func(r *ReadRequest) { r.Run.Paid = true }, "deadline": func(r *ReadRequest) { r.Run.Deadline = r.Run.CreatedAt },
		"lease-expired": func(r *ReadRequest) { r.Run.LeaseUntil = r.Run.CreatedAt }, "no-lease-owner": func(r *ReadRequest) { r.Run.LeaseOwner = "" }, "no-attempt": func(r *ReadRequest) { r.Run.Attempts = 0 }, "uncertain": func(r *ReadRequest) { r.Run.UsageUncertain = true },
		"remote-logs": func(r *ReadRequest) {
			r.Kind = assistant.Logs
			r.LogConsent = true
			r.Destination = contextbuilder.Remote
		}, "unconsented-logs": func(r *ReadRequest) { r.Kind = assistant.Logs },
		"wide-logs": func(r *ReadRequest) {
			r.Kind = assistant.Logs
			r.LogConsent = true
			r.Scope.Range.Start = r.Scope.Range.End.Add(-16 * time.Minute)
			r.Run.From = r.Scope.Range.Start
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			edit(&f.request)
			if evidence, err := f.reader.Read(context.Background(), f.request); err == nil || evidence.reader != nil {
				t.Fatal("invalid request emitted evidence")
			}
			if f.gate.checks.Load() != 0 || f.live.calls.Load() != 0 || f.reads.resourceCalls != 0 {
				t.Fatal("invalid request performed I/O")
			}
		})
	}
}

func TestAuthorityRevocationAndProviderFailuresCannotPublish(t *testing.T) {
	for name, setup := range map[string]func(fixture){
		"initial-metadata": func(f fixture) {
			f.gate.check = func(context.Context, assistant.Actor, assistant.Run, time.Time) error { return assistant.ErrDenied }
		},
		"initial-live": func(f fixture) { f.live.err = ErrDenied },
		"after-family-metadata": func(f fixture) {
			f.reads.family = func(assistant.StepKind) error {
				f.gate.check = func(context.Context, assistant.Actor, assistant.Run, time.Time) error { return assistant.ErrDenied }
				return nil
			}
		},
		"after-family-live": func(f fixture) {
			f.reads.family = func(assistant.StepKind) error { f.live.err = ErrDenied; return nil }
		},
		"before-publication": func(f fixture) {
			f.gate.check = func(context.Context, assistant.Actor, assistant.Run, time.Time) error {
				if f.gate.checks.Load() >= 5 {
					return ErrDenied
				}
				return nil
			}
		},
		"raw-provider-error": func(f fixture) {
			f.reads.family = func(assistant.StepKind) error { return errors.New("provider_private_error") }
		},
		"invalid-provider-scope": func(f fixture) {
			f.reads.metricEdit = func(result *domain.MetricHistoryResult) { result.Samples[0].ResourceID = "invalid" }
		},
		"missing-capability": func(f fixture) {
			f.reads.capabilityEdit = func(set *domain.CapabilitySet) { set.Capabilities = set.Capabilities[1:] }
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = assistant.Metrics
			setup(f)
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err == nil || evidence.reader != nil || strings.Contains(fmt.Sprint(err), "private") {
				t.Fatal("revoked or invalid evidence escaped")
			}
		})
	}
}

func TestReadCancellationReleasesSlotAndSafeError(t *testing.T) {
	f := newFixture(t)
	f.reader.timeout = time.Millisecond
	f.live.check = func(ctx context.Context) error { <-ctx.Done(); return errors.New("provider_private_text") }
	started := time.Now()
	if evidence, err := f.reader.Read(context.Background(), f.request); !errors.Is(err, ErrCancelled) || evidence.reader != nil {
		t.Fatal("bounded cancellation failed")
	}
	if len(f.reader.slots) != 0 || time.Since(started) > time.Second {
		t.Fatal("slot or deadline leaked")
	}
}

func TestConstructorRejectsMissingPortsAndUnboundedOptions(t *testing.T) {
	f := newFixture(t)
	source := f.reader.sources[f.request.Scope.Source.SourceID]
	for name, edit := range map[string]func(*Options){"no-gate": func(o *Options) { o.Gate = nil }, "no-clock": func(o *Options) { o.Now = nil }, "no-source": func(o *Options) { o.Sources = nil }, "duplicate": func(o *Options) { o.Sources = append(o.Sources, o.Sources[0]) }, "no-read": func(o *Options) { o.Sources[0].Read = nil }, "no-live": func(o *Options) { o.Sources[0].Live = nil }, "wide-timeout": func(o *Options) { o.Timeout = MaximumDuration + 1 }, "tiny-timeout": func(o *Options) { o.Timeout = time.Nanosecond }, "raw-logs": func(o *Options) { o.Sources[0].Logs = &logpolicy.Reader{Provider: f.reads} }} {
		t.Run(name, func(t *testing.T) {
			options := Options{Gate: f.gate, Now: func() time.Time { return *f.now }, Sources: []Source{source}}
			edit(&options)
			if reader, err := New(options); !errors.Is(err, ErrInvalid) || reader != nil {
				t.Fatal("invalid composition accepted")
			}
		})
	}
}

func TestFamilyAuthorityAndLogPolicyRemainBoundToSealedEvidence(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.Metrics, assistant.Events, assistant.Logs} {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = kind
			f.request.LogConsent = true
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, evidence); err != nil {
				t.Fatal("sealed evidence denied")
			}
			f.live.check = func(context.Context) error { return ErrDenied }
			if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, evidence); !errors.Is(err, ErrDenied) {
				t.Fatal("family revocation left old content readable")
			}
		})
	}
	for _, change := range []string{"settings-version", "effective-limit", "redactor-fields", "consent", "late-denial"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = assistant.Logs
			f.request.LogConsent = true
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "settings-version":
				f.settings.version = 2
			case "effective-limit":
				f.settings.limit = 20
			case "redactor-fields":
				policy, err := logsafe.NewPolicy([]string{"customer_id", "request_id"})
				if err != nil {
					t.Fatal(err)
				}
				f.reader.sources[f.request.Scope.Source.SourceID].Logs.Redactor = policy
			case "consent":
				evidence.logConsent = false
			case "late-denial":
				f.reads.logEdit = func(page *domain.LogPage) {
					page.Records = nil
					page.Status.State = domain.AvailabilityPermissionDenied
					page.Status.ReasonCode = "source.denied"
				}
			}
			if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, evidence); !errors.Is(err, ErrDenied) {
				t.Fatal("changed effective policy replayed old content")
			}
		})
	}
}

func TestUnsupportedAndOutOfScopeEdgesStayTruthful(t *testing.T) {
	f := newFixture(t)
	f.request.Kind = assistant.Metrics
	f.reads.capabilityState = domain.AvailabilityUnsupported
	evidence, err := f.reader.Read(context.Background(), f.request)
	if err != nil || evidence.Bundle().RecordCount() != 0 || len(evidence.References()) != 0 || f.reads.calls[assistant.Metrics] != 0 {
		t.Fatal("unsupported fabricated data")
	}
	encoded, _ := evidence.Bundle().JSON()
	if !bytes.Contains(encoded, []byte(`"availability":"unsupported"`)) {
		t.Fatal("unsupported status lost")
	}
	f = newFixture(t)
	f.request.Kind = assistant.Topology
	outside := newRef(t, f.request.Scope.Source, "outside")
	f.reads.outside = &outside
	evidence, err = f.reader.Read(context.Background(), f.request)
	if err != nil || evidence.Bundle().RecordCount() != 0 {
		t.Fatal("edge widened scope")
	}
	encoded, _ = evidence.Bundle().JSON()
	if !bytes.Contains(encoded, []byte(`"more_available":true`)) {
		t.Fatal("scope filtering hid incomplete page")
	}
}

func TestLogLineProjectionBoundsTextAndReportsTruncation(t *testing.T) {
	for name, message := range map[string]string{"ascii": strings.Repeat("x", contextbuilder.MaximumLogLineBytes+1), "unicode": strings.Repeat("界", contextbuilder.MaximumLogLineBytes/3+1)} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = assistant.Logs
			f.request.LogConsent = true
			f.reads.logEdit = func(page *domain.LogPage) { page.Records[0].Message = message }
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := evidence.Bundle().JSON()
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Records []struct {
					Log struct {
						Message   string
						Truncated bool
					}
				}
			}
			if json.Unmarshal(encoded, &body) != nil || len(body.Records) != 1 {
				t.Fatal("invalid bounded log projection")
			}
			line := body.Records[0].Log
			if !line.Truncated || len(line.Message) > contextbuilder.MaximumLogLineBytes || len(line.Message) < contextbuilder.MaximumLogLineBytes-2 || !utf8.ValidString(line.Message) {
				t.Fatal("log line cap or truncation quality lost")
			}
		})
	}
}

func TestMalformedProviderLogPagesFailClosed(t *testing.T) {
	for name, edit := range map[string]func(*domain.LogPage){"too-many": func(p *domain.LogPage) {
		for len(p.Records) <= 10 {
			p.Records = append(p.Records, p.Records[0])
		}
	}, "outside": func(p *domain.LogPage) { p.Records[0].Timestamp = p.Status.ObservedAt.Add(time.Second) }} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.request.Kind = assistant.Logs
			f.request.LogConsent = true
			f.reads.logEdit = edit
			if evidence, err := f.reader.Read(context.Background(), f.request); err == nil || evidence.reader != nil {
				t.Fatal("invalid logs emitted")
			}
		})
	}
}
