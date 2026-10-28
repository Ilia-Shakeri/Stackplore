package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

const helperRunID = "ar_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const helperCitationID = "ev_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func helperSnapshot() assistantservice.Snapshot {
	at := time.Date(2026, 10, 28, 10, 15, 0, 1000, time.UTC)
	resource := "r1_" + strings.Repeat("A", 43)
	run := assistant.Run{ID: helperRunID, ConversationID: "ac_" + strings.Repeat("c", 64), OwnerID: "user_helper", SourceID: "src_demo_01", ResourceID: resource, Goal: assistant.ExplainResource, State: assistant.Completed, Version: 3, SourceVersion: 1, PolicyVersion: 1, ProfileVersion: 1, ProfileID: "internal_profile_marker", From: at.Add(-15 * time.Minute), Until: at, CreatedAt: at, UpdatedAt: at.Add(time.Second), Deadline: at.Add(120 * time.Second), Volatile: true, LeaseOwner: "internal_lease_marker", ReservedUnits: 16001}
	return assistantservice.Snapshot{Run: run, Volatile: true, Answer: &inference.Answer{Sections: []inference.Section{{Kind: inference.SectionFacts, Text: "Selected resource is available.", Citations: []string{helperCitationID}}}}, Citations: []contextbuilder.CitationReference{{ID: helperCitationID, Kind: "resource", ObjectID: resource, ResourceID: domain.ResourceID(resource), Source: domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "src_demo_01"}, ConfigurationVersion: 1, PolicyVersion: 1, RestoreGeneration: 0, ObservedAt: at.Add(1001 * time.Nanosecond), Range: domain.TimeRange{Start: run.From, End: run.Until}}}}
}

func TestAssistantPublicationMatchesSharedSyntheticFixture(t *testing.T) {
	snapshot := helperSnapshot()
	reply, err := assistantReply(snapshot, "user_helper", helperRunID)
	if err != nil {
		t.Fatal("valid sealed fixture rejected")
	}
	wire := httptest.NewRecorder()
	writeAssistantReply(wire, reply)
	fixture, err := os.ReadFile("../../../../contracts/fixtures/assistant-ready.json")
	if err != nil {
		t.Fatal("read shared synthetic fixture")
	}
	var want, got any
	if json.Unmarshal(fixture, &want) != nil || json.Unmarshal(wire.Body.Bytes(), &got) != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("server publication does not match shared client fixture")
	}
	encoded, err := json.Marshal(reply)
	if err != nil || string(encoded) != "{}" || fmt.Sprintf("%#v", reply) != "[protected conversation reply]" || reply.LogValue().String() != "[protected conversation reply]" {
		t.Fatal("generic reply serialization disclosed content")
	}
	snapshot.Answer.Sections[0].Text = "changed"
	snapshot.Answer.Sections[0].Citations[0] = "changed"
	if reply.Answer.Sections[0].Text != "Selected resource is available." || reply.Answer.Sections[0].Citations[0] != helperCitationID {
		t.Fatal("publication retained mutable answer slices")
	}
}

func TestAssistantProjectionPreservesDistinctMetadataStates(t *testing.T) {
	for _, state := range []assistant.State{assistant.Queued, assistant.Running, assistant.Completed, assistant.Cancelled, assistant.Failed, assistant.Expired, assistant.Interrupted, assistant.Uncertain} {
		for _, volatile := range []bool{true, false} {
			t.Run(string(state)+fmt.Sprint(volatile), func(t *testing.T) {
				snapshot := helperSnapshot()
				snapshot.Run.State = state
				snapshot.Volatile = volatile
				snapshot.Answer = nil
				snapshot.Citations = nil
				snapshot.Run.UsageUncertain = true
				snapshot.Run.UsageConservative = true
				reply, err := assistantReply(snapshot, "user_helper", helperRunID)
				if err != nil || reply.Run.State != state || !reply.Run.UsageUncertain || !reply.Run.UsageConservative || reply.Answer != nil || len(reply.Citations) != 0 {
					t.Fatal("metadata state or usage flags changed")
				}
				want := "unavailable"
				if volatile && (state == assistant.Queued || state == assistant.Running) {
					want = "pending"
				}
				if !volatile && state == assistant.Completed {
					want = "volatile_lost"
				}
				if reply.ContentState != want {
					t.Fatal("content retention state misrepresented")
				}
			})
		}
	}
}

