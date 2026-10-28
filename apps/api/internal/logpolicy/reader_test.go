package logpolicy_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

type provider struct {
	page      domain.LogPage
	calls     int
	limit     int
	afterRead func()
}

func (p *provider) ReadLogs(_ context.Context, q domain.LogReadQuery) (domain.LogPage, error) {
	p.calls++
	p.limit = q.LineLimit
	if p.afterRead != nil {
		p.afterRead()
	}
	page := p.page
	page.Page.Limit = q.LineLimit
	return page, nil
}

type settings struct {
	record domain.ApplicationSettingsRecord
	err    error
}

func (s *settings) Get(context.Context) (domain.ApplicationSettingsRecord, error) {
	return s.record, s.err
}

func fixture(t *testing.T) (domain.LogReadQuery, *provider) {
	t.Helper()
	at := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	identity, err := domain.NewResourceIdentity(domain.ProviderDemo, "source_demo", "environment_demo", "demo.service", domain.Scope{Kind: domain.ScopeGlobal}, "sample")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	q := domain.LogReadQuery{Source: domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}, Targets: []domain.LogTarget{{Resource: ref}}, Range: domain.TimeRange{Start: at.Add(-time.Minute), End: at}, LineLimit: 100, ByteLimit: 1024}
	p := &provider{page: domain.LogPage{Status: domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}, Page: domain.PageInfo{Limit: q.LineLimit}, Records: []domain.LogRecord{{ID: "log_sample", ResourceID: ref.ID, Stream: domain.LogStreamStdout, Severity: domain.LogSeverityInfo, Timestamp: at.Add(-time.Second), Message: "ordinary sample", Sequence: 1, Synthetic: true}}}}
	return q, p
}

func TestEffectiveSettingsReloadAndSettingsFailureStopsProvider(t *testing.T) {
	q, p := fixture(t)
	values := domain.DefaultApplicationValues()
	values.LogLineLimit = 10
	s := &settings{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: q.Range.End, Values: values}}
	r := logpolicy.Reader{Provider: p, Settings: s}
	if _, err := r.ReadLogs(context.Background(), q); err != nil || p.limit != 10 {
		t.Fatal("effective line limit missing")
	}
	s.record.Values.LogLineLimit = 20
	if _, err := r.ReadLogs(context.Background(), q); err != nil || p.limit != 20 {
		t.Fatal("changed line limit not reloaded")
	}
	s.err = errors.New("synthetic settings outage")
	if _, err := r.ReadLogs(context.Background(), q); !errors.Is(err, domain.ErrRepositoryUnavailable) || p.calls != 2 {
		t.Fatal("settings failure reached provider")
	}
	s.err = nil
	s.record.Version = 0
	if _, err := r.ReadLogs(context.Background(), q); !errors.Is(err, domain.ErrRepositoryUnavailable) || p.calls != 2 {
		t.Fatal("invalid settings reached provider")
	}
}

func TestAdditionalFieldsPreserveProviderDataAndUTF8ByteBudget(t *testing.T) {
	q, p := fixture(t)
	p.page.Records[0].Message = "customer_id=sample_value متن"
	original := p.page.Records[0].Message
	policy, err := logsafe.NewPolicy([]string{"customer_id"})
	if err != nil {
		t.Fatal(err)
	}
	q.ByteLimit = 15
	page, err := (logpolicy.Reader{Provider: p, Redactor: policy}).ReadLogs(context.Background(), q)
	if err != nil || len(page.Records[0].Message) > 15 || !utf8.ValidString(page.Records[0].Message) || !page.Records[0].Redacted || !page.Records[0].Truncated || strings.Contains(page.Records[0].Message, "sample_value") || p.page.Records[0].Message != original {
		t.Fatal("additive policy or immutable byte cap failed")
	}
}

