package domain

import (
	"math"
	"testing"
	"time"
)

func TestTransportEnumsRejectUnknownAuthorityAndState(t *testing.T) {
	invalid := []interface{ Validate() error }{ProviderKind("unknown"), Scope{Kind: ScopeKind("unknown")}, HealthState("unknown_value"), EvidenceStatus("unknown_value"), Severity("unknown_value"), AlertState("unknown_value"), AlertRuleKind("unknown_value"), ComparisonOperator("unknown_value"), MetricName("unknown_value"), MetricUnit("unknown_value"), LogStreamKind("unknown_value"), LogSeverity("unknown_value"), CapabilityName("unknown_value"), AuditActor{Kind: AuditActorKind("unknown_value")}, AuditOutcome("unknown_value"), OperationAction("unknown_value"), ResourceKind("plain"), RelationshipKind("plain"), RelationshipDirection("unknown_value"), RelationshipProvenance("unknown_value"), EvidenceCertainty("unknown_value")}
	for _, value := range invalid {
		if value.Validate() == nil {
			t.Errorf("unknown closed value accepted: %T", value)
		}
	}
}

func TestMetricSamplesRejectInvalidNumericAndIdentityFields(t *testing.T) {
	sample := MetricSample{ID: "metric_safe", ResourceID: fixtureResourceRef(t, "metric").ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores, Value: .1, SampledAt: fixtureTime(), Interval: time.Second, Synthetic: true}
	for name, change := range map[string]func(*MetricSample){
		"empty id": func(s *MetricSample) { s.ID = "" }, "empty resource": func(s *MetricSample) { s.ResourceID = "" },
		"unknown metric": func(s *MetricSample) { s.Name = "plain" }, "unknown unit": func(s *MetricSample) { s.Unit = "plain" },
		"wrong unit": func(s *MetricSample) { s.Unit = MetricUnitBytes }, "nan": func(s *MetricSample) { s.Value = math.NaN() },
		"infinity": func(s *MetricSample) { s.Value = math.Inf(1) }, "unsafe number": func(s *MetricSample) { s.Value = 1 << 54 },
		"fractional bytes": func(s *MetricSample) { s.Name = MetricMemoryUsageBytes; s.Unit = MetricUnitBytes; s.Value = .5 },
		"missing clock":    func(s *MetricSample) { s.SampledAt = time.Time{} }, "negative interval": func(s *MetricSample) { s.Interval = -1 },
	} {
		t.Run(name, func(t *testing.T) { s := sample; change(&s); requireInvalid(t, s.Validate()) })
	}
}

func TestAlertRuleSafetyLimitsRejectMalformedPolicy(t *testing.T) {
	base := reviewRuleRecord().Rule
	for name, change := range map[string]func(*AlertRule){
		"rule id": func(r *AlertRule) { r.ID = "" }, "source": func(r *AlertRule) { r.SourceID = "" },
		"display": func(r *AlertRule) { r.DisplayName = "\x00" }, "kind": func(r *AlertRule) { r.Kind = "unknown" },
		"severity": func(r *AlertRule) { r.Severity = "unknown" }, "interval": func(r *AlertRule) { r.EvaluationInterval = 0 },
		"pending":         func(r *AlertRule) { r.PendingFor = -time.Second },
		"bad environment": func(r *AlertRule) { v := EnvironmentID(""); r.EnvironmentID = &v },
		"bad resource":    func(r *AlertRule) { v := ResourceID(""); r.ResourceID = &v },
	} {
		t.Run(name, func(t *testing.T) { r := base; change(&r); requireInvalid(t, r.Validate()) })
	}
}

