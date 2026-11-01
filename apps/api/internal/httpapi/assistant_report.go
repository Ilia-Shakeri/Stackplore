package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/incidentreport"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type assistantIncidentMetricJSON struct {
	Name            string  `json:"name"`
	Unit            string  `json:"unit"`
	Value           float64 `json:"value"`
	IntervalSeconds int64   `json:"interval_seconds"`
}

type assistantIncidentObservationJSON struct {
	CitationID   string                       `json:"citation_id"`
	Kind         string                       `json:"kind"`
	At           string                       `json:"at"`
	TimeBasis    string                       `json:"time_basis"`
	Availability string                       `json:"availability"`
	Completeness string                       `json:"completeness"`
	Synthetic    bool                         `json:"synthetic"`
	Health       string                       `json:"health"`
	Metric       *assistantIncidentMetricJSON `json:"metric"`
	Severity     string                       `json:"severity"`
	Redacted     bool                         `json:"redacted"`
	Truncated    bool                         `json:"truncated"`
}

type assistantIncidentHypothesisJSON struct {
	Code       string   `json:"code"`
	Rank       int      `json:"rank"`
	Qualified  bool     `json:"qualified"`
	Supporting []string `json:"supporting"`
	Contrary   []string `json:"contrary"`
}

type assistantIncidentReportJSON struct {
	Timeline   []assistantIncidentObservationJSON `json:"timeline"`
	Hypotheses []assistantIncidentHypothesisJSON  `json:"hypotheses"`
	Gaps       []string                          `json:"gaps"`
	NextReads  []string                          `json:"next_reads"`
}

type assistantIncidentReplyJSON struct {
	Run          assistantRunJSON             `json:"run"`
	ContentState string                       `json:"content_state"`
	Report       *assistantIncidentReportJSON  `json:"report,omitempty"`
	Citations    []assistantCitationJSON        `json:"citations,omitempty"`
	Coverage     []inference.ToolOutcome       `json:"coverage,omitempty"`
}

func (assistantIncidentReplyJSON) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected incident reply]")
}

func (assistantIncidentReplyJSON) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (assistantIncidentReplyJSON) LogValue() slog.Value {
	return slog.StringValue("[protected incident reply]")
}

func incidentReply(snapshot assistantservice.Snapshot, owner, id string) (assistantIncidentReplyJSON, error) {
	base, err := assistantReply(snapshot, owner, id)
	if err != nil {
		return assistantIncidentReplyJSON{}, err
	}
	if snapshot.Run.Goal != assistant.InvestigateIncident {
		return assistantIncidentReplyJSON{}, assistant.ErrDenied
	}
	reply := assistantIncidentReplyJSON{Run: base.Run, ContentState: base.ContentState}
	if base.ContentState != "ready" {
		return reply, nil
	}
	if snapshot.Report == nil || len(snapshot.Citations) == 0 {
		return assistantIncidentReplyJSON{}, assistant.ErrUnavailable
	}
	run := snapshot.Run
	scope := incidentreport.Scope{Source: snapshot.Citations[0].Source, ResourceID: domain.ResourceID(run.ResourceID), ConfigurationVersion: run.SourceVersion, PolicyVersion: run.PolicyVersion, RestoreGeneration: run.RestoreGeneration, Range: domain.TimeRange{Start: run.From, End: run.Until}, ObservedAt: run.UpdatedAt}
	if incidentreport.Validate(scope, *snapshot.Report, snapshot.Citations) != nil {
		return assistantIncidentReplyJSON{}, assistant.ErrUnavailable
	}
	report := assistantIncidentReportJSON{Timeline: make([]assistantIncidentObservationJSON, 0, len(snapshot.Report.Timeline)), Hypotheses: make([]assistantIncidentHypothesisJSON, 0, len(snapshot.Report.Hypotheses)), Gaps: slices.Clone(snapshot.Report.Gaps), NextReads: slices.Clone(snapshot.Report.NextReads)}
	for _, observation := range snapshot.Report.Timeline {
		item := assistantIncidentObservationJSON{CitationID: observation.Reference.ID, Kind: observation.Reference.Kind, At: observation.Time.Format(time.RFC3339Nano), TimeBasis: observation.TimeBasis, Availability: string(observation.Availability), Completeness: string(observation.Completeness), Synthetic: observation.Synthetic, Health: string(observation.Health), Redacted: observation.Redacted, Truncated: observation.Truncated}
		if observation.Reference.Kind == "metric" {
			item.Metric = &assistantIncidentMetricJSON{Name: string(observation.MetricName), Unit: string(observation.MetricUnit), Value: observation.MetricValue, IntervalSeconds: observation.IntervalSeconds}
		}
		item.Severity = string(observation.EventSeverity)
		if observation.Reference.Kind == "log" {
			item.Severity = string(observation.LogSeverity)
		}
		report.Timeline = append(report.Timeline, item)
	}
	for _, hypothesis := range snapshot.Report.Hypotheses {
		report.Hypotheses = append(report.Hypotheses, assistantIncidentHypothesisJSON{Code: hypothesis.Code, Rank: hypothesis.Rank, Qualified: hypothesis.Qualified, Supporting: append([]string{}, hypothesis.Supporting...), Contrary: append([]string{}, hypothesis.Contrary...)})
	}
	reportBytes, reportErr := json.Marshal(report)
	answerBytes, answerErr := json.Marshal(base.Answer)
	defer clear(reportBytes)
	defer clear(answerBytes)
	if reportErr != nil || answerErr != nil || len(reportBytes)+len(answerBytes) > inference.MaximumOutputBytes {
		return assistantIncidentReplyJSON{}, assistant.ErrUnavailable
	}
	reply.Report, reply.Citations, reply.Coverage = &report, base.Citations, base.Coverage
	return reply, nil
}

func (handler assistantHTTP) report(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.product.authenticateIdentity(w, r)
	if !ok {
		return
	}
	id, ok := assistantPath(w, r)
	if !ok {
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	snapshot, err := handler.runs.Get(r.Context(), actor, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	reply, err := incidentReply(snapshot, actor.UserID, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	type wireReply assistantIncidentReplyJSON
	writeJSON(w, http.StatusOK, wireReply(reply))
}
