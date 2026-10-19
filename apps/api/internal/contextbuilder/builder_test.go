package contextbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

func requestFixture(t *testing.T) Request {
	t.Helper()
	at := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_private_identifier"}
	resource := func(native string) domain.ResourceRef {
		identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "private_environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "private_scope_name"}, domain.NativeResourceID(native))
		if err != nil {
			t.Fatal(err)
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	first, second := resource("private_native_first"), resource("private_native_second")
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	relationship, err := domain.NewRelationship(first.ID, second.ID, domain.RelationshipMemberOf, domain.RelationshipUndirected, []domain.RelationshipEvidence{{ID: "private_relationship_evidence", Provider: source.Provider, SourceID: source.SourceID, Provenance: domain.ProvenanceNetworkMembership, Certainty: domain.EvidenceInferred, Confidence: .5, ObservedAt: at, Synthetic: true}})
	if err != nil {
		t.Fatal(err)
	}
	return Request{
		Scope:       Scope{RunID: "run_sample", Source: source, ConfigurationVersion: 3, PolicyVersion: 2, RestoreGeneration: 0, Range: domain.TimeRange{Start: at.Add(-5 * time.Minute), End: at}, ObservedAt: at, Resources: []domain.ResourceRef{first, second}},
		Destination: Remote, Status: status,
		Resources:     []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: first, DisplayName: "private_display_name", Version: "private_version", Synthetic: true}, Status: status, Health: domain.HealthHealthy}},
		Health:        []domain.HealthSnapshot{{ResourceID: first.ID, State: domain.HealthHealthy, Status: status, Evidence: []domain.HealthEvidence{{ID: "private_health_id", Code: "private.health_code", SourceCode: "private.source_code", Status: domain.EvidencePositive, ObservedAt: at, Synthetic: true}}}},
		Metrics:       []domain.MetricSample{{ID: "private_metric_id", ResourceID: first.ID, Name: domain.MetricMemoryUsageBytes, Unit: domain.MetricUnitBytes, Value: 1024, SampledAt: at.Add(-time.Second), Interval: time.Second, Synthetic: true}},
		Events:        []domain.InfrastructureEvent{{ID: "private_event_id", Provider: source.Provider, SourceID: source.SourceID, ResourceID: &first.ID, Kind: "private.event_kind", Severity: domain.SeverityWarning, ReasonCode: "private.event_reason", Summary: "private_event_prose", OccurredAt: at.Add(-time.Second), ObservedAt: at, Synthetic: true}},
		Relationships: []domain.Relationship{relationship},
	}
}

