package domain

import (
	"testing"
	"time"
)

func reviewRuleRecord() AlertRuleRecord {
	return AlertRuleRecord{Rule: AlertRule{ID: "rule_review", SourceID: "source_demo", DisplayName: "Review rule", Kind: AlertRuleProviderConnectionLost, Enabled: true, Severity: SeverityError, EvaluationInterval: time.Minute}, CreatedByUserID: "user_admin", UpdatedByUserID: "user_admin", CreatedAt: fixtureTime(), UpdatedAt: fixtureTime(), Version: 1}
}

func reviewRuleAudit(record AlertRuleRecord, action AuditAction, reason ReasonCode) AuditEntry {
	return AuditEntry{ID: "audit_review", OccurredAt: record.UpdatedAt, Actor: AuditActor{Kind: AuditActorUser, ID: string(record.UpdatedByUserID)}, Action: action, Target: AuditTarget{Kind: "alert.rule", ID: string(record.Rule.ID)}, Outcome: AuditSuccess, RequestID: "request_review", ReasonCode: reason}
}

func TestRuleCommandsRequireMatchingAuditAndVersion(t *testing.T) {
	record := reviewRuleRecord()
	create := AlertRuleCreate{Record: record, Audit: reviewRuleAudit(record, "alert.rule_create", "alert.rule_created")}
	if err := create.Validate(); err != nil {
		t.Fatal(err)
	}
	create.Audit.Actor.ID = "user_other"
	requireInvalid(t, create.Validate())
	create.Audit = reviewRuleAudit(record, "alert.rule_create", "alert.rule_created")
	create.Record.SystemOwned = true
	requireInvalid(t, create.Validate())
	record.Version = 2
	record.UpdatedAt = record.UpdatedAt.Add(time.Second)
	replace := AlertRuleReplace{Record: record, ExpectedVersion: 1, Audit: reviewRuleAudit(record, "alert.rule_replace", "alert.rule_replaced")}
	if err := replace.Validate(); err != nil {
		t.Fatal(err)
	}
	replace.ExpectedVersion = 2
	requireInvalid(t, replace.Validate())
	replace.ExpectedVersion = 1
	replace.Audit.Target.ID = "rule_other"
	requireInvalid(t, replace.Validate())
	deletion := AlertRuleDelete{RuleID: record.Rule.ID, UpdatedByUserID: record.UpdatedByUserID, ExpectedVersion: 2, DeletedAt: record.UpdatedAt, Audit: reviewRuleAudit(record, "alert.rule_delete", "alert.rule_deleted")}
	if err := deletion.Validate(); err != nil {
		t.Fatal(err)
	}
	deletion.Audit.OccurredAt = fixtureTime()
	requireInvalid(t, deletion.Validate())
}

func TestRulePageRejectsDuplicateUnsortedAndForgedSystemOwnership(t *testing.T) {
	first := reviewRuleRecord()
	second := first
	second.Rule.ID = "rule_second"
	page := AlertRulePage{Rules: []AlertRuleRecord{first, second}, Page: fixturePage(2)}
	if err := page.ValidateFor(PageRequest{Limit: 2}); err != nil {
		t.Fatal(err)
	}
	page.Rules[1] = first
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	page.Rules = []AlertRuleRecord{second, first}
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	first.SystemOwned = true
	requireInvalid(t, first.Validate())
	first.CreatedByUserID = ""
	first.UpdatedByUserID = ""
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	first.Rule.SourceID = "source_docker"
	requireInvalid(t, first.Validate())
	first = reviewRuleRecord()
	first.Version = 0
	requireInvalid(t, first.Validate())
	page = AlertRulePage{Rules: []AlertRuleRecord{first}, Page: fixturePage(2)}
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	page = AlertRulePage{Rules: []AlertRuleRecord{second}, Page: fixturePage(2)}
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 1}))
}