func TestCaptureReappliesBuiltInMaskingAndSealsCopies(t *testing.T) {
	q, p := fixture(t)
	p.page.Records[0].Message = "password=synthetic_marker done"
	r := logpolicy.Reader{Provider: p, Redactor: logsafe.Policy{}}
	page, err := r.ReadLogs(context.Background(), q)
	if err != nil || page.Records[0].Redacted {
		t.Fatal("transport default behavior changed")
	}
	capture, err := r.Capture(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	query, page, valid := capture.Snapshot()
	if !valid || strings.Contains(page.Records[0].Message, "synthetic_marker") || !page.Records[0].Redacted {
		t.Fatal("capture skipped built-in masking")
	}
	query.Targets[0].Container = "changed"
	page.Records[0].Message = "changed"
	query, page, _ = capture.Snapshot()
	if query.Targets[0].Container != "" || page.Records[0].Message == "changed" {
		t.Fatal("capture exposed mutable state")
	}
	if strings.Contains(fmt.Sprintf("%+v", capture), "done") {
		t.Fatal("formatted capture exposed text")
	}
	encoded, err := json.Marshal(capture)
	if err != nil || string(encoded) != "{}" {
		t.Fatal("structured logging exposed capture")
	}
}

func TestCaptureAndCancellationFailClosed(t *testing.T) {
	q, p := fixture(t)
	r := logpolicy.Reader{Provider: p, Redactor: logsafe.Policy{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Capture(ctx, q); !errors.Is(err, context.Canceled) || p.calls != 0 {
		t.Fatal("cancelled capture reached provider")
	}
	q.ByteLimit = 0
	if _, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) || p.calls != 0 {
		t.Fatal("invalid query reached provider")
	}
	q.ByteLimit = 1024
	p.page.Records[0].Timestamp = q.Range.End
	if _, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) {
		t.Fatal("invalid provider page accepted")
	}
	if _, err := (logpolicy.Reader{Provider: p}).Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) {
		t.Fatal("unconfigured masking accepted")
	}
}

func TestLateProviderResponseAndOversizedPagesAreRejected(t *testing.T) {
	q, p := fixture(t)
	r := logpolicy.Reader{Provider: p, Redactor: logsafe.Policy{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.afterRead = cancel
	if _, err := r.Capture(ctx, q); !errors.Is(err, context.Canceled) {
		t.Fatal("late cancelled page accepted")
	}
	p.afterRead = nil
	p.page.Records[0].Message = strings.Repeat("x", domain.MaxLogLineBytes+1)
	if _, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) {
		t.Fatal("oversized line reached redaction")
	}
	p.page.Records[0].Message = "ordinary sample"
	p.page.Records = append(p.page.Records, p.page.Records[0])
	q.LineLimit = 1
	if _, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) {
		t.Fatal("oversized record page accepted")
	}
}

func TestCaptureFingerprintTracksEffectivePolicyAndSettings(t *testing.T) {
	for name, edit := range map[string]func(*domain.LogReadQuery, *settings, *logpolicy.Reader){
		"settings-version":    func(_ *domain.LogReadQuery, s *settings, _ *logpolicy.Reader) { s.record.Version++ },
		"settings-line-limit": func(_ *domain.LogReadQuery, s *settings, _ *logpolicy.Reader) { s.record.Values.LogLineLimit++ },
		"query-line-limit":    func(q *domain.LogReadQuery, _ *settings, _ *logpolicy.Reader) { q.LineLimit = 5 },
		"query-byte-limit":    func(q *domain.LogReadQuery, _ *settings, _ *logpolicy.Reader) { q.ByteLimit-- },
		"query-window": func(q *domain.LogReadQuery, _ *settings, _ *logpolicy.Reader) {
			q.Range.Start = q.Range.Start.Add(-time.Minute)
		},
		"added-mask-field": func(_ *domain.LogReadQuery, _ *settings, r *logpolicy.Reader) {
			policy, err := logsafe.NewPolicy([]string{"member_ref"})
			if err != nil {
				t.Fatal(err)
			}
			r.Redactor = policy
		},
	} {
		t.Run(name, func(t *testing.T) {
			q, p := fixture(t)
			values := domain.DefaultApplicationValues()
			values.LogLineLimit = 10
			s := &settings{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: q.Range.End, Values: values}}
			r := logpolicy.Reader{Provider: p, Settings: s, Redactor: logsafe.Policy{}}
			first, err := r.Capture(context.Background(), q)
			if err != nil || first.PolicyFingerprint() == ([32]byte{}) {
				t.Fatal("valid policy identity missing")
			}
			edit(&q, s, &r)
			changed, err := r.Capture(context.Background(), q)
			if err != nil || changed.PolicyFingerprint() == first.PolicyFingerprint() {
				t.Fatal("effective policy change reused a sealed identity")
			}
		})
	}
}