func TestRemoteProjectionDropsProseAndPreservesTypedQualityAndReferences(t *testing.T) {
	request := requestFixture(t)
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bundle.JSON()
	if err != nil || !json.Valid(encoded) || bundle.RecordCount() != 5 || len(bundle.Citations()) != 5 {
		t.Fatal("invalid bounded bundle")
	}
	if bytes.Contains(encoded, []byte("private_")) || bytes.Contains(encoded, []byte("private.")) {
		t.Fatal("arbitrary source/name/native ID/version/prose escaped projection")
	}
	if !bytes.Contains(encoded, []byte(`"restore_generation":0`)) || !bytes.Contains(encoded, []byte(`"synthetic":true`)) || !bytes.Contains(encoded, []byte(`"name":"memory.usage.bytes"`)) {
		t.Fatal("typed source/time/quality facts missing")
	}
	for _, citation := range bundle.Citations() {
		if citation.Source != request.Scope.Source || citation.ConfigurationVersion != 3 || citation.PolicyVersion != 2 || citation.RestoreGeneration != 0 || citation.ObservedAt.IsZero() || citation.ID == "" || citation.ObjectID == "" || citation.Range != request.Scope.Range {
			t.Fatal("citation lost exact authority metadata")
		}
	}
	citations := bundle.Citations()
	citations[0].ID = "changed"
	if bundle.Citations()[0].ID == "changed" {
		t.Fatal("reference state was mutable")
	}
	for _, value := range []any{request, bundle} {
		if strings.Contains(fmt.Sprintf("%+v", value), "private_") {
			t.Fatal("formatted value exposed context")
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != "{}" {
			t.Fatal("ordinary logging serialized context")
		}
	}
}

func TestRemoteHandlesAreRunScopedWhileLocalCitationsKeepExactIdentity(t *testing.T) {
	firstRequest := requestFixture(t)
	first, err := Build(context.Background(), firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := Build(context.Background(), firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := first.JSON()
	repeatJSON, _ := repeat.JSON()
	if !bytes.Equal(firstJSON, repeatJSON) {
		t.Fatal("same run changed its opaque references")
	}
	secondRequest := requestFixture(t)
	secondRequest.Scope.RunID = "other_server_run"
	second, err := Build(context.Background(), secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if first.document.SourceReference == second.document.SourceReference || first.document.Records[0].ResourceID == second.document.Records[0].ResourceID || first.document.Records[0].VersionReference == second.document.Records[0].VersionReference {
		t.Fatal("source/resource/version reference correlated two runs")
	}
	for index, item := range first.document.Records {
		other := second.document.Records[index]
		if item.CitationID == other.CitationID {
			t.Fatal("event or relationship citation reused another run's handle")
		}
		if item.Relationship != nil && (item.Relationship.From == other.Relationship.From || item.Relationship.To == other.Relationship.To) {
			t.Fatal("relationship endpoints correlated two runs")
		}
	}
	for _, resource := range firstRequest.Scope.Resources {
		if bytes.Contains(firstJSON, []byte(resource.ID)) {
			t.Fatal("stable estate resource ID escaped remote projection")
		}
	}
	if first.document.Records[0].ResourceID != first.document.Records[3].ResourceID {
		t.Fatal("event lost its within-run resource relation")
	}
	for index, citation := range first.Citations() {
		if citation.Source != firstRequest.Scope.Source || citation.Range != firstRequest.Scope.Range {
			t.Fatal("local reference lost source/window")
		}
		if index < 4 && citation.ResourceID != firstRequest.Scope.Resources[0].ID {
			t.Fatal("remote alias overwrote local resource identity")
		}
	}
	if first.Citations()[3].ObjectID != string(firstRequest.Events[0].ID) || first.Citations()[4].ObjectID != string(firstRequest.Relationships[0].ID) {
		t.Fatal("local event or relationship lookup lost exact identity")
	}
	firstRequest.Destination = Local
	local, err := Build(context.Background(), firstRequest)
	if err != nil || local.document.Records[0].ResourceID != string(firstRequest.Scope.Resources[0].ID) {
		t.Fatal("local normalized resource identity changed")
	}
}

func TestCitationsBindContentPolicyRestoreAndWindow(t *testing.T) {
	request := requestFixture(t)
	original, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := Build(context.Background(), request)
	if err != nil || original.Citations()[0].ID != repeat.Citations()[0].ID {
		t.Fatal("unchanged evidence was not stable")
	}
	for name, edit := range map[string]func(*Request){
		"policy":           func(r *Request) { r.Scope.PolicyVersion++ },
		"restore":          func(r *Request) { r.Scope.RestoreGeneration++ },
		"window":           func(r *Request) { r.Scope.Range.Start = r.Scope.Range.Start.Add(-time.Second) },
		"resource-version": func(r *Request) { r.Resources[0].Resource.Version = "changed_version" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := requestFixture(t)
			edit(&changed)
			bundle, err := Build(context.Background(), changed)
			if err != nil || bundle.Citations()[0].ID == original.Citations()[0].ID {
				t.Fatal("changed evidence reused citation")
			}
		})
	}
	changed := requestFixture(t)
	changed.Metrics[0].Value++
	bundle, err := Build(context.Background(), changed)
	if err != nil || bundle.Citations()[2].ID == original.Citations()[2].ID {
		t.Fatal("metric update reused citation")
	}
}

func TestProjectionRejectsCrossScopeMalformedAndFutureEvidence(t *testing.T) {
	for name, edit := range map[string]func(*Request){
		"restore-negative":    func(r *Request) { r.Scope.RestoreGeneration = -1 },
		"source-version":      func(r *Request) { r.Scope.ConfigurationVersion = 0 },
		"policy-version":      func(r *Request) { r.Scope.PolicyVersion = 0 },
		"duplicate-scope":     func(r *Request) { r.Scope.Resources = append(r.Scope.Resources, r.Scope.Resources[0]) },
		"scope-source":        func(r *Request) { r.Scope.Source.SourceID = "other_source" },
		"resource-synthetic":  func(r *Request) { r.Resources[0].Resource.Synthetic = false },
		"health-outside":      func(r *Request) { r.Health[0].ResourceID = "r1_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX" },
		"metric-not-finite":   func(r *Request) { r.Metrics[0].Value = math.NaN() },
		"metric-outside-time": func(r *Request) { r.Metrics[0].SampledAt = r.Scope.Range.Start.Add(-time.Microsecond) },
		"event-source":        func(r *Request) { r.Events[0].SourceID = "other_source" },
		"event-unscoped":      func(r *Request) { r.Events[0].ResourceID = nil },
		"event-environment": func(r *Request) {
			other := domain.EnvironmentID("other_environment")
			r.Events[0].EnvironmentID = &other
		},
		"event-future":        func(r *Request) { r.Events[0].ObservedAt = r.Scope.ObservedAt.Add(time.Second) },
		"relationship-source": func(r *Request) { r.Relationships[0].Evidence[0].SourceID = "other_source" },
		"microseconds":        func(r *Request) { r.Status.ObservedAt = r.Status.ObservedAt.Add(time.Nanosecond) },
		"duplicate-event":     func(r *Request) { r.Events = append(r.Events, r.Events[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			request := requestFixture(t)
			edit(&request)
			bundle, err := Build(context.Background(), request)
			if !errors.Is(err, ErrInvalid) || bundle.RecordCount() != 0 {
				t.Fatalf("invalid evidence accepted: %v", err)
			}
		})
	}
}

func TestDenialPartialAndUnavailableRemainDistinct(t *testing.T) {
	request := requestFixture(t)
	request.Status.State, request.Status.ReasonCode = domain.AvailabilityPermissionDenied, "source.permission_denied"
	if bundle, err := Build(context.Background(), request); !errors.Is(err, ErrDenied) || bundle.RecordCount() != 0 {
		t.Fatal("denied source emitted context")
	}
	request = requestFixture(t)
	request.Health[0].Status = domain.ObservationStatus{State: domain.AvailabilityPermissionDenied, Completeness: domain.CompletenessComplete, ObservedAt: request.Scope.ObservedAt, ReasonCode: "source.permission_denied"}
	request.Health[0].State, request.Health[0].Evidence = domain.HealthUnknown, nil
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrDenied) {
		t.Fatal("denied health emitted a bundle")
	}
	request = requestFixture(t)
	request.Status.State, request.Status.Completeness, request.Status.ReasonCode = domain.AvailabilityStale, domain.CompletenessPartial, "source.partial"
	request.Resources[0].Status = request.Status
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := bundle.JSON()
	if !bytes.Contains(data, []byte(`"availability":"stale"`)) || bundle.document.Records[0].Health != domain.HealthUnknown {
		t.Fatal("partial evidence invented health")
	}
	request.Resources, request.Health, request.Metrics, request.Events, request.Relationships = nil, nil, nil, nil, nil
	request.Status = domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: request.Scope.ObservedAt, ReasonCode: "source.unsupported"}
	bundle, err = Build(context.Background(), request)
	if err != nil || bundle.RecordCount() != 0 || bundle.document.Quality.Availability != domain.AvailabilityUnsupported {
		t.Fatal("unsupported metadata was fabricated or hidden")
	}
}

func TestExactRecordByteAndCancellationBounds(t *testing.T) {
	request := requestFixture(t)
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := bundle.JSON()
	request.ByteLimit = len(encoded)
	if _, err := Build(context.Background(), request); err != nil {
		t.Fatal("exact byte cap rejected")
	}
	request.ByteLimit--
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrBudget) {
		t.Fatal("byte cap bypassed")
	}
	request = requestFixture(t)
	request.RecordLimit = 4
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrBudget) {
		t.Fatal("record cap bypassed")
	}
	request.RecordLimit = MaximumRecords + 1
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("caller expanded hard cap")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, requestFixture(t)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled projection survived")
	}
}

