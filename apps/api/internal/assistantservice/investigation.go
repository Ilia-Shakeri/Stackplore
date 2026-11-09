package assistantservice

import (
	"context"
	"errors"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantplan"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

func coverage(kind assistant.StepKind, bundle contextbuilder.Bundle) inference.ToolOutcome {
	summary := bundle.Summary()
	return inference.ToolOutcome{Tool: string(kind), Availability: summary.Availability, Completeness: summary.Completeness, Records: summary.Records, MoreAvailable: summary.MoreAvailable}
}

// Caller holds service.mu. References remain bound to their separate read proofs.
func (service *Service) citationSetLocked(entry *memoryRun) []contextbuilder.CitationReference {
	var result []contextbuilder.CitationReference
	for _, proof := range service.evidenceSetLocked(entry) {
		result = append(result, proof.References()...)
	}
	return result
}

func collectionError(err error) error {
	if errors.Is(err, assistantevidence.ErrDenied) {
		return assistant.ErrDenied
	}
	if errors.Is(err, assistantevidence.ErrBudget) {
		return assistant.ErrQuota
	}
	return assistant.ErrUnavailable
}

func additionalItems(scope contextbuilder.Scope, kind assistant.StepKind, proof assistantevidence.Evidence) ([]inference.Evidence, error) {
	if proof.ReadKind() != kind {
		return nil, assistant.ErrDenied
	}
	items, err := proof.Bundle().Items()
	refs := proof.References()
	if err != nil || len(items) != len(refs) || len(items) > assistantplan.RecordsPerRead {
		return nil, assistant.ErrQuota
	}
	expected := map[assistant.StepKind]string{assistant.Health: "health", assistant.Metrics: "metric", assistant.Events: "event", assistant.Topology: "relationship", assistant.Logs: "log", assistant.AlertHistory: "alert", assistant.WorkloadHistory: "workload", assistant.ReviewedDocument: "document"}[kind]
	if expected == "" || len(scope.Resources) != 1 {
		return nil, assistant.ErrDenied
	}
	result := make([]inference.Evidence, 0, len(items))
	for index, item := range items {
		ref := refs[index]
		resourceMatches := ref.ResourceID == scope.Resources[0].ID
		if kind == assistant.Topology {
			resourceMatches = ref.ResourceID == ""
		}
		if ref.Kind != expected || !resourceMatches || !assistant.Identifier(ref.ObjectID) || ref.Source != scope.Source || ref.ConfigurationVersion != scope.ConfigurationVersion || ref.PolicyVersion != scope.PolicyVersion || ref.RestoreGeneration != scope.RestoreGeneration || ref.Range != scope.Range || ref.ID != item.ID {
			return nil, assistant.ErrDenied
		}
		if kind == assistant.ReviewedDocument && (ref.Document.Validate() != nil || ref.Document != proof.DocumentReference()) {
			return nil, assistant.ErrDenied
		}
		result = append(result, inference.Evidence{ID: item.ID, Text: item.Text, Guidance: kind == assistant.ReviewedDocument})
	}
	return result, nil
}

// execute already reserved/completed the mandatory inventory step. Every extra
// family has its own durable reservation and never follows a returned cursor.
func (service *Service) investigate(ctx context.Context, entry *memoryRun, initial []inference.Evidence) (result []inference.Evidence, failure error) {
	service.mu.Lock()
	input := entry.input
	if entry.revoked || entry.evidence == nil {
		service.mu.Unlock()
		return nil, assistant.ErrDenied
	}
	first := coverage(assistant.Inventory, entry.evidence.Bundle())
	service.mu.Unlock()
	plan, err := assistantplan.WithDocument(input.Admission.Goal, input.LogConsent, input.Document != nil)
	if err != nil || len(plan) < 2 || len(plan) > assistantplan.MaximumReads {
		return nil, assistant.ErrInvalid
	}
	result = make([]inference.Evidence, len(initial), inference.MaximumEvidence)
	copy(result, initial)
	defer func() {
		if failure != nil {
			clear(result)
		}
	}()
	service.mu.Lock()
	entry.outcomes = []inference.ToolOutcome{first}
	service.mu.Unlock()
	seen := make(map[string]bool, inference.MaximumEvidence)
	bytes := len(input.Question)
	for _, item := range initial {
		seen[item.ID] = true
		bytes += len(item.ID) + len(item.Text)
	}
	for _, kind := range plan[1:] {
		step, err := service.begin(ctx, entry, kind, 0)
		if err != nil {
			return result, err
		}
		service.mu.Lock()
		run := entry.run
		scope := cloneScope(entry.input.Scope)
		revoked := entry.revoked
		service.mu.Unlock()
		if revoked || ctx.Err() != nil {
			return result, assistant.ErrDenied
		}
		scope.ObservedAt = service.time()
		work, cancel := service.evidenceContext(ctx)
		var document *runbook.Reference
		if kind == assistant.ReviewedDocument {
			document = input.Document
		}
		proof, err := service.collector.Read(work, assistantevidence.ReadRequest{Actor: input.Admission.Actor, Run: run, Scope: scope, Kind: kind, Destination: contextbuilder.Local, LogConsent: kind == assistant.Logs && input.LogConsent, RecordLimit: assistantplan.RecordsPerRead, ByteLimit: assistantplan.BytesPerRead, Document: document})
		cancel()
		if err != nil {
			return result, collectionError(err)
		}
		if kind == assistant.ReviewedDocument && (document == nil || proof.DocumentReference() != *document) {
			return result, assistant.ErrDenied
		}
		items, err := additionalItems(scope, kind, proof)
		if err != nil {
			return result, err
		}
		if len(result)+len(items) > inference.MaximumEvidence {
			return result, assistant.ErrQuota
		}
		for _, item := range items {
			if seen[item.ID] {
				return result, assistant.ErrDenied
			}
			seen[item.ID] = true
			bytes += len(item.ID) + len(item.Text)
		}
		if bytes > inference.MaximumInputBytes {
			return result, assistant.ErrQuota
		}
		result = append(result, items...)
		service.mu.Lock()
		if entry.revoked || service.closed || len(entry.extraEvidence) >= assistantplan.MaximumReads-1 {
			service.mu.Unlock()
			return result, assistant.ErrDenied
		}
		entry.extraEvidence = append(entry.extraEvidence, proof)
		entry.outcomes = append(entry.outcomes, coverage(kind, proof.Bundle()))
		service.mu.Unlock()
		if err := service.authorizeContent(ctx, entry); err != nil {
			return result, err
		}
		if err := service.completeStep(ctx, entry, step, 0, false); err != nil {
			return result, err
		}
	}
	return result, nil
}