func TestCaptureFingerprintIgnoresContentIdentityAndClippedRequestedLimit(t *testing.T) {
	q, p := fixture(t)
	values := domain.DefaultApplicationValues()
	values.LogLineLimit = 10
	s := &settings{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: q.Range.End, Values: values}}
	r := logpolicy.Reader{Provider: p, Settings: s, Redactor: logsafe.Policy{}}
	first, err := r.Capture(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	q.LineLimit = 20 // Both requests have the same effective ten-line policy.
	q.Targets[0].Container, p.page.Records[0].Container = "different_container", "different_container"
	p.page.Records[0].Message = "different ordinary observation"
	q.Range.Start, q.Range.End = q.Range.Start.Add(time.Minute), q.Range.End.Add(time.Minute)
	p.page.Status.ObservedAt, p.page.Records[0].Timestamp = p.page.Status.ObservedAt.Add(time.Minute), p.page.Records[0].Timestamp.Add(time.Minute)
	other, err := r.Capture(context.Background(), q)
	if err != nil || first.PolicyFingerprint() != other.PolicyFingerprint() {
		t.Fatal("content, target or fixed timestamps contaminated policy identity")
	}
	digest := first.PolicyFingerprint()
	digest[0]++
	if first.PolicyFingerprint() != other.PolicyFingerprint() {
		t.Fatal("returned digest mutated a sealed capture")
	}
}

func TestCaptureFingerprintUsesSettingsSnapshotActuallyAppliedBeforeRead(t *testing.T) {
	q, p := fixture(t)
	values := domain.DefaultApplicationValues()
	values.LogLineLimit = 10
	s := &settings{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: q.Range.End, Values: values}}
	r := logpolicy.Reader{Provider: p, Settings: s, Redactor: logsafe.Policy{}}
	baseline, err := r.Capture(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	p.afterRead = func() {
		s.record.Version = 2
		s.record.Values.LogLineLimit = 20
	}
	oldSnapshot, err := r.Capture(context.Background(), q)
	if err != nil || oldSnapshot.PolicyFingerprint() != baseline.PolicyFingerprint() || p.limit != 10 {
		t.Fatal("capture claimed a later settings version that did not govern its read")
	}
	p.afterRead = nil
	newSnapshot, err := r.Capture(context.Background(), q)
	if err != nil || newSnapshot.PolicyFingerprint() == baseline.PolicyFingerprint() || p.limit != 20 {
		t.Fatal("following capture failed to reload changed policy")
	}
}

type mutableFingerprintRedactor struct{ digest [32]byte }

func (*mutableFingerprintRedactor) HasAdditionalFields() bool            { return false }
func (redactor *mutableFingerprintRedactor) PolicyFingerprint() [32]byte { return redactor.digest }
func (*mutableFingerprintRedactor) Message(value string, truncated bool) (string, bool, bool) {
	return logsafe.Message(value, truncated)
}

func TestCaptureFingerprintRejectsUnidentifiedOrChangedMaskingPolicy(t *testing.T) {
	if (logpolicy.Capture{}).PolicyFingerprint() != ([32]byte{}) {
		t.Fatal("zero capture claimed a policy identity")
	}
	q, p := fixture(t)
	redactor := &mutableFingerprintRedactor{}
	r := logpolicy.Reader{Provider: p, Redactor: redactor}
	if capture, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) || capture.PolicyFingerprint() != ([32]byte{}) || p.calls != 0 {
		t.Fatal("unidentified policy reached the provider or produced a sealed result")
	}
	redactor.digest = (logsafe.Policy{}).PolicyFingerprint()
	p.afterRead = func() { redactor.digest[0]++ }
	if capture, err := r.Capture(context.Background(), q); !errors.Is(err, logpolicy.ErrInvalidCapture) || capture.PolicyFingerprint() != ([32]byte{}) {
		t.Fatal("capture kept evidence after masking policy changed during the read")
	}
}
