package domain

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestVisibleTextBoundsCountUnicodeCharacters(t *testing.T) {
	resource := Resource{
		Ref:         fixtureResourceRef(t, "service-unicode"),
		DisplayName: strings.Repeat("😀", maxDisplayNameLength),
	}
	if err := resource.Validate(); err != nil {
		t.Fatalf("Unicode Resource.Validate() error = %v", err)
	}
	resource.DisplayName += "😀"
	requireInvalid(t, resource.Validate())

	filter := ResourceFilter{Search: strings.Repeat("😀", MaxSearchLength)}
	if err := filter.Validate(); err != nil {
		t.Fatalf("Unicode ResourceFilter.Validate() error = %v", err)
	}
	filter.Search += "😀"
	requireInvalid(t, filter.Validate())

	record := LogRecord{
		ID: "log_unicode", ResourceID: resource.Ref.ID, Stream: LogStreamStdout,
		Severity: LogSeverityInfo, Timestamp: fixtureTime(), Message: strings.Repeat("😀", MaxLogLineLength),
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Unicode LogRecord.Validate() error = %v", err)
	}
	record.Message = strings.Repeat("x", MaxLogLineLength+1)
	requireInvalid(t, record.Validate())
}

func TestObservationStatusKeepsPartialAndUnavailableDistinct(t *testing.T) {
	partial := fixtureStatus()
	partial.Completeness = CompletenessPartial
	partial.ReasonCode = "provider.partial_read"
	if err := partial.Validate(); err != nil {
		t.Fatalf("partial status error = %v", err)
	}
	denied := fixtureStatus()
	denied.State = AvailabilityPermissionDenied
	denied.ReasonCode = "provider.permission_denied"
	if err := denied.Validate(); err != nil {
		t.Fatalf("denied status error = %v", err)
	}
	denied.Completeness = CompletenessPartial
	requireInvalid(t, denied.Validate())
	partial.ReasonCode = ""
	requireInvalid(t, partial.Validate())
	partial = fixtureStatus()
	partial.ObservedAt = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	requireInvalid(t, partial.Validate())
}

func TestHealthSnapshotRequiresEvidenceForKnownHealth(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	snapshot := HealthSnapshot{ResourceID: resource.ID, State: HealthHealthy, Status: fixtureStatus()}
	requireInvalid(t, snapshot.Validate())
	snapshot.Evidence = []HealthEvidence{{
		ID: "evidence_probe", Code: "health.probe_ready", SourceCode: "demo.ready", Status: EvidencePositive, ObservedAt: fixtureTime(),
	}}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("HealthSnapshot.Validate() error = %v", err)
	}
	snapshot.Status.State = AvailabilityUnavailable
	snapshot.Status.ReasonCode = "provider.offline"
	requireInvalid(t, snapshot.Validate())
	snapshot.State = HealthUnknown
	requireInvalid(t, snapshot.Validate())
}

func TestRelationshipCoalescesDuplicateEvidenceAndKeepsProvenance(t *testing.T) {
	first := fixtureResourceRef(t, "first").ID
	second := fixtureResourceRef(t, "second").ID
	earlier := fixtureTime().Add(-time.Minute)
	evidence := []RelationshipEvidence{
		{ID: "evidence_owner", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceOwnership, Certainty: EvidenceExplicit, Confidence: 0.95, ObservedAt: earlier, Synthetic: true},
		{ID: "evidence_owner", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceOwnership, Certainty: EvidenceExplicit, Confidence: 0.9, ObservedAt: fixtureTime(), Synthetic: true},
		{ID: "evidence_route", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceRoute, Certainty: EvidenceExplicit, Confidence: 0.7, ObservedAt: fixtureTime(), Synthetic: true},
	}
	relationship, err := NewRelationship(first, second, RelationshipOwns, RelationshipSourceToTarget, evidence)
	if err != nil {
		t.Fatalf("NewRelationship() error = %v", err)
	}
	if len(relationship.Evidence) != 2 {
		t.Fatalf("evidence count = %d, want 2", len(relationship.Evidence))
	}
	if relationship.Evidence[0].ID != "evidence_owner" || relationship.Evidence[0].Confidence != 0.9 || !relationship.Evidence[0].ObservedAt.Equal(fixtureTime()) {
		t.Fatalf("coalesced evidence = %#v", relationship.Evidence[0])
	}
	if err := relationship.Validate(); err != nil {
		t.Fatalf("Relationship.Validate() error = %v", err)
	}
}

func TestRelationshipRejectsUnsafeConfidenceAndConflictingEvidence(t *testing.T) {
	evidence := RelationshipEvidence{
		ID: "evidence_owner", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceOwnership, Certainty: EvidenceExplicit, Confidence: math.NaN(), ObservedAt: fixtureTime(),
	}
	requireInvalid(t, evidence.Validate())
	evidence.Confidence = 0.5
	other := evidence
	other.Provenance = ProvenanceRoute
	_, err := CoalesceRelationshipEvidence([]RelationshipEvidence{evidence, other})
	requireInvalid(t, err)
	other = evidence
	other.Confidence = 0.6
	_, err = CoalesceRelationshipEvidence([]RelationshipEvidence{evidence, other})
	requireInvalid(t, err)
}

