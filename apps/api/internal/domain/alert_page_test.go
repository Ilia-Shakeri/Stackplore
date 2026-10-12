package domain

import (
	"testing"
	"time"
)

func TestAlertPageRequiresStableNewestFirstOrder(t *testing.T) {
	now := fixtureTime()
	alerts := []Alert{
		validPageAlert("alert_a", now),
		validPageAlert("alert_b", now.Add(-time.Minute)),
	}
	page := AlertPage{Alerts: alerts, Page: PageInfo{Limit: 10}}
	if err := page.ValidateFor("source_demo", PageRequest{Limit: 10}); err != nil {
		t.Fatal(err)
	}
	page.Alerts[0], page.Alerts[1] = page.Alerts[1], page.Alerts[0]
	if page.ValidateFor("source_demo", PageRequest{Limit: 10}) == nil {
		t.Fatal("oldest-first alert page was accepted")
	}
}

func validPageAlert(id AlertID, updatedAt time.Time) Alert {
	return Alert{
		ID: id, RuleID: "rule_one", SourceID: "source_demo", State: AlertFiring,
		Severity:      SeverityError,
		EvidenceRange: TimeRange{Start: updatedAt.Add(-time.Minute), End: updatedAt},
		StartedAt:     updatedAt.Add(-time.Minute), UpdatedAt: updatedAt,
		SummaryCode: "alert.resource_unhealthy", Synthetic: true,
	}
}