func TestMetricHistoryCannotLeakUnrequestedOrUnorderedData(t *testing.T) {
	resource := fixtureResourceRef(t, "history")
	query := MetricHistoryQuery{Source: fixtureProvider(), Resources: []ResourceRef{resource}, Names: []MetricName{MetricCPUUsageCores}, Range: TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime()}, PointLimit: 2}
	first := MetricSample{ID: "metric_a", ResourceID: resource.ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores, Value: .2, SampledAt: fixtureTime().Add(-time.Minute), Interval: time.Second, Synthetic: true}
	second := first
	second.ID = "metric_b"
	second.SampledAt = first.SampledAt.Add(time.Second)
	base := MetricHistoryResult{Status: fixtureStatus(), Samples: []MetricSample{first, second}}
	if err := base.ValidateFor(query); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*MetricHistoryResult){
		"duplicate":      func(r *MetricHistoryResult) { r.Samples[1] = r.Samples[0] },
		"reverse":        func(r *MetricHistoryResult) { r.Samples[0], r.Samples[1] = r.Samples[1], r.Samples[0] },
		"other resource": func(r *MetricHistoryResult) { r.Samples[0].ResourceID = fixtureResourceRef(t, "other").ID },
		"other metric": func(r *MetricHistoryResult) {
			r.Samples[0].Name = MetricMemoryUsageBytes
			r.Samples[0].Unit = MetricUnitBytes
		},
		"before window":    func(r *MetricHistoryResult) { r.Samples[0].SampledAt = query.Range.Start.Add(-time.Microsecond) },
		"at exclusive end": func(r *MetricHistoryResult) { r.Samples[1].SampledAt = query.Range.End },
		"future receipt":   func(r *MetricHistoryResult) { r.Status.ObservedAt = query.Range.Start },
		"denied": func(r *MetricHistoryResult) {
			r.Status.State = AvailabilityPermissionDenied
			r.Status.ReasonCode = "provider.denied"
		},
		"invalid sample": func(r *MetricHistoryResult) { r.Samples[0].Value = -1 },
		"too many":       func(r *MetricHistoryResult) { r.Samples = append(r.Samples, second) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			result := base
			result.Samples = append([]MetricSample(nil), base.Samples...)
			change(&result)
			requireInvalid(t, result.ValidateFor(query))
		})
	}
	query.PointLimit = 0
	requireInvalid(t, base.ValidateFor(query))
	query.PointLimit = 2
	query.Range.Start = query.Range.End.Add(-MaxMetricLookback - time.Second)
	requireInvalid(t, base.ValidateFor(query))
	query.Range.Start = query.Range.End.Add(-time.Hour)
	query.Resources = nil
	requireInvalid(t, base.ValidateFor(query))
}

func TestSourceConfigurationAndCaptureRejectAuthorityChanges(t *testing.T) {
	base := SourceConfiguration{Source: InfrastructureSource{ID: "source_demo", Provider: ProviderDemo, Name: "Demo", Synthetic: true}, ProfileID: SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: fixtureTime(), UpdatedAt: fixtureTime()}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*SourceConfiguration){
		"wrong profile":      func(c *SourceConfiguration) { c.ProfileID = SourceProfileKubernetesInCluster },
		"synthetic mismatch": func(c *SourceConfiguration) { c.Source.Synthetic = false },
		"time reversed":      func(c *SourceConfiguration) { c.UpdatedAt = c.CreatedAt.Add(-time.Second) },
		"zero version":       func(c *SourceConfiguration) { c.Version = 0 },
		"non UTC":            func(c *SourceConfiguration) { c.CreatedAt = c.CreatedAt.In(time.FixedZone("offset", 3600)) },
	} {
		t.Run(name, func(t *testing.T) { c := base; change(&c); requireInvalid(t, c.Validate()) })
	}
	page := SourceConfigurationPage{Configurations: []SourceConfiguration{base}, Page: fixturePage(2)}
	if err := page.ValidateFor(PageRequest{Limit: 2}); err != nil {
		t.Fatal(err)
	}
	page.Configurations = append(page.Configurations, base)
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	page.Configurations[1].Source.ID = "source_alpha"
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	page.Configurations = []SourceConfiguration{base}
	page.Configurations[0].Version = 0
	requireInvalid(t, page.ValidateFor(PageRequest{Limit: 2}))
	token := CaptureToken{Source: fixtureProvider(), ConfigurationVersion: 1, Generation: 1, ObservedAt: fixtureTime()}
	metric := MetricSample{ID: "metric_capture", ResourceID: fixtureResourceRef(t, "capture").ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores, Value: .1, SampledAt: fixtureTime(), Interval: time.Second, Synthetic: true}
	batch := CaptureBatch{Token: token, Metrics: []MetricSample{metric}}
	if err := batch.Validate(); err != nil {
		t.Fatal(err)
	}
	batch.Metrics[0].Synthetic = false
	requireInvalid(t, batch.Validate())
	batch.Metrics[0] = metric
	batch.Metrics[0].SampledAt = fixtureTime().Add(time.Microsecond)
	requireInvalid(t, batch.Validate())
	batch.Metrics = nil
	requireInvalid(t, batch.Validate())
	token.ConfigurationVersion = 0
	requireInvalid(t, token.Validate())
}

