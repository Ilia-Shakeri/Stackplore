package httpapi

import (
	"slices"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

func investigationSnapshot() assistantservice.Snapshot {
	snapshot := helperSnapshot()
	snapshot.Run.Goal = assistant.InvestigateIncident
	health := snapshot.Citations[0]
	health.ID, health.Kind = "ev_"+strings.Repeat("d", 64), "health"
	snapshot.Citations = append(snapshot.Citations, health)
	for _, kind := range []string{"inventory", "health", "metrics", "events", "topology", "alert_history", "workload_history"} {
		outcome := inference.ToolOutcome{Tool: kind, Availability: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete}
		if kind == "inventory" || kind == "health" {
			outcome.Availability, outcome.Records = domain.AvailabilityAvailable, 1
		}
		snapshot.Outcomes = append(snapshot.Outcomes, outcome)
	}
	snapshot.Answer.Sections[0].Citations = append(snapshot.Answer.Sections[0].Citations, health.ID)
	return snapshot
}

func TestInvestigationPublicationBindsEveryFamilyAndCoverage(t *testing.T) {
	snapshot := investigationSnapshot()
	reply, err := assistantReply(snapshot, "user_helper", helperRunID)
	if err != nil || len(reply.Citations) != 2 || len(reply.Coverage) != 7 || !slices.Equal(reply.Answer.Sections[0].Citations, snapshot.Answer.Sections[0].Citations) {
		t.Fatal("bounded multi-family publication rejected", err)
	}
	snapshot.Outcomes[0].Records = 7
	if reply.Coverage[0].Records != 1 {
		t.Fatal("coverage retained caller mutation")
	}
	for _, mutate := range []func(*assistantservice.Snapshot){
		func(s *assistantservice.Snapshot) { s.Outcomes = nil },
		func(s *assistantservice.Snapshot) { s.Outcomes[1].Records = 0 },
		func(s *assistantservice.Snapshot) { s.Outcomes[1].Tool = "shell" },
		func(s *assistantservice.Snapshot) { s.Outcomes[1].Availability = domain.AvailabilityUnsupported },
		func(s *assistantservice.Snapshot) { s.Citations[1].Kind = "log" },
		func(s *assistantservice.Snapshot) { s.Citations[1].ID = s.Citations[0].ID },
		func(s *assistantservice.Snapshot) { s.Citations[1].ObjectID = "another_resource" },
		func(s *assistantservice.Snapshot) {
			s.Answer.Sections[0].Citations = []string{"ev_" + strings.Repeat("e", 64)}
		},
		func(s *assistantservice.Snapshot) { s.Run.Goal = assistant.ExplainResource },
	} {
		changed := investigationSnapshot()
		mutate(&changed)
		if _, err := assistantReply(changed, "user_helper", helperRunID); err == nil {
			t.Fatal("forged cross-family metadata published")
		}
	}
}
