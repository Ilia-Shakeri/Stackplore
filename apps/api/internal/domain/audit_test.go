package domain

import "testing"

func TestAuditPageRequiresStableNewestFirstOrder(t *testing.T) {
	request := PageRequest{Limit: 2}
	first := AuditEntry{
		ID: "audit_b", OccurredAt: fixtureTime(), Actor: AuditActor{Kind: AuditActorSystem},
		Action: "audit.test", Target: AuditTarget{Kind: "audit.target"}, Outcome: AuditSuccess,
		RequestID: "request_audit_b", ReasonCode: "audit.test_passed",
	}
	second := first
	second.ID = "audit_c"
	second.RequestID = "request_audit_c"
	page := AuditPage{Entries: []AuditEntry{first, second}, Page: PageInfo{Limit: 2}}
	if err := page.ValidateFor(request); err != nil {
		t.Fatalf("AuditPage.ValidateFor() error = %v", err)
	}
	page.Entries[0], page.Entries[1] = page.Entries[1], page.Entries[0]
	requireInvalid(t, page.ValidateFor(request))
}