func TestSymmetricRelationshipStoresCanonicalEndpoints(t *testing.T) {
	first := fixtureResourceRef(t, "first").ID
	second := fixtureResourceRef(t, "second").ID
	evidence := []RelationshipEvidence{{
		ID: "evidence_route", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceRoute,
		Certainty: EvidenceExplicit, Confidence: 1, ObservedAt: fixtureTime(), Synthetic: true,
	}}
	relationship, err := NewRelationship(second, first, RelationshipRoutesTo, RelationshipBidirectional, evidence)
	if err != nil {
		t.Fatalf("NewRelationship() error = %v", err)
	}
	if relationship.From > relationship.To {
		t.Fatalf("symmetric endpoints are not canonical: %q > %q", relationship.From, relationship.To)
	}
	relationship.From, relationship.To = relationship.To, relationship.From
	requireInvalid(t, relationship.Validate())
}

func TestMetricSampleBindsNameUnitAndFiniteValue(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	sample := MetricSample{
		ID: "metric_one", ResourceID: resource.ID, Name: MetricMemoryUsageBytes, Unit: MetricUnitBytes,
		Value: 1024, SampledAt: fixtureTime(), Interval: time.Second, Synthetic: true,
	}
	if err := sample.Validate(); err != nil {
		t.Fatalf("MetricSample.Validate() error = %v", err)
	}
	sample.Unit = MetricUnitCores
	requireInvalid(t, sample.Validate())
	sample.Unit = MetricUnitBytes
	sample.Value = math.Inf(1)
	requireInvalid(t, sample.Validate())
	sample.Value = 1.5
	requireInvalid(t, sample.Validate())
	sample.Value = 1024
	sample.Interval = 1500 * time.Millisecond
	requireInvalid(t, sample.Validate())
}

func TestEventRejectsFutureOccurrence(t *testing.T) {
	environment := EnvironmentID("env_primary")
	event := InfrastructureEvent{
		ID: "event_one", Provider: ProviderDemo, SourceID: "source_demo", EnvironmentID: &environment,
		Kind: "resource.changed", Severity: SeverityInfo, ReasonCode: "resource.updated",
		OccurredAt: fixtureTime(), ObservedAt: fixtureTime(), Synthetic: true,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("InfrastructureEvent.Validate() error = %v", err)
	}
	event.OccurredAt = event.ObservedAt.Add(time.Second)
	requireInvalid(t, event.Validate())
}

func TestAlertStateRequiresMatchingLifecycleTimes(t *testing.T) {
	alert := Alert{
		ID: "alert_one", RuleID: "rule_one", SourceID: "source_demo", State: AlertFiring,
		Severity: SeverityWarning, EvidenceRange: TimeRange{Start: fixtureTime().Add(-time.Minute), End: fixtureTime()},
		StartedAt: fixtureTime().Add(-time.Minute), UpdatedAt: fixtureTime(), SummaryCode: "alert.resource_unhealthy",
	}
	if err := alert.Validate(); err != nil {
		t.Fatalf("Alert.Validate() error = %v", err)
	}
	alert.State = AlertResolved
	requireInvalid(t, alert.Validate())
	resolvedAt := fixtureTime()
	alert.ResolvedAt = &resolvedAt
	if err := alert.Validate(); err != nil {
		t.Fatalf("resolved Alert.Validate() error = %v", err)
	}
	resolvedAt = fixtureTime().Add(-10 * time.Second)
	acknowledgedAt := fixtureTime().Add(-5 * time.Second)
	alert.ResolvedAt = &resolvedAt
	alert.AcknowledgedAt = &acknowledgedAt
	alert.EvidenceRange.End = resolvedAt
	requireInvalid(t, alert.Validate())
}

func TestAuditEntryRequiresSafeSummaryCode(t *testing.T) {
	entry := AuditEntry{
		ID: "audit_one", OccurredAt: fixtureTime(), Actor: AuditActor{Kind: AuditActorOperator, ID: "user_one"},
		Action: "source.test", Target: AuditTarget{Kind: "source.connection", ID: "source_demo"},
		Outcome: AuditSuccess, RequestID: "request_one", ReasonCode: "source.test_succeeded",
	}
	if err := entry.Validate(); err != nil {
		t.Fatalf("AuditEntry.Validate() error = %v", err)
	}
	entry.ReasonCode = ""
	requireInvalid(t, entry.Validate())
	entry.Actor = AuditActor{Kind: AuditActorSystem, ID: "user_one"}
	requireInvalid(t, entry.Validate())
}

func TestCapabilitySetRequiresEveryDistinctCapability(t *testing.T) {
	capabilities := make([]CapabilityStatus, 0, len(requiredCapabilities))
	for _, name := range requiredCapabilities {
		capabilities = append(capabilities, CapabilityStatus{Name: name, Status: fixtureStatus()})
	}
	set := CapabilitySet{Source: fixtureProvider(), Capabilities: capabilities}
	if err := set.Validate(); err != nil {
		t.Fatalf("CapabilitySet.Validate() error = %v", err)
	}
	if err := set.ValidateFor(fixtureProvider()); err != nil {
		t.Fatalf("CapabilitySet.ValidateFor() error = %v", err)
	}
	otherSource := fixtureProvider()
	otherSource.SourceID = "source_other"
	requireInvalid(t, set.ValidateFor(otherSource))
	set.Capabilities[0] = set.Capabilities[1]
	requireInvalid(t, set.Validate())
}
