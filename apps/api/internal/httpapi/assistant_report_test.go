package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/incidentreport"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

func reportSnapshot(t *testing.T) assistantservice.Snapshot {
	t.Helper()
	snapshot := investigationSnapshot()
	var observations []contextbuilder.Observation
	for index := range snapshot.Citations {
		ref := &snapshot.Citations[index]
		// This new fixture uses the canonical bundle's microsecond precision.
		// The independent legacy citation fixture retains its nanosecond test.
		ref.ObservedAt = ref.ObservedAt.Truncate(time.Microsecond)
		health := domain.HealthHealthy
		if index == 0 {
			health = domain.HealthUnhealthy
		}
		observations = append(observations, contextbuilder.Observation{Reference: *ref, Time: ref.ObservedAt, TimeBasis: "observed", Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Health: health, Synthetic: true})
	}
	run := snapshot.Run
	report, err := incidentreport.Build(incidentreport.Scope{Source: snapshot.Citations[0].Source, ResourceID: domain.ResourceID(run.ResourceID), ConfigurationVersion: run.SourceVersion, PolicyVersion: run.PolicyVersion, RestoreGeneration: run.RestoreGeneration, Range: domain.TimeRange{Start: run.From, End: run.Until}, ObservedAt: run.UpdatedAt}, observations)
	if err != nil {
		t.Fatal("construct typed report fixture", err)
	}
	snapshot.Report = &report
	return snapshot
}

func TestReportProjectionProtectsOrdinaryFormattingAndDetachesTypedData(t *testing.T) {
	snapshot := reportSnapshot(t)
	reply, err := incidentReply(snapshot, "user_helper", helperRunID)
	if err != nil || reply.ContentState != "ready" || reply.Report == nil || len(reply.Report.Timeline) != 2 || len(reply.Report.Hypotheses) != 1 || len(reply.Coverage) != 7 || len(reply.Citations) != 2 {
		t.Fatal("typed report publication failed", err)
	}
	encoded, err := json.Marshal(reply)
	if err != nil || string(encoded) != "{}" || fmt.Sprintf("%+v", reply) != "[protected incident reply]" || reply.LogValue().String() != "[protected incident reply]" {
		t.Fatal("ordinary serialization exposed incident report")
	}
	snapshot.Report.Gaps[0] = "changed"
	snapshot.Report.Timeline[0].Reference.ID = "changed"
	snapshot.Report.Hypotheses[0].Supporting[0] = "changed"
	if reply.Report.Gaps[0] == "changed" || reply.Report.Timeline[0].CitationID == "changed" || reply.Report.Hypotheses[0].Supporting[0] == "changed" {
		t.Fatal("wire report retained mutable private slices")
	}
}

func TestReportPublicationRequiresIndependentHistoryReadAuthority(t *testing.T) {
	snapshot := reportSnapshot(t)
	run := snapshot.Run
	scope := incidentreport.Scope{Source: snapshot.Citations[0].Source, ResourceID: domain.ResourceID(run.ResourceID), ConfigurationVersion: run.SourceVersion, PolicyVersion: run.PolicyVersion, RestoreGeneration: run.RestoreGeneration, Range: domain.TimeRange{Start: run.From, End: run.Until}, ObservedAt: run.UpdatedAt}
	report, err := incidentreport.BuildWithHistory(scope, snapshot.Report.Timeline, incidentreport.HistoryCoverage{AlertsRead: true, WorkloadRead: true})
	if err != nil {
		t.Fatal("construct internally valid history coverage fixture", err)
	}
	snapshot.Report = &report
	if reply, err := incidentReply(snapshot, "user_helper", helperRunID); err == nil || reply.Report != nil {
		t.Fatal("report flags substituted for independent reserved history proofs")
	}
}

func TestReportPublishesHistoryFactsWithRecoveryAndCurrentCoverage(t *testing.T) {
	snapshot := reportSnapshot(t)
	run := snapshot.Run
	observations := append([]contextbuilder.Observation{}, snapshot.Report.Timeline...)
	for index, kind := range []string{"alert", "workload", "workload"} {
		ref := snapshot.Citations[0]
		ref.ID, ref.Kind, ref.ObjectID = "ev_"+strings.Repeat([]string{"e", "f", "9"}[index], 64), kind, "history_fixture"
		at := run.From.Add(time.Duration(index+1)*time.Second)
		item := contextbuilder.Observation{Reference: ref, Time: at, TimeBasis: "occurred", Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Synthetic: true}
		if kind == "alert" { item.AlertTo = domain.AlertFiring } else { item.TimeBasis, item.WorkloadCounts, item.Desired, item.Ready = "sampled", true, 3, uint64(index); if index == 2 { item.Ready = 3 } }
		observations, snapshot.Citations = append(observations, item), append(snapshot.Citations, ref)
	}
	snapshot.HistoryCoverage = incidentreport.HistoryCoverage{AlertsRead: true, WorkloadRead: true}
	for index, count := range []int{1, 2} {
		snapshot.Outcomes[5+index].Availability, snapshot.Outcomes[5+index].Records = domain.AvailabilityAvailable, count
	}
	report, err := incidentreport.BuildWithHistory(incidentreport.Scope{Source: snapshot.Citations[0].Source, ResourceID: domain.ResourceID(run.ResourceID), ConfigurationVersion: run.SourceVersion, PolicyVersion: run.PolicyVersion, RestoreGeneration: run.RestoreGeneration, Range: domain.TimeRange{Start: run.From, End: run.Until}, ObservedAt: run.UpdatedAt}, observations, snapshot.HistoryCoverage)
	if err != nil { t.Fatal(err) }
	snapshot.Report = &report
	reply, err := incidentReply(snapshot, "user_helper", helperRunID)
	if err != nil || reply.Report == nil || len(reply.Report.Timeline) != 5 { t.Fatal("history transport failed", err) }
	alerts, workloads := 0, 0
	for _, item := range reply.Report.Timeline {
		if item.Alert != nil { alerts++; if item.Alert.From != nil || item.Alert.To != "firing" { t.Fatal("alert transition changed") } }
		if item.Workload != nil { workloads++; if item.Workload.Desired == nil || *item.Workload.Desired != 3 || item.Workload.Ready == nil { t.Fatal("workload counts lost") } }
	}
	if alerts != 1 || workloads != 2 { t.Fatal("historical family omitted") }
	snapshot.HistoryCoverage.WorkloadRead = false
	if _, err := incidentReply(snapshot, "user_helper", helperRunID); err == nil { t.Fatal("report trusted its own history flags") }
}

