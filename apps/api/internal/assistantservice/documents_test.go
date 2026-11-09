package assistantservice

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

type fixtureDocumentSource struct {
	denied atomic.Bool
	reads  atomic.Int32
}

func (source *fixtureDocumentSource) AuthorizeAssistantRunbook(context.Context, assistant.Actor, runbook.Reference) error {
	if source.denied.Load() {
		return runbook.ErrDenied
	}
	return nil
}
func (source *fixtureDocumentSource) ReadAssistantRunbook(_ context.Context, _ assistant.Actor, reference runbook.Reference) (runbook.Content, error) {
	source.reads.Add(1)
	if source.denied.Load() {
		return runbook.Content{}, runbook.ErrDenied
	}
	return runbook.Content{Document: runbook.Document{ID: reference.DocumentID, Locale: reference.Locale, Title: "Reviewed local guidance", Body: "Inspect current observations before choosing a cause."}, Provenance: runbook.Provenance{Kind: "operator_authored", Source: "Synthetic fixture", License: "Fixture permission", UseReviewed: true, AssistanceReviewed: true}}, nil
}
func (authorizer *fakeAuthorizer) AuthorizeDocumentAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope, reference runbook.Reference) error {
	if err := authorizer.check(ctx); err != nil {
		return err
	}
	return authorizer.reader.AuthorizeDocumentAdmission(ctx, actor, run, scope, reference)
}

func TestSelectedDocumentRunReservesReadAndKeepsGuidanceSeparate(t *testing.T) {
	for _, goal := range []assistant.Goal{assistant.ExplainResource, assistant.InvestigateIncident} {
		service, repository, authority, model := fixtureService(t, func(options *Options, model *modelFixture) {
			options.AllowReviewedDocumentProse = true
			model.countHook = func(request inference.Request) error {
				found := 0
				for _, item := range request.Evidence {
					if item.Guidance {
						found++
						if !strings.Contains(item.Text, "reviewed_guidance_not_live_facts") {
							t.Fatal("guide supplied as observation")
						}
					}
				}
				if found != 1 {
					t.Fatal("selected guide not included exactly once")
				}
				return nil
			}
		})
		input := inputFixture(t, 1)
		input.Admission.Goal = goal
		reference := runbook.Reference{CorpusID: "operations", Version: 2, Digest: [32]byte{1}, DocumentID: "guide", Locale: "en"}
		input.Document = &reference
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal("selected guide admission failed", err)
		}
		if claimed, err := service.runOne(context.Background(), "worker"); !claimed || err != nil {
			t.Fatal("selected guide execution failed", err)
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		expected := 2
		if goal == assistant.InvestigateIncident {
			expected = 8
		}
		if err != nil || snapshot.Run.ToolCalls != expected || model.calls.Load() != 1 || authority.documents.reads.Load() != 1 {
			t.Fatal("guide dispatch or accounting mismatch", err)
		}
		if snapshot.Citations[len(snapshot.Citations)-1].Document != reference {
			t.Fatal("exact guide witness lost")
		}
		if snapshot.Report != nil {
			for _, observation := range snapshot.Report.Timeline {
				if observation.Reference.Kind == "document" {
					t.Fatal("guide entered factual timeline")
				}
			}
		}
		repository.mu.Lock()
		trace := slices.Clone(repository.trace)
		repository.mu.Unlock()
		begin := slices.Index(trace, "begin:reviewed_document")
		read := slices.Index(trace, "read:reviewed_document")
		finish := slices.Index(trace, "finish:reviewed_document")
		if begin < 0 || read <= begin || finish <= read {
			t.Fatal("guide read preceded durable reservation", trace)
		}
		authority.documents.denied.Store(true)
		if _, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID); err != assistant.ErrDenied {
			t.Fatal("withdrawn guide content released", err)
		}
	}
}