func TestCurrentMetricResponsesCannotSmuggleDeniedOrMismatchedSamples(t *testing.T) {
	resource := fixtureResourceRef(t, "current")
	query := CurrentMetricQuery{Source: fixtureProvider(), Resources: []ResourceRef{resource}, Names: []MetricName{MetricCPUUsageCores, MetricMemoryUsageBytes}}
	first := MetricSample{ID: "metric_cpu", ResourceID: resource.ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores, Value: .2, SampledAt: fixtureTime(), Interval: time.Second, Synthetic: true}
	second := first
	second.ID = "metric_memory"
	second.Name = MetricMemoryUsageBytes
	second.Unit = MetricUnitBytes
	second.Value = 100
	base := CurrentMetricResult{Status: fixtureStatus(), Sets: []CurrentMetricSet{{ResourceID: resource.ID, Status: fixtureStatus(), Samples: []MetricSample{first, second}}}}
	if err := base.ValidateFor(query); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*CurrentMetricResult){
		"invalid receipt":      func(r *CurrentMetricResult) { r.Status.ObservedAt = time.Time{} },
		"missing complete set": func(r *CurrentMetricResult) { r.Sets = nil },
		"denied with data": func(r *CurrentMetricResult) {
			r.Status.State = AvailabilityPermissionDenied
			r.Status.ReasonCode = "provider.denied"
		},
		"unrequested set":     func(r *CurrentMetricResult) { r.Sets[0].ResourceID = fixtureResourceRef(t, "other").ID },
		"invalid set id":      func(r *CurrentMetricResult) { r.Sets[0].ResourceID = "" },
		"invalid set receipt": func(r *CurrentMetricResult) { r.Sets[0].Status.ObservedAt = time.Time{} },
		"future set":          func(r *CurrentMetricResult) { r.Sets[0].Status.ObservedAt = fixtureTime().Add(time.Second) },
		"too many samples":    func(r *CurrentMetricResult) { r.Sets[0].Samples = append(r.Sets[0].Samples, first) },
		"denied set": func(r *CurrentMetricResult) {
			r.Sets[0].Status.State = AvailabilityPermissionDenied
			r.Sets[0].Status.ReasonCode = "provider.denied"
		},
		"incomplete complete set": func(r *CurrentMetricResult) { r.Sets[0].Samples = r.Sets[0].Samples[:1] },
		"invalid sample":          func(r *CurrentMetricResult) { r.Sets[0].Samples[0].ID = "" },
		"cross set sample":        func(r *CurrentMetricResult) { r.Sets[0].Samples[0].ResourceID = fixtureResourceRef(t, "other").ID },
		"unrequested metric": func(r *CurrentMetricResult) {
			r.Sets[0].Samples[0].Name = MetricRestartCount
			r.Sets[0].Samples[0].Unit = MetricUnitCount
			r.Sets[0].Samples[0].Value = 1
		},
		"future sample":    func(r *CurrentMetricResult) { r.Sets[0].Samples[0].SampledAt = fixtureTime().Add(time.Second) },
		"duplicate metric": func(r *CurrentMetricResult) { r.Sets[0].Samples[1] = first },
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			r.Sets = append([]CurrentMetricSet(nil), base.Sets...)
			r.Sets[0].Samples = append([]MetricSample(nil), base.Sets[0].Samples...)
			change(&r)
			requireInvalid(t, r.ValidateFor(query))
		})
	}
}

func TestHealthEvidenceAndSnapshotsRejectUntrustedFields(t *testing.T) {
	evidence := HealthEvidence{ID: "evidence_probe", Code: "health.ready", SourceCode: "provider.probe", Status: EvidencePositive, ObservedAt: fixtureTime(), Synthetic: true}
	for name, change := range map[string]func(*HealthEvidence){"id": func(e *HealthEvidence) { e.ID = "" }, "code": func(e *HealthEvidence) { e.Code = "plain" }, "source": func(e *HealthEvidence) { e.SourceCode = "plain" }, "state": func(e *HealthEvidence) { e.Status = "plain" }, "clock": func(e *HealthEvidence) { e.ObservedAt = time.Time{} }} {
		t.Run(name, func(t *testing.T) { e := evidence; change(&e); requireInvalid(t, e.Validate()) })
	}
	base := HealthSnapshot{ResourceID: fixtureResourceRef(t, "probe").ID, State: HealthHealthy, Status: fixtureStatus(), Evidence: []HealthEvidence{evidence}}
	for name, change := range map[string]func(*HealthSnapshot){
		"resource": func(s *HealthSnapshot) { s.ResourceID = "" }, "state": func(s *HealthSnapshot) { s.State = "plain" },
		"receipt": func(s *HealthSnapshot) { s.Status.ObservedAt = time.Time{} },
		"denied evidence": func(s *HealthSnapshot) {
			s.State = HealthUnknown
			s.Status.State = AvailabilityPermissionDenied
			s.Status.ReasonCode = "provider.denied"
		},
		"invalid evidence": func(s *HealthSnapshot) { s.Evidence[0].ID = "" },
		"future evidence":  func(s *HealthSnapshot) { s.Evidence[0].ObservedAt = fixtureTime().Add(time.Second) },
		"over cap":         func(s *HealthSnapshot) { s.Evidence = make([]HealthEvidence, MaxHealthEvidence+1) },
	} {
		t.Run(name, func(t *testing.T) {
			s := base
			s.Evidence = append([]HealthEvidence(nil), base.Evidence...)
			change(&s)
			requireInvalid(t, s.Validate())
		})
	}
}