type logProvider struct{ page domain.LogPage }

func (provider logProvider) ReadLogs(_ context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	page := provider.page
	page.Page.Limit = query.LineLimit
	return page, nil
}

func captureFixture(t *testing.T, request Request, edit func(*domain.LogReadQuery, *domain.LogPage)) logpolicy.Capture {
	t.Helper()
	query := domain.LogReadQuery{Source: request.Scope.Source, Targets: []domain.LogTarget{{Resource: request.Scope.Resources[0]}}, Range: request.Scope.Range, LineLimit: 20, ByteLimit: 1024}
	page := domain.LogPage{Status: request.Status, Page: domain.PageInfo{Limit: 20}, Records: []domain.LogRecord{{ID: "local_log", ResourceID: query.Targets[0].Resource.ID, Stream: domain.LogStreamStdout, Sequence: 1, Severity: domain.LogSeverityInfo, Timestamp: query.Range.End.Add(-time.Second), Message: "customer_id=synthetic_marker done", Synthetic: true}}}
	if edit != nil {
		edit(&query, &page)
	}
	policy, err := logsafe.NewPolicy([]string{"customer_id"})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := (logpolicy.Reader{Provider: logProvider{page: page}, Redactor: policy}).Capture(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	return capture
}

func TestLocalLogsRequireConsentPolicyCaptureAndExactScope(t *testing.T) {
	request := requestFixture(t)
	capture := captureFixture(t, request, nil)
	request.Logs, request.LogConsent = &capture, true
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrDenied) {
		t.Fatal("remote logs accepted")
	}
	request.Destination, request.LogConsent = Local, false
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrDenied) {
		t.Fatal("local logs skipped consent")
	}
	request.LogConsent = true
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := bundle.JSON()
	if bytes.Contains(encoded, []byte("synthetic_marker")) || !bytes.Contains(encoded, []byte("[redacted]")) {
		t.Fatal("shared redaction bypassed")
	}
	request.Logs = &logpolicy.Capture{}
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("unsealed capture accepted")
	}
	request.Logs = &capture
	request.Scope.Range.Start = request.Scope.Range.Start.Add(time.Minute)
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("capture escaped selected time scope")
	}
	request = requestFixture(t)
	request.Destination, request.LogConsent, request.Logs = Local, true, &capture
	request.Scope.Resources = request.Scope.Resources[1:]
	request.Resources, request.Health, request.Metrics, request.Events, request.Relationships = nil, nil, nil, nil, nil
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("capture escaped resource scope")
	}
}