func TestAssistantProjectionRejectsMalformedOrCrossScopeContent(t *testing.T) {
	cases := map[string]func(*assistantservice.Snapshot){
		"owner":           func(s *assistantservice.Snapshot) { s.Run.OwnerID = "another_user" },
		"run":             func(s *assistantservice.Snapshot) { s.Run.ID = "ar_" + strings.Repeat("d", 64) },
		"conversation":    func(s *assistantservice.Snapshot) { s.Run.ConversationID = "invalid" },
		"goal":            func(s *assistantservice.Snapshot) { s.Run.Goal = assistant.InvestigateIncident },
		"state":           func(s *assistantservice.Snapshot) { s.Run.State = "unknown" },
		"version":         func(s *assistantservice.Snapshot) { s.Run.Version = 65 },
		"source_epoch":    func(s *assistantservice.Snapshot) { s.Run.SourceVersion = 0 },
		"source_id":       func(s *assistantservice.Snapshot) { s.Run.SourceID = "../private" },
		"resource_id":     func(s *assistantservice.Snapshot) { s.Run.ResourceID = "private" },
		"window":          func(s *assistantservice.Snapshot) { s.Run.From = s.Run.Until.Add(-time.Minute) },
		"range_precision": func(s *assistantservice.Snapshot) { s.Run.From = s.Run.From.Add(time.Nanosecond) },
		"deadline":        func(s *assistantservice.Snapshot) { s.Run.Deadline = s.Run.CreatedAt },
		"update":          func(s *assistantservice.Snapshot) { s.Run.UpdatedAt = s.Run.CreatedAt.Add(-time.Second) },
		"citation_source": func(s *assistantservice.Snapshot) { s.Citations[0].Source.SourceID = "another_source" },
		"citation_resource": func(s *assistantservice.Snapshot) {
			s.Citations[0].ResourceID = "r1_" + domain.ResourceID(strings.Repeat("B", 42)+"A")
		},
		"citation_object":  func(s *assistantservice.Snapshot) { s.Citations[0].ObjectID = "another_object" },
		"citation_policy":  func(s *assistantservice.Snapshot) { s.Citations[0].PolicyVersion = 2 },
		"citation_restore": func(s *assistantservice.Snapshot) { s.Citations[0].RestoreGeneration = 1 },
		"citation_range": func(s *assistantservice.Snapshot) {
			s.Citations[0].Range.End = s.Citations[0].Range.End.Add(time.Microsecond)
		},
		"citation_count": func(s *assistantservice.Snapshot) { s.Citations = append(s.Citations, s.Citations[0]) },
		"forged_reference": func(s *assistantservice.Snapshot) {
			s.Answer.Sections[0].Citations[0] = "ev_" + strings.Repeat("d", 64)
		},
		"markup":     func(s *assistantservice.Snapshot) { s.Answer.Sections[0].Text = "<img src=private>" },
		"http_link":  func(s *assistantservice.Snapshot) { s.Answer.Sections[0].Text = "https://example.invalid" },
		"blank":      func(s *assistantservice.Snapshot) { s.Answer.Sections[0].Text = "\u0085" },
		"utf8":       func(s *assistantservice.Snapshot) { s.Answer.Sections[0].Text = string([]byte{0xff}) },
		"text_bytes": func(s *assistantservice.Snapshot) { s.Answer.Sections[0].Text = strings.Repeat("😀", 1025) },
		"json_bytes": func(s *assistantservice.Snapshot) {
			s.Answer.Sections = make([]inference.Section, 12)
			for i := range s.Answer.Sections {
				s.Answer.Sections[i] = inference.Section{Kind: inference.SectionGaps, Text: strings.Repeat("\x01", 4096), Citations: []string{}}
			}
		},
		"retention_contradiction": func(s *assistantservice.Snapshot) { s.Volatile = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := helperSnapshot()
			mutate(&snapshot)
			reply, err := assistantReply(snapshot, "user_helper", helperRunID)
			if err == nil || reply.Answer != nil || len(reply.Citations) != 0 {
				t.Fatal("malformed publication released content")
			}
		})
	}
}