func TestReportRejectsForgedClaimsReferencesAndSharedOutputOverflow(t *testing.T) {
	for _, mutate := range []func(*assistantservice.Snapshot){
		func(snapshot *assistantservice.Snapshot) { snapshot.Report = nil },
		func(snapshot *assistantservice.Snapshot) { snapshot.Report.Hypotheses[0].Qualified = false },
		func(snapshot *assistantservice.Snapshot) { snapshot.Report.Hypotheses[0].Rank = 4 },
		func(snapshot *assistantservice.Snapshot) { snapshot.Report.Hypotheses[0].Code = "oom_proven" },
		func(snapshot *assistantservice.Snapshot) { snapshot.Report.NextReads[0] = "run_shell" },
		func(snapshot *assistantservice.Snapshot) {
			snapshot.Report.Timeline[0].Reference.ID = "ev_" + strings.Repeat("f", 64)
		},
		func(snapshot *assistantservice.Snapshot) { snapshot.Report.Timeline[0].Reference.PolicyVersion++ },
		func(snapshot *assistantservice.Snapshot) {
			snapshot.Report.Timeline[0].Time = snapshot.Run.UpdatedAt.Add(time.Second)
		},
		func(snapshot *assistantservice.Snapshot) { snapshot.Run.OwnerID = "other_owner" },
		func(snapshot *assistantservice.Snapshot) { snapshot.Run.Goal = assistant.ExplainResource },
		func(snapshot *assistantservice.Snapshot) { snapshot.Volatile = false },
	} {
		snapshot := reportSnapshot(t)
		mutate(&snapshot)
		if reply, err := incidentReply(snapshot, "user_helper", helperRunID); err == nil || reply.Report != nil {
			t.Fatal("unqualified or foreign report published")
		}
	}
	snapshot := reportSnapshot(t)
	section := inference.Section{Kind: inference.SectionGaps, Text: strings.Repeat("\x01", 4096), Citations: []string{}}
	snapshot.Answer.Sections = []inference.Section{section, section, {Kind: inference.SectionGaps, Text: "placeholder", Citations: []string{}}}
	encoded, err := json.Marshal(snapshot.Answer)
	if err != nil {
		t.Fatal(err)
	}
	remaining := inference.MaximumOutputBytes - len(encoded) - 256
	snapshot.Answer.Sections[2].Text = strings.Repeat("\x01", remaining/6)
	if _, err := assistantReply(snapshot, "user_helper", helperRunID); err != nil {
		t.Fatal("answer fixture alone must remain within canonical budget")
	}
	if reply, err := incidentReply(snapshot, "user_helper", helperRunID); err == nil || reply.Report != nil {
		t.Fatal("report acquired a second independent output allowance")
	}
}

func TestReportRouteBindsSessionScopeAndFreshOwnerReadWithoutDispatch(t *testing.T) {
	identity := helperIdentity(t)
	calls, denied := 0, false
	reader := helperReadStub{get: func(ctx context.Context, actor assistant.Actor, id string) (assistantservice.Snapshot, error) {
		calls++
		if ctx == nil || actor.UserID != "user_helper" || actor.DigestKeyID != "helper_key" || id != helperRunID {
			t.Error("report read changed trusted actor or run")
		}
		if denied {
			return assistantservice.Snapshot{}, assistant.ErrDenied
		}
		return reportSnapshot(t), nil
	}, cancel: func(context.Context, assistant.Actor, string) (assistant.Run, error) {
		t.Error("report attempted a mutation")
		return assistant.Run{}, assistant.ErrDenied
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
	})
	path := "/api/v1/assistant/runs/" + helperRunID + "/report"
	response := serve(handler, helperRequest(http.MethodGet, path))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"qualified":true`) || calls != 1 {
		t.Fatal("private report route failed or was cacheable")
	}
	for _, forbidden := range []string{"internal_profile_marker", "internal_lease_marker", "user_helper", "session_helper", "object_id", "configuration_version", "log_message", "answer"} {
		if strings.Contains(response.Body.String(), forbidden) || strings.Contains(logs.String(), "health_problem_possible") {
			t.Fatal("report disclosed internal metadata, text or diagnostic content")
		}
	}
	for _, request := range []*http.Request{
		makeRequest(http.MethodGet, path, "198.51.100.10:4000"),
		helperRequest(http.MethodGet, path+"?scope=other"),
		helperRequest(http.MethodGet, "/api/v1/assistant/runs/invalid/report"),
	} {
		response := serve(handler, request)
		if response.Code < 400 || calls != 1 || strings.Contains(response.Body.String(), "timeline") {
			t.Fatal("rejected report request reached use case")
		}
	}
	denied = true
	response = serve(handler, helperRequest(http.MethodGet, path))
	if response.Code != http.StatusNotFound || calls != 2 || strings.Contains(response.Body.String(), "timeline") {
		t.Fatal("revoked report skipped fresh use-case authority")
	}
}
