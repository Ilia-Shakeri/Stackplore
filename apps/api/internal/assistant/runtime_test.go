package assistant

import (
	"testing"
	"time"
)

func validAdmission() Admission {
	at := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	return Admission{ID: "run", ConversationID: "conversation", Actor: Actor{UserID: "user", SessionDigest: [32]byte{1}, DigestKeyID: "key", AuthRevision: 1}, IdempotencyDigest: [32]byte{2}, Fingerprint: [32]byte{3}, Goal: ExplainResource, SourceID: "source", SourceVersion: 1, From: at.Add(-time.Minute), Until: at, PolicyVersion: 1, ProfileID: "profile", ProfileVersion: 1, ReservedUnits: 1, At: at, Deadline: at.Add(MaximumRunDuration)}
}

func TestRuntimeTypedStatesAndToolAllowlist(t *testing.T) {
	for _, state := range []State{Completed, Failed, Cancelled, Expired, Interrupted, Uncertain} {
		if !state.Terminal() {
			t.Fatal("terminal state rejected", state)
		}
	}
	for _, state := range []State{Queued, Running, "invented"} {
		if state.Terminal() {
			t.Fatal("nonterminal state accepted", state)
		}
	}
	for _, goal := range []Goal{ExplainResource, ExplainAlert, InvestigateIncident} {
		if !goal.Valid() {
			t.Fatal(goal)
		}
	}
	if Goal("exec").Valid() {
		t.Fatal("unknown goal accepted")
	}
	for _, kind := range []StepKind{Inventory, Health, Metrics, Events, Topology, Logs, Inference} {
		if !kind.Valid() {
			t.Fatal(kind)
		}
	}
	if StepKind("write").Valid() {
		t.Fatal("unknown tool accepted")
	}
	for _, id := range []string{"a", "source:docker_1", "run.1-profile"} {
		if !Identifier(id) {
			t.Fatal(id)
		}
	}
	for _, id := range []string{"", "path/value", "raw text", "value\n", "\x00"} {
		if Identifier(id) {
			t.Fatal("unsafe identifier accepted")
		}
	}
}

func TestAdmissionRejectsEachInvalidBoundary(t *testing.T) {
	base := validAdmission()
	if !base.Valid() {
		t.Fatal("valid admission rejected")
	}
	for i, alter := range []func(*Admission){
		func(a *Admission) { a.ID = "" }, func(a *Admission) { a.ConversationID = "" }, func(a *Admission) { a.Actor.UserID = "" }, func(a *Admission) { a.Actor.DigestKeyID = "" }, func(a *Admission) { a.Actor.SessionDigest = [32]byte{} }, func(a *Admission) { a.Actor.AuthRevision = 0 },
		func(a *Admission) { a.IdempotencyDigest = [32]byte{} }, func(a *Admission) { a.Fingerprint = [32]byte{} }, func(a *Admission) { a.Goal = "shell" }, func(a *Admission) { a.SourceID = "" }, func(a *Admission) { a.SourceVersion = 0 }, func(a *Admission) { a.ResourceID = "bad/path" },
		func(a *Admission) { a.From = time.Time{} }, func(a *Admission) { a.Until = time.Time{} }, func(a *Admission) { a.Until = a.From.Add(-time.Second) }, func(a *Admission) { a.From = a.Until.Add(-25 * time.Hour) }, func(a *Admission) { a.RestoreGeneration = -1 },
		func(a *Admission) { a.PolicyVersion = 0 }, func(a *Admission) { a.ProfileID = "" }, func(a *Admission) { a.ProfileVersion = 0 }, func(a *Admission) { a.Volatile = true; a.MemoryOwner = "" }, func(a *Admission) { a.ReservedUnits = 0 }, func(a *Admission) { a.ReservedUnits = MaximumReservationUnits + 1 },
		func(a *Admission) { a.At = time.Time{} }, func(a *Admission) { a.Deadline = time.Time{} }, func(a *Admission) { a.Deadline = a.At }, func(a *Admission) { a.Deadline = a.At.Add(MaximumRunDuration + time.Microsecond) },
	} {
		a := base
		alter(&a)
		if a.Valid() {
			t.Fatal("invalid admission accepted", i)
		}
	}
	base.Volatile = true
	base.MemoryOwner = "process"
	base.ResourceID = "resource"
	if !base.Valid() {
		t.Fatal("bounded volatile request rejected")
	}
}

func TestSettingsLeaseAuditAndTimestampBoundaries(t *testing.T) {
	s := Settings{Version: 1, PolicyVersion: 1, ProfileID: "profile", ProfileVersion: 1, UserQuota: 1, InstallationQuota: 1}
	if !s.Valid() {
		t.Fatal("settings rejected")
	}
	for _, alter := range []func(*Settings){func(s *Settings) { s.Version = 0 }, func(s *Settings) { s.PolicyVersion = 0 }, func(s *Settings) { s.ProfileID = "" }, func(s *Settings) { s.ProfileVersion = 0 }, func(s *Settings) { s.UserQuota = 0 }, func(s *Settings) { s.UserQuota = MaximumQuotaUnits + 1 }, func(s *Settings) { s.InstallationQuota = 0 }, func(s *Settings) { s.InstallationQuota = MaximumQuotaUnits + 1 }} {
		copy := s
		alter(&copy)
		if copy.Valid() {
			t.Fatal("invalid settings accepted")
		}
	}
	at := validAdmission().At
	if Timestamp(time.Time{}) || Timestamp(at.Add(time.Nanosecond)) || Timestamp(time.UnixMicro(-1)) || Timestamp(at.In(time.FixedZone("offset", 3600))) || Timestamp(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("invalid timestamp accepted")
	}
	a := OperatorAudit{ID: "id", OperatorID: "operator", RequestID: "request", At: at}
	if !a.Valid() {
		t.Fatal("audit rejected")
	}
	for _, alter := range []func(*OperatorAudit){func(a *OperatorAudit) { a.ID = "" }, func(a *OperatorAudit) { a.OperatorID = "" }, func(a *OperatorAudit) { a.RequestID = "" }, func(a *OperatorAudit) { a.At = time.Time{} }} {
		copy := a
		alter(&copy)
		if copy.Valid() {
			t.Fatal("invalid audit accepted")
		}
	}
	l := Lease{RunID: "run", Worker: "worker", Version: 1, Fence: 1, At: at}
	if !l.Valid() {
		t.Fatal("lease rejected")
	}
	for _, alter := range []func(*Lease){func(l *Lease) { l.RunID = "" }, func(l *Lease) { l.Worker = "" }, func(l *Lease) { l.Version = 0 }, func(l *Lease) { l.Fence = 0 }, func(l *Lease) { l.At = time.Time{} }} {
		copy := l
		alter(&copy)
		if copy.Valid() {
			t.Fatal("invalid lease accepted")
		}
	}
}