func TestLocalLogCapsAndUTF8Truncation(t *testing.T) {
	request := requestFixture(t)
	request.Destination, request.LogConsent = Local, true
	capture := captureFixture(t, request, func(q *domain.LogReadQuery, p *domain.LogPage) {
		q.ByteLimit = MaximumLogBytes
		p.Records[0].Message = strings.Repeat("ژ", 3000)
	})
	request.Logs = &capture
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	line := bundle.document.Records[len(bundle.document.Records)-1].Log
	if len(line.Message) > MaximumLogLineBytes || !utf8.ValidString(line.Message) || !line.Truncated {
		t.Fatal("local line exceeded bounded UTF-8 cap")
	}
	capture = captureFixture(t, request, func(q *domain.LogReadQuery, _ *domain.LogPage) { q.ByteLimit = MaximumLogBytes + 1 })
	request.Logs = &capture
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("capture byte cap bypassed")
	}
	request = requestFixture(t)
	request.Destination, request.LogConsent = Local, true
	request.Scope.Range.Start = request.Scope.Range.End.Add(-20 * time.Minute)
	capture = captureFixture(t, request, nil)
	request.Logs = &capture
	if _, err := Build(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatal("capture exceeded local time cap")
	}
	request = requestFixture(t)
	request.Destination, request.LogConsent = Local, true
	capture = captureFixture(t, request, func(_ *domain.LogReadQuery, p *domain.LogPage) { p.Page.HasMore = true; p.Page.NextCursor = "bmV4dA" })
	request.Logs = &capture
	bundle, err = Build(context.Background(), request)
	if err != nil || !bundle.document.MoreAvailable {
		t.Fatal("bounded log page hid remaining data")
	}
}
