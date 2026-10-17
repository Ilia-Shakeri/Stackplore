package domain

import (
	"testing"
	"time"
)

func TestApplicationPolicyRejectsUnsafeRetentionAndCleanupClasses(t *testing.T) {
	baseline := DefaultApplicationValues()
	if baseline.Validate() != nil {
		t.Fatal("default policy invalid")
	}
	for _, mutate := range []func(*ApplicationValues){
		func(v *ApplicationValues) { v.MetricRetentionHours = 0 },
		func(v *ApplicationValues) { v.HealthRetentionHours = 8761 },
		func(v *ApplicationValues) { v.LogLineLimit = 201 },
		func(v *ApplicationValues) { v.DefaultAlertPendingSeconds = -1 },
	} {
		value := baseline
		mutate(&value)
		if value.Validate() == nil {
			t.Fatal("unsafe policy accepted")
		}
	}
	for _, classes := range [][]HistoryClass{nil, {"audit_entries"}, {HistoryMetricSamples, HistoryMetricSamples}, {"users"}, {"resources"}, {"sessions"}} {
		if ValidateHistoryClasses(classes) == nil {
			t.Fatal("protected or duplicate cleanup class accepted")
		}
	}
	policy := AuditLifecyclePolicy{RetentionDays: 89, QuotaRows: 1000}
	if policy.Validate() == nil {
		t.Fatal("audit minimum bypassed")
	}
	policy.RetentionDays = 90
	policy.ArchiveVerifiedThrough = time.Now()
	if policy.Validate() == nil {
		t.Fatal("noncanonical archive clock accepted")
	}
}