func TestAlertEvidenceQueriesRejectCrossSourceAndBounds(t *testing.T) {
	resource := fixtureResourceRef(t, "alert-history")
	metric := AlertMetricHistoryQuery{Source: fixtureProvider(), Resource: resource, Name: MetricCPUUsageCores, Range: TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime()}, Limit: 100}
	if err := metric.Validate(); err != nil {
		t.Fatal(err)
	}
	metric.Source.SourceID = "source_other"
	requireInvalid(t, metric.Validate())
	metric.Source = fixtureProvider()
	metric.Limit = MaximumAlertMetricHistory + 1
	requireInvalid(t, metric.Validate())
	replicas := WorkloadReplicaHistoryQuery{Source: fixtureProvider(), Resource: resource, Range: metric.Range, Limit: 1}
	if err := replicas.Validate(); err != nil {
		t.Fatal(err)
	}
	replicas.Source.SourceID = "source_other"
	requireInvalid(t, replicas.Validate())
	replicas.Source = fixtureProvider()
	replicas.Limit = 0
	requireInvalid(t, replicas.Validate())
}

func TestCleanupImpactRequiresBoundedDistinctBackupScope(t *testing.T) {
	values := DefaultApplicationValues()
	if err := (ApplicationSettingsRecord{Version: 1, UpdatedAt: fixtureTime(), Values: values}).Validate(); err != nil {
		t.Fatal(err)
	}
	requireInvalid(t, (ApplicationSettingsRecord{Version: 0, UpdatedAt: fixtureTime(), Values: values}).Validate())
	for _, class := range HistoryClasses() {
		if values.RetentionHours(class) == 0 {
			t.Fatalf("missing retention for %s", class)
		}
	}
	if values.RetentionHours("unknown") != 0 {
		t.Fatal("unknown history has retention")
	}
	impact := HistoryImpact{Classes: []HistoryImpactClass{{Name: HistoryMetricSamples, Cutoff: fixtureTime(), EligibleRows: 1}}, MaximumRows: MaximumHistoryCleanupRows, BackupRequired: true, ExpiryPolicyChanged: true}
	if err := impact.Validate(); err != nil {
		t.Fatal(err)
	}
	impact.BackupRequired = false
	requireInvalid(t, impact.Validate())
	impact.BackupRequired = true
	impact.Classes = append(impact.Classes, impact.Classes[0])
	requireInvalid(t, impact.Validate())
	impact.Classes = impact.Classes[:1]
	impact.Classes[0].EligibleRows = HistoryImpactCountCap
	requireInvalid(t, impact.Validate())
	impact.Classes[0].EligibleRowsCapped = true
	if err := impact.Validate(); err != nil {
		t.Fatal(err)
	}
	impact.MaximumRows++
	requireInvalid(t, impact.Validate())
}
